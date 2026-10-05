// Command mcp-proxmox is the composition root of the mcp-proxmox MCP server.
// It wires configuration (envconfig), optional backend enablement, logging
// (including the stdio-friendly file sink), the session transport, and the
// tool-registration gate. See SPEC.md §4 / §5.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-proxmox/adapter/config"
	"github.com/teran/mcp-proxmox/adapter/logging"
	"github.com/teran/mcp-proxmox/adapter/mcp"
	"github.com/teran/mcp-proxmox/adapter/pbs"
	"github.com/teran/mcp-proxmox/adapter/pve"
	"github.com/teran/mcp-proxmox/adapter/token"
	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/port"
)

// Build metadata. These are injected at build time via ldflags (see
// .goreleaser.yml / .forgejo/workflows/ci.yml):
//
//	-X main.appName=... -X main.appVersion=... -X main.appCommitHash=... -X main.appTimestamp=...
//
// `version` is the semver used for the -version flag and the MCP implementation
// version; it defaults to v0.0.0-dev when not set (e.g. plain `go build`).
var (
	appName       = "mcp-proxmox"
	appVersion    = "v0.0.0-dev"
	appCommitHash = "unknown"
	appTimestamp  = "unknown"
	version       = "v0.0.0-dev"
)

// defaultLogFile is the stdio-friendly file sink used when LOG_LEVEL is set
// (override via LOG_FILENAME). See SPEC.md §5.2 / L1–L3.
const defaultLogFile = "/tmp/mcp-proxmox.log"

// versionString returns the human-readable version line printed by the
// -version flag (e.g. "mcp-proxmox v1.2.3").
func versionString() string {
	return "mcp-proxmox " + version
}

// buildApp wires the backends enabled in cfg into a single App.
func buildApp(cfg config.Config, logger port.AppLogger, version string) *application.App {
	opts := make([]application.Option, 0, 2)

	if cfg.PVEEnabled() {
		pveToken := token.NewStaticTokenSource(cfg.PVEToken)
		pveGW := pve.NewGateway(pve.Config{
			Endpoint:     cfg.PVEEndpoint,
			TokenSource:  pveToken,
			CACertPath:   cfg.PVECACertPath,
			Logger:       logger,
			Retries:      2,
			RetryBackoff: 200 * time.Millisecond,
		})
		opts = append(opts, application.WithPVE(pveGW))
		logger.Infof("PVE backend enabled: %s", cfg.PVEEndpoint)
	}

	if cfg.PBSEnabled() {
		pbsToken := token.NewPBSStaticTokenSource(cfg.PBSToken)
		pbsGW := pbs.NewGateway(pbs.Config{
			Endpoint:     cfg.PBSEndpoint,
			TokenSource:  pbsToken,
			CACertPath:   cfg.PBSCACertPath,
			Logger:       logger,
			Retries:      2,
			RetryBackoff: 200 * time.Millisecond,
		})
		opts = append(opts, application.WithPBS(pbsGW))
		logger.Infof("PBS backend enabled: %s", cfg.PBSEndpoint)
	}

	return application.New(logger, version, opts...)
}

func main() {
	showVersion := flag.Bool("version", false, "print the build version and exit")
	flag.Parse()

	// -version: print the injected semver and exit before any server setup.
	if *showVersion {
		fmt.Println(versionString())
		os.Exit(0)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Configuration comes from environment variables (kelseyhightower/envconfig):
	// PVE_ENDPOINT/PVE_TOKEN(/PVE_CA_CERT_PATH), PBS_ENDPOINT/PBS_TOKEN
	// (/PBS_CA_CERT_PATH), LOG_LEVEL, LOG_FILENAME, LOG_FORMAT. See SPEC.md §4.4.
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	// Logging (SPEC.md §5 / L2). When LOG_LEVEL is set, logs are ENABLED and
	// written to LOG_FILENAME (default /tmp/mcp-proxmox.log) at that level in
	// LOG_FORMAT (default text). When LOG_LEVEL is unset, logging is DISABLED:
	// the logger discards all output, so nothing leaks to stdout/stderr (a stdio
	// transport must not pollute the protocol channel). Per B5/L6 the very first
	// log line emitted (only when enabled) is the startup banner.
	logrusLogger := logging.NewLogger(cfg.LogLevel, cfg.LogFormat)
	if cfg.LogLevel == "" {
		logrusLogger.SetOutput(io.Discard)
	} else {
		logFile := cfg.LogFileName
		if logFile == "" {
			logFile = defaultLogFile
		}
		// #nosec G304 -- logFile is a configuration-supplied LOG_FILENAME path
		// (default /tmp/mcp-proxmox.log), never attacker-controlled input. Its
		// content is never logged.
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			logrusLogger.Fatalf("open log file %s: %v", logFile, err)
		}
		logrusLogger.SetOutput(f)
		// B5/L6: the first log line, emitted only when logging is enabled.
		logrusLogger.Infof("Starting %s/%s (commit: %s; built at %s) MCP server over stdio",
			appName, appVersion, appCommitHash, appTimestamp)
	}
	appLogger := logging.NewAppLogger(logrusLogger)   // port.AppLogger (core)
	slogLogger := logging.NewSlogLogger(logrusLogger) // *slog.Logger for the SDK

	// Build the App, injecting each backend only when it is enabled. A backend
	// whose endpoint or token is missing is simply absent — its tools are not
	// registered and nothing fails.
	app := buildApp(cfg, appLogger, version)

	impl := &mcpSDK.Implementation{Name: "mcp-proxmox", Version: version}

	// Mutation gate (SPEC.md §2.5): mutation tools are registered only when
	// ENABLE_MUTATIONS=true; otherwise only read-only + system tools are
	// exposed. The value comes from the env-driven config (default false).
	enableMutations := cfg.EnableMutations

	// One session ID per MCP session (a single process run in stdio mode). It is
	// shared between the stdio transport connection and the request context so
	// connect/disconnect and tool/application lines correlate.
	sessionID := mcp.NewSessionID()

	s := mcp.NewServer(impl, mcp.Deps{
		App:             app,
		Logger:          appLogger,
		SlogLogger:      slogLogger,
		Log:             logrusLogger,
		Instructions:    "MCP server exposing the Proxmox VE and Proxmox Backup Server REST APIs.",
		SessionID:       sessionID,
		EnableMutations: enableMutations,
	}, &mcpSDK.ServerOptions{
		Logger:       slogLogger,
		Instructions: "MCP server exposing the Proxmox VE and Proxmox Backup Server REST APIs.",
	})

	logrusLogger.Info("stdio mode")
	// s.Run returns on ctx cancellation (SIGINT/SIGTERM via signal.NotifyContext),
	// which lets us exit gracefully instead of ignoring Ctrl+C.
	transport := mcp.NewSessionTransport(&mcpSDK.StdioTransport{}, sessionID)
	if err := s.Run(ctx, transport); err != nil && !errors.Is(err, context.Canceled) {
		logrusLogger.Fatal(err)
	}
}
