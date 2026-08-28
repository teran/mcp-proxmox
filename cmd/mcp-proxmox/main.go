// Command mcp-proxmox is the composition root of the mcp-proxmox MCP server.
// It wires configuration (envconfig), optional backend enablement, logging
// (including the stdio-friendly /tmp/mcp-proxmox.log sink), the session
// transport, and the tool-registration gate. See SPEC.md §4 / §5.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
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
)

// version is injected at build time via -ldflags "-X main.version=...".
// Defaults to v0.0.0-dev when not set (e.g. plain `go build`).
var version = "v0.0.0-dev"

// versionString returns the human-readable version line printed by the
// -version flag (e.g. "mcp-proxmox v1.2.3").
func versionString() string {
	return "mcp-proxmox " + version
}

func main() {
	logFormat := flag.String("log-format", "text", "text|json")
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
	// (/PBS_CA_CERT_PATH), LOG_LEVEL. See SPEC.md §4.4.
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	// Logging. When LOG_LEVEL is set, logs are written to /tmp/mcp-proxmox.log
	// at that level (stdio-friendly file sink); otherwise the default info level
	// and default destination are used.
	logrusLogger := logging.NewLogger(cfg.LogLevel, *logFormat)
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		f, err := os.OpenFile("/tmp/mcp-proxmox.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			logrusLogger.Fatalf("open log file /tmp/mcp-proxmox.log: %v", err)
		}
		logrusLogger.SetOutput(f)
	}
	appLogger := logging.NewAppLogger(logrusLogger)   // port.AppLogger (core)
	slogLogger := logging.NewSlogLogger(logrusLogger) // *slog.Logger for the SDK

	// Build the App, injecting each backend only when it is enabled. A backend
	// whose endpoint or token is missing is simply absent — its tools are not
	// registered and nothing fails.
	app := application.New(appLogger, version)

	if cfg.PVEEnabled() {
		pveToken := token.NewStaticTokenSource(cfg.PVEToken)
		pveGW := pve.NewGateway(pve.Config{
			Endpoint:     cfg.PVEEndpoint,
			TokenSource:  pveToken,
			HTTPClient:   &http.Client{Timeout: 30 * time.Second},
			CACertPath:   cfg.PVECACertPath,
			Logger:       appLogger,
			Retries:      2,
			RetryBackoff: 200 * time.Millisecond,
		})
		app = application.New(appLogger, version, application.WithPVE(pveGW))
		logrusLogger.Infof("PVE backend enabled: %s", cfg.PVEEndpoint)
	}

	if cfg.PBSEnabled() {
		pbsToken := token.NewPBSStaticTokenSource(cfg.PBSToken)
		pbsGW := pbs.NewGateway(pbs.Config{
			Endpoint:     cfg.PBSEndpoint,
			TokenSource:  pbsToken,
			HTTPClient:   &http.Client{Timeout: 30 * time.Second},
			CACertPath:   cfg.PBSCACertPath,
			Logger:       appLogger,
			Retries:      2,
			RetryBackoff: 200 * time.Millisecond,
		})
		app = application.New(appLogger, version, application.WithPBS(pbsGW))
		logrusLogger.Infof("PBS backend enabled: %s", cfg.PBSEndpoint)
	}

	impl := &mcpSDK.Implementation{Name: "mcp-proxmox", Version: version}

	// Mutation gate: for this milestone only read-only + system tools are
	// registered, so the gate is off. The exact gating UX (env flag, config,
	// CLI flag) is TBD (SPEC.md §2.4); the seam is Deps.EnableMutations, which
	// adapter/mcp honors when registering mutation tools.
	enableMutations := false

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
