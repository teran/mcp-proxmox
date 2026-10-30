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
	"maps"
	"reflect"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-proxmox/adapter/logging"
	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/port"
)

// ServerInstructions is the server-level Instructions text advertised in the
// initialize result (M08/M09). It makes the server self-describing over the MCP
// protocol: what it is, how the optional backends are enabled, how tools are
// grouped, and how secrets are handled. It is shared between Deps.Instructions
// and ServerOptions.Instructions by the composition root (cmd/mcp-proxmox).
const ServerInstructions = "Local stdio MCP server exposing the Proxmox VE and Proxmox Backup " +
	"Server (PBS) REST APIs. Both backends are optional and enabled only when " +
	"their endpoint and API token are configured (via environment variables). " +
	"Tools are grouped read → write → delete by domain prefix: `pve_` (Proxmox " +
	"VE), `pbs_` (Proxmox Backup Server), plus system tools `ping`/`status`. " +
	"Read-only/query tools are always available; mutation tools are registered " +
	"only when ENABLE_MUTATIONS=true (default off). API tokens are configured " +
	"once per backend via environment variables, never per-call parameters and " +
	"never logged; TLS verification is always on. See the guidance resources " +
	"under mcp-proxmox://guidance/* (overview, tools, security) for detailed " +
	"usage."

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
	// EnableMutations gates ALL mutation tools (SPEC.md §2.5). adapter/mcp
	// registers mutation tools only when this is true; read-only (query) tools
	// are always registered for an enabled backend. The value comes from the
	// ENABLE_MUTATIONS env var (default false) and is set by the composition
	// root.
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
			logIncomingRequest(ctx, deps, method, req, res, time.Since(start), err)

			return res, err
		}
	})
	RegisterTools(s, deps.App, toolLogger{log: deps.Logger, l: deps.Log}, deps.EnableMutations)
	RegisterGuidanceResources(s)

	return s
}

// logIncomingRequest emits a structured per-request access-log line at info
// level (SPEC.md L8 — unconditionally when logging is enabled, NOT gated behind
// debug). For a tool call it carries the tool name and the REDACTED args
// (sanitizeArgs redacts secrets per S02, so no raw secret can reach the
// journal), plus source ("STDIO"), duration and outcome, tagged with
// session_id/request_id from ctx. Byte-size metrics (in_bytes/out_bytes) are
// confined to the trace line below. A nil logger is a silent no-op.
//
// At trace level it additionally emits a DEDICATED trace line (below the info
// line) that carries the tool name, redacted args and the byte-size/duration/
// outcome metrics, so per-tool payload sizes stay confined to trace. This line
// is trace-gated: it never appears unless LOG_LEVEL=trace.
func logIncomingRequest(
	ctx context.Context, deps Deps, method string, req mcpSDK.Request, res mcpSDK.Result, dur time.Duration, err error,
) {
	if deps.Log == nil {
		return
	}
	outcome := "ok"
	if err != nil {
		outcome = "error"
	}

	fields := logrus.Fields{
		"source":      "STDIO",
		"duration_ms": dur.Milliseconds(),
		"outcome":     outcome,
	}
	extra, traceFields := applyToolCall(req, res, deps.Log, dur, outcome)
	maps.Copy(fields, extra)

	e := logging.WithSession(ctx, deps.Log)
	e.WithFields(fields).Infof("mcp request: %s", method)
	if traceFields != nil {
		e.WithFields(traceFields).Tracef("mcp tool trace: %s", method)
	}
}

// applyToolCall enriches the request log fields for a tools/call request and
// returns the optional trace-level fields. For non-tool-call requests (e.g.
// initialize) it returns (nil, nil). The REDACTED args are placed on the info
// line (sanitizeArgs guarantees no secrets, S02), while byte-size metrics stay
// on the trace-only fields.
func applyToolCall(
	req mcpSDK.Request, res mcpSDK.Result, log *logrus.Logger, dur time.Duration, outcome string,
) (logrus.Fields, logrus.Fields) {
	name, args, ok := toolCallFromRequest(req)
	if !ok {
		return nil, nil
	}

	fields := logrus.Fields{"tool": name}
	if args != nil {
		fields["args"] = sanitizeArgs(args)
	}

	// Byte-size metrics are confined to the trace line (below); only a logrus
	// logger at trace level populates them.
	if !log.IsLevelEnabled(logrus.TraceLevel) {
		return fields, nil
	}
	if args != nil {
		if in, err := json.Marshal(args); err == nil {
			fields["in_bytes"] = len(in)
		}
	}
	if res != nil {
		if out, err := json.Marshal(res); err == nil {
			fields["out_bytes"] = len(out)
		}
	}

	traceFields := logrus.Fields{
		"tool":        name,
		"in_bytes":    fields["in_bytes"],
		"out_bytes":   fields["out_bytes"],
		"duration_ms": dur.Milliseconds(),
		"outcome":     outcome,
	}
	if args != nil {
		traceFields["args"] = sanitizeArgs(args)
	}

	return fields, traceFields
}

// toolCallFromRequest extracts the tool name and its arguments from a tools/call
// request. The go-sdk hands the receiving middleware a *CallToolParamsRaw (the
// wire form) for real tool calls, while unit tests may construct a
// *CallToolParams directly, so both shapes are supported. The returned args is
// an unmarshalled map (suitable for sanitizeArgs) or nil when there are none.
func toolCallFromRequest(req mcpSDK.Request) (name string, args any, ok bool) {
	switch p := req.GetParams().(type) {
	case *mcpSDK.CallToolParams:
		return p.Name, p.Arguments, true
	case *mcpSDK.CallToolParamsRaw:
		if len(p.Arguments) > 0 {
			var m map[string]any
			if err := json.Unmarshal(p.Arguments, &m); err == nil {
				return p.Name, m, true
			}
		}

		return p.Name, nil, true
	default:
		return "", nil, false
	}
}

// sanitizeArgs returns a JSON-ish string of the tool arguments, omitting any
// value whose key looks sensitive (token, password, secret, key, auth, cookie,
// credential, fingerprint, host_key) or whose field is tagged secret:"true"
// (S02). Typed struct inputs (e.g. pbsVMRestoreIn) are walked reflectively so a
// secret-tagged field is redacted even when its json name is not in the
// heuristic list; map inputs are handled by key name. Current tools take only
// node/vmid/store names, so this is defensive for future mutation tools.
func sanitizeArgs(args any) string {
	val := reflect.ValueOf(args)
	for val.Kind() == reflect.Pointer || val.Kind() == reflect.Interface {
		if val.IsNil() {
			return "{}"
		}
		val = val.Elem()
	}
	if val.IsValid() && val.Kind() == reflect.Struct {
		return sanitizeArgsStruct(val)
	}
	m, ok := args.(map[string]any)
	if !ok {
		b, err := json.Marshal(args)
		if err != nil {
			return "{}"
		}

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
	b, err := json.Marshal(clean)
	if err != nil {
		return "{}"
	}

	return string(b)
}

// sanitizeArgsStruct walks a typed tool-input struct and redacts any field
// tagged secret:"true" (S02) or whose json name looks sensitive, returning the
// remaining fields as a JSON object keyed by their json tag names.
func sanitizeArgsStruct(v reflect.Value) string {
	m := make(map[string]any, v.NumField())
	t := v.Type()
	for i := range v.NumField() {
		sf := t.Field(i)
		name := sf.Name
		if jsonTag := sf.Tag.Get("json"); jsonTag != "" {
			name = strings.Split(jsonTag, ",")[0]
		}
		if sf.Tag.Get("secret") == "true" || isSensitiveKey(name) {
			m[name] = "[redacted]"

			continue
		}
		m[name] = v.Field(i).Interface()
	}
	b, err := json.Marshal(m)
	if err != nil {
		return "{}"
	}

	return string(b)
}

// isSensitiveKey reports whether a tool-argument key should never be logged.
func isSensitiveKey(k string) bool {
	switch strings.ToLower(k) {
	case "token", "tokens", "password", "pass", "secret", "secret_key", "secretkey",
		"api_key", "apikey", "auth", "authorization", "cookie", "credential", "credentials",
		"fingerprint", "host_key", "hostkey":

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
	b := make([]byte, requestIDLen)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}

	return hex.EncodeToString(b)
}

// requestIDLen is the byte length of the random per-request / per-session IDs.
const requestIDLen = 8
