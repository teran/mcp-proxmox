// Package mcp assembles the go-sdk MCP server and registers the tool registry.
// It is the primary "interface" adapter: it translates between MCP tool calls
// and the application layer. This is the ONLY place (besides cmd/mcp-proxmox)
// that imports the go-sdk. See SPEC.md §2.3 / §6.
package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"

	"github.com/sirupsen/logrus"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"

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
	// request, so all log lines from a single tool call share it.
	s.AddReceivingMiddleware(func(next mcpSDK.MethodHandler) mcpSDK.MethodHandler {
		return func(ctx context.Context, method string, req mcpSDK.Request) (mcpSDK.Result, error) {
			return next(withSessionContext(deps)(ctx), method, req)
		}
	})
	RegisterTools(s, deps.App, toolLogger{log: deps.Logger, l: deps.Log}, deps.EnableMutations)
	return s
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
