// Package mcp assembles the go-sdk MCP server and registers the tool registry.
// It is the primary "interface" adapter: it translates between MCP tool calls
// and the application layer. This is the ONLY place (besides cmd/mcp-proxmox)
// that imports the go-sdk. See SPEC.md §2.3 / §6.
package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-proxmox/adapter/logging"
	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/port"
)

// Deps is the dependencies of the primary MCP adapter.
type Deps struct {
	App          *application.App
	Logger       port.AppLogger
	SlogLogger   *slog.Logger   // for ServerOptions.Logger
	Log          *logrus.Logger // for ctx-aware tool logging (optional)
	Instructions string
	// SessionID is the ID of the current MCP session. It is injected into the
	// context of every incoming request so tool and application log lines carry
	// session_id (in addition to request_id). When empty, no session_id field is
	// added. It should match the ID used by the transport's connection (see
	// NewSessionTransport) so connect/disconnect lines and tool lines correlate.
	SessionID string
	// EnableMutations gates ALL mutation tools. The exact gating UX is TBD
	// (SPEC.md §2.4): the seam is that adapter/mcp registers mutation tools only
	// when this is true. Read-only (query) tools are always registered. For this
	// milestone it is always false (read-only + system vertical slice).
	EnableMutations bool
}

// NewServer creates a go-sdk server with registered tools.
func NewServer(impl *mcpSDK.Implementation, deps Deps, opts *mcpSDK.ServerOptions) *mcpSDK.Server {
	s := mcpSDK.NewServer(impl, opts) // opts.Logger = deps.SlogLogger, shared SchemaCache
	// Inject a unique per-request ID into the context for every incoming MCP
	// request, so all log lines from a single tool call share it, and emit a
	// structured per-request debug/trace log (SPEC.md §5.5 / L8).
	s.AddReceivingMiddleware(func(next mcpSDK.MethodHandler) mcpSDK.MethodHandler {
		return func(ctx context.Context, method string, req mcpSDK.Request) (mcpSDK.Result, error) {
			ctx = withSessionContext(deps)(ctx)
			start := time.Now()
			res, err := next(ctx, method, req)
			logIncomingRequest(deps, ctx, method, req, res, time.Since(start), err)
			return res, err
		}
	})
	RegisterTools(s, deps.App, toolLogger{log: deps.Logger, l: deps.Log}, deps.EnableMutations)
	return s
}

// logIncomingRequest emits a structured per-request log line at debug level
// (args/in_bytes/out_bytes only at trace level) when logging is enabled. It
// carries the tool name, source ("STDIO"), duration and outcome, and is tagged
// with session_id/request_id from ctx. Sensitive arguments are never logged.
func logIncomingRequest(deps Deps, ctx context.Context, method string, req mcpSDK.Request, res mcpSDK.Result, dur time.Duration, err error) {
	if deps.Log == nil || !deps.Log.IsLevelEnabled(logrus.DebugLevel) {
		return
	}
	fields := logrus.Fields{
		"source":      "STDIO",
		"duration_ms": dur.Milliseconds(),
		"outcome":     "ok",
	}
	if err != nil {
		fields["outcome"] = "error"
	}

	// tools/call: capture the tool name; args + byte sizes only at trace level
	// (never for sensitive inputs — tokens are never tool arguments).
	if params, ok := req.GetParams().(*mcpSDK.CallToolParams); ok {
		fields["tool"] = params.Name
		if deps.Log.IsLevelEnabled(logrus.TraceLevel) && params.Arguments != nil {
			in, _ := json.Marshal(params.Arguments)
			fields["in_bytes"] = len(in)
			fields["args"] = sanitizeArgs(params.Arguments)
		}
		if deps.Log.IsLevelEnabled(logrus.TraceLevel) && res != nil {
			if out, err := json.Marshal(res); err == nil {
				fields["out_bytes"] = len(out)
			}
		}
	}

	e := logging.WithSession(ctx, deps.Log)
	e.WithFields(fields).Debugf("mcp request: %s", method)
}

// sanitizeArgs returns a JSON-ish string of the tool arguments, omitting any
// value whose key looks sensitive (token, password, secret, key, auth, cookie,
// credential). Current tools take only node/vmid/store names, so this is
// defensive for future mutation tools.
func sanitizeArgs(args any) string {
	m, ok := args.(map[string]any)
	if !ok {
		b, _ := json.Marshal(args)
		return string(b)
	}
	clean := make(map[string]any, len(m))
	for k, v := range m {
		if isSensitiveKey(k) {
			clean[k] = "[redacted]"
			continue
		}
		clean[k] = v
	}
	b, _ := json.Marshal(clean)
	return string(b)
}

// isSensitiveKey reports whether a tool-argument key should never be logged.
func isSensitiveKey(k string) bool {
	switch strings.ToLower(k) {
	case "token", "tokens", "password", "pass", "secret", "secret_key", "secretkey",
		"api_key", "apikey", "auth", "authorization", "cookie", "credential", "credentials":
		return true
	}
	return false
}

// withSessionContext returns a function that augments a request context with a
// fresh per-request ID and, when deps.SessionID is non-empty, the session ID.
func withSessionContext(deps Deps) func(context.Context) context.Context {
	return func(ctx context.Context) context.Context {
		ctx = port.WithRequestID(ctx, newRequestID())
		if deps.SessionID != "" {
			ctx = port.WithSessionID(ctx, deps.SessionID)
		}
		return ctx
	}
}

// newRequestID returns a short random hex string used as a per-request ID.
func newRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}
