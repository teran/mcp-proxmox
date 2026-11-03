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
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sirupsen/logrus"

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

const (
	// Launch modes (M06): the transport is selectable via -mode, defaulting to
	// stdio. Only stdio is implemented — this is a Local stdio-only server, so
	// http (Streamable-HTTP) is not yet supported and is rejected.
	modeStdio = "stdio"
	modeHTTP  = "http"

	// defaultLogFile is the stdio-friendly file sink used when LOG_LEVEL is set
	// (override via LOG_FILENAME). See SPEC.md §5.2 / L1–L3.
	defaultLogFile = "/tmp/mcp-proxmox.log"
	// maxBackends is the capacity hint for the App options slice (PVE + PBS).
	maxBackends = 2
	// defaultRetries / defaultRetryBackoff configure the gateway read retries.
	defaultRetries      = 2
	defaultRetryBackoff = 200 * time.Millisecond
	// logFileMode is the permission mask applied to the log file (0600).
	logFileMode = 0o600
)

// versionString returns the human-readable version line printed by the
// -version flag (e.g. "mcp-proxmox v1.2.3").
func versionString() string {
	return "mcp-proxmox " + version
}

// validateMode checks the -mode flag value (M06). Only "stdio" is implemented —
// this is a Local stdio-only server. "http" (Streamable-HTTP) is recognized but
// not implemented, and any other value is invalid. It returns nil for "stdio"
// and a descriptive error otherwise.
func validateMode(mode string) error {
	switch mode {
	case modeStdio:
		return nil
	case modeHTTP:
		return errors.New("http mode is not implemented: this is a Local stdio-only server")
	default:
		return fmt.Errorf("invalid mode %q: must be %q or %q", mode, modeStdio, modeHTTP)
	}
}

// logRegisteredTools emits the startup tool-surface log (SPEC L10/L11/L05GO):
// one info line per registered tool, tagged with the tool name, its
// human-readable title, and the always_present field (true for every tool
// because registration is static per run — there are no per-caller dynamic
// tools). It is a no-op on an empty surface, and when logging is disabled the
// logger discards the lines anyway.
func logRegisteredTools(log *logrus.Logger, surface []mcp.ToolSurface) {
	for _, t := range surface {
		log.WithField("tool", t.Name).WithField("title", t.Title).WithField("always_present", t.AlwaysPresent).Infof("registered tool")
	}
}

// buildApp wires the backends enabled in cfg into a single App.
func buildApp(cfg config.Config, logger port.AppLogger, version string) *application.App {
	opts := make([]application.Option, 0, maxBackends)

	if cfg.PVEEnabled() {
		pveToken := token.NewStaticTokenSource(cfg.PVEToken)
		pveGW := pve.NewGateway(pve.Config{
			Endpoint:     cfg.PVEEndpoint,
			TokenSource:  pveToken,
			CACertPath:   cfg.PVECACertPath,
			Logger:       logger,
			Retries:      defaultRetries,
			RetryBackoff: defaultRetryBackoff,
			UserAgent:    fmt.Sprintf("%s/%s (https://github.com/teran/mcp-proxmox)", appName, appVersion),
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
			Retries:      defaultRetries,
			RetryBackoff: defaultRetryBackoff,
			UserAgent:    fmt.Sprintf("%s/%s (https://github.com/teran/mcp-proxmox)", appName, appVersion),
		})
		opts = append(opts, application.WithPBS(pbsGW))
		logger.Infof("PBS backend enabled: %s", cfg.PBSEndpoint)
	}

	return application.New(logger, version, opts...)
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the server and returns the process exit code. It is split out of
// main for testability: args are the CLI arguments (excluding argv[0] when
// called from main via os.Args[1:]), and stdout/stderr are the output streams
// used for normal output and diagnostics. Exit codes: 0 on success / -version,
// 1 on any error (config load, invalid mode, log setup, run failure).
func run(args []string, stdout, stderr io.Writer) int {
	flagSet := flag.NewFlagSet("mcp-proxmox", flag.ContinueOnError)
	flagSet.SetOutput(stderr)

	// Tolerate a leading program-name argument (argv[0]) for callers that pass
	// the full argv. main() passes os.Args[1:] (no program name); tests may pass
	// argv including it. Go's flag package stops parsing at the first non-flag
	// (positional) argument, so a leading bare program name would otherwise hide
	// every flag. This CLI has no positional arguments, so skipping one is safe.
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		args = args[1:]
	}

	var (
		showVersion = flagSet.Bool("version", false, "print the build version and exit")
		mode        = flagSet.String("mode", modeStdio, "transport mode: stdio (default) or http (not yet implemented)")
	)

	if err := flagSet.Parse(args); err != nil {
		// The FlagSet has already printed the error and usage to stderr.
		return 1
	}

	// -version: print the injected semver and exit before any server setup.
	if *showVersion {
		fmt.Fprintln(stdout, versionString())

		return 0
	}

	// Launch mode (M06): reject anything that isn't the implemented stdio mode.
	if err := validateMode(*mode); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)

		return 1
	}

	// Configuration comes from environment variables (kelseyhightower/envconfig):
	// PVE_ENDPOINT/PVE_TOKEN(/PVE_CA_CERT_PATH), PBS_ENDPOINT/PBS_TOKEN
	// (/PBS_CA_CERT_PATH), LOG_LEVEL, LOG_FILENAME, LOG_FORMAT. See SPEC.md §4.4.
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(stderr, "load config: %v\n", err)

		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logrusLogger, appLogger, slogLogger, err := setupLogging(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)

		return 1
	}

	if err := runServer(ctx, cfg, appLogger, slogLogger, logrusLogger, version); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)

		return 1
	}

	return 0
}

// setupLogging builds the logrus logger and its port/slog adapters per the
// config (SPEC.md §5 / L2). When LOG_LEVEL is set, logs are ENABLED and written
// to LOG_FILENAME (default /tmp/mcp-proxmox.log) at that level in LOG_FORMAT;
// when LOG_LEVEL is unset, logging is DISABLED so nothing pollutes the stdio
// protocol channel. Per B5/L6 the very first line (only when enabled) is the
// startup banner. A non-nil error means the log file could not be opened.
func setupLogging(cfg config.Config) (*logrus.Logger, port.AppLogger, *slog.Logger, error) {
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
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, logFileMode)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("open log file %s: %w", logFile, err)
		}
		logrusLogger.SetOutput(f)
		// B5/L6: the first log line, emitted only when logging is enabled.
		logrusLogger.Infof("Starting %s/%s (commit: %s; built at %s) MCP server over stdio",
			appName, appVersion, appCommitHash, appTimestamp)
	}

	return logrusLogger, logging.NewAppLogger(logrusLogger), logging.NewSlogLogger(logrusLogger), nil
}

// runServer builds the MCP server over the configured backends and runs it over
// the stdio transport until ctx is done. It returns nil on success or on
// cancellation (SIGINT/SIGTERM), and an error if the server fails for any other
// reason. Mutation tools are registered only when ENABLE_MUTATIONS is set
// (SPEC.md §2.5).
func runServer(
	ctx context.Context,
	cfg config.Config,
	appLogger port.AppLogger,
	slogLogger *slog.Logger,
	logrusLogger *logrus.Logger,
	version string,
) error {
	// Build the App, injecting each backend only when it is enabled. A backend
	// whose endpoint or token is missing is simply absent — its tools are not
	// registered and nothing fails.
	app := buildApp(cfg, appLogger, version)

	impl := &mcpSDK.Implementation{Name: "mcp-proxmox", Version: version}

	// One session ID per MCP session (a single process run in stdio mode). It is
	// shared between the stdio transport connection and the request context so
	// connect/disconnect and tool/application lines correlate.
	sessionID := mcp.NewSessionID()

	s := mcp.NewServer(impl, mcp.Deps{
		App:             app,
		Logger:          appLogger,
		SlogLogger:      slogLogger,
		Log:             logrusLogger,
		Instructions:    mcp.ServerInstructions,
		SessionID:       sessionID,
		EnableMutations: cfg.EnableMutations,
	}, &mcpSDK.ServerOptions{
		Logger:       slogLogger,
		Instructions: mcp.ServerInstructions,
	})

	logrusLogger.Info("stdio mode")
	logRegisteredTools(logrusLogger, mcp.RegisteredToolSurface(app, cfg.EnableMutations))

	// s.Run returns on ctx cancellation (SIGINT/SIGTERM via signal.NotifyContext),
	// which lets us exit gracefully instead of ignoring Ctrl+C.
	transport := mcp.NewSessionTransport(&mcpSDK.StdioTransport{}, sessionID)
	if err := s.Run(ctx, transport); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}
