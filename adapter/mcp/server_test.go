package mcp

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/port"
)

// TestSanitizeArgs verifies sensitive argument keys are redacted and non-map
// values pass through.
func TestSanitizeArgs(t *testing.T) {
	out := sanitizeArgs(map[string]any{
		"node":     "pve1",
		"password": "hunter2",
		"token":    "secret",
		"vmid":     100,
	})
	assert.Contains(t, out, `"node":"pve1"`)
	assert.Contains(t, out, `"vmid":100`)
	assert.NotContains(t, out, "hunter2")
	assert.NotContains(t, out, "secret")
	assert.Contains(t, out, "[redacted]")

	// Non-map input (e.g. a scalar) is serialized as-is.
	scalar := sanitizeArgs("just-a-string")
	assert.Equal(t, `"just-a-string"`, scalar)
}

func TestIsSensitiveKey(t *testing.T) {
	for _, k := range []string{"token", "password", "secret_key", "auth", "api_key", "cookie", "credential",
		"fingerprint", "host_key", "hostkey"} {
		assert.True(t, isSensitiveKey(k), "expected %q sensitive", k)
	}
	for _, k := range []string{"node", "vmid", "store", "upid", "backup_id"} {
		assert.False(t, isSensitiveKey(k), "expected %q not sensitive", k)
	}
}

// secretTagInput models a tool input struct whose sensitive fields are tagged
// `secret:"true"` (S02 conformance fix). Fields tagged this way must be redacted
// in sanitized args even though their json names are not in the heuristic list.
type secretTagInput struct {
	Node        string `json:"node"`
	Password    string `json:"password,omitempty" secret:"true"`
	Fingerprint string `json:"fingerprint,omitempty" secret:"true"`
	Pool        string `json:"pool,omitempty"`
}

// TestSanitizeArgsSecretTag verifies sanitizeArgs honors the `secret:"true"`
// struct tag (reflect-based) in addition to the key-name heuristic, so a secret
// carried in a typed struct input is redacted and never reaches the log.
func TestSanitizeArgsSecretTag(t *testing.T) {
	out := sanitizeArgs(secretTagInput{
		Node:        "pve1",
		Password:    "hunter2",
		Fingerprint: "SHA256:AA:BB",
		Pool:        "pool1",
	})

	assert.Contains(t, out, `"node":"pve1"`)
	assert.Contains(t, out, `"pool":"pool1"`)
	assert.NotContains(t, out, "hunter2", "password tagged secret must be redacted")
	assert.NotContains(t, out, "SHA256:AA:BB", "fingerprint tagged secret must be redacted")
	assert.Contains(t, out, "[redacted]")
}

// TestLogIncomingRequest verifies the per-request access log is emitted at
// info level (L08 — NOT gated behind debug), carries tool/source/duration/
// outcome fields and the session/request IDs, and redacts sensitive args.
func TestLogIncomingRequest(t *testing.T) {
	var buf bytes.Buffer
	l := logrus.New()
	l.SetOutput(&buf)
	l.SetFormatter(&logrus.JSONFormatter{})
	l.SetLevel(logrus.InfoLevel)

	deps := Deps{Log: l}
	ctx := port.WithSessionID(context.Background(), "sess")
	ctx = port.WithRequestID(ctx, "req")

	req := &mcpSDK.ServerRequest[*mcpSDK.CallToolParams]{
		Params: &mcpSDK.CallToolParams{Name: "pve_node_list", Arguments: map[string]any{"node": "pve1", "token": "x"}},
	}
	logIncomingRequest(deps, ctx, "tools/call", req, nil, 5*time.Millisecond, nil)

	out := buf.String()
	assert.Contains(t, out, `"tool":"pve_node_list"`)
	assert.Contains(t, out, `"source":"STDIO"`)
	assert.Contains(t, out, `"outcome":"ok"`)
	assert.Contains(t, out, `"session_id":"sess"`)
	assert.Contains(t, out, `"request_id":"req"`)
	assert.NotContains(t, out, `"token":"x"`)

	// Error path.
	buf.Reset()
	logIncomingRequest(deps, ctx, "tools/call", req, nil, 3*time.Millisecond, errors.New("boom"))
	assert.Contains(t, buf.String(), `"outcome":"error"`)
}

// TestLogIncomingRequestAtInfoLevel verifies the per-request access line is
// emitted at info level even when trace/debug are disabled (L08 conformance
// fix): the line must NOT be suppressed at Info. A nil logger stays a silent
// no-op.
func TestLogIncomingRequestAtInfoLevel(t *testing.T) {
	var buf bytes.Buffer
	l := logrus.New()
	l.SetOutput(&buf)
	l.SetFormatter(&logrus.JSONFormatter{})
	l.SetLevel(logrus.InfoLevel) // debug/trace disabled, info enabled

	deps := Deps{Log: l}
	ctx := port.WithSessionID(context.Background(), "sess")
	req := &mcpSDK.ServerRequest[*mcpSDK.CallToolParams]{Params: &mcpSDK.CallToolParams{Name: "x"}}

	logIncomingRequest(deps, ctx, "tools/call", req, nil, time.Millisecond, nil)

	// The access line must be present at info level.
	assert.NotEmpty(t, buf.String(), "access log must be emitted at info level")
	assert.Contains(t, buf.String(), `"source":"STDIO"`)
	assert.Contains(t, buf.String(), `"session_id":"sess"`)

	// nil logger is a silent no-op: no panic, no output.
	assert.NotPanics(t, func() {
		logIncomingRequest(Deps{}, ctx, "tools/call", req, nil, time.Millisecond, nil)
	})
}

// newTestServerWithLogger builds an mcpSDK.Server wired through the real
// application.App and RegisterTools path with an injected logrus logger (so
// per-tool trace lines can be captured), returning the server plus a connected
// client session.
func newTestServerWithLogger(t *testing.T, app *application.App, l *logrus.Logger) (*mcpSDK.Server, *mcpSDK.ClientSession) {
	t.Helper()
	s := NewServer(
		&mcpSDK.Implementation{Name: "mcp-proxmox-test", Version: "v0.0.0-test"},
		Deps{
			App:             app,
			Logger:          nopLogger{},
			Log:             l,
			SessionID:       "test-session",
			EnableMutations: false,
		},
		nil,
	)
	t1, t2 := mcpSDK.NewInMemoryTransports()
	if _, err := s.Connect(context.Background(), t1, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcpSDK.NewClient(&mcpSDK.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	sess, err := client.Connect(context.Background(), t2, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return s, sess
}

// traceLineContaining returns the first captured logrus line (JSON) whose level
// is trace and which contains the given substring, or "" if none matches.
func traceLineContaining(out, sub string) string {
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		if strings.Contains(line, `"level":"trace"`) && strings.Contains(line, sub) {
			return line
		}
	}
	return ""
}

// TestPerToolTraceLine verifies that, at LOG_LEVEL=trace, a real tool call
// through the server additionally emits a dedicated trace-level line carrying
// the tool name and the redacted args/byte-size fields, while the info access
// line (L08) remains present.
func TestPerToolTraceLine(t *testing.T) {
	var buf bytes.Buffer
	l := logrus.New()
	l.SetOutput(&buf)
	l.SetFormatter(&logrus.JSONFormatter{})
	l.SetLevel(logrus.TraceLevel)

	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServerWithLogger(t, app, l)

	callTool(t, sess, "pve_vm_list", map[string]any{"node": "pve1"})

	out := buf.String()
	traceLine := traceLineContaining(out, `"tool":"pve_vm_list"`)
	assert.NotEmpty(t, traceLine, "expected a trace-level line with the tool name")
	assert.Contains(t, traceLine, `"in_bytes":`)
	assert.Contains(t, traceLine, `"out_bytes":`)
	assert.Contains(t, traceLine, `"duration_ms":`)
	assert.Contains(t, traceLine, `"outcome":"ok"`)
	// The existing info access line must still be present.
	assert.Contains(t, out, `"level":"info"`)
}

// TestPerToolTraceLineRedactsSensitiveArgs verifies a sensitive tool argument
// (e.g. password) never reaches the trace line: it is shown as [redacted].
func TestPerToolTraceLineRedactsSensitiveArgs(t *testing.T) {
	var buf bytes.Buffer
	l := logrus.New()
	l.SetOutput(&buf)
	l.SetFormatter(&logrus.JSONFormatter{})
	l.SetLevel(logrus.TraceLevel)

	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServerWithLogger(t, app, l)

	callTool(t, sess, "pve_vm_list", map[string]any{"node": "pve1", "password": "hunter2"})

	out := buf.String()
	traceLine := traceLineContaining(out, `"tool":"pve_vm_list"`)
	assert.NotEmpty(t, traceLine, "expected a trace-level line with the tool name")
	assert.Contains(t, traceLine, "[redacted]")
	assert.NotContains(t, traceLine, "hunter2", "sensitive argument must be redacted in the trace line")
}

// TestPerToolTraceLineInfoGated verifies the per-tool trace line is NOT emitted
// when logrus is at info level (trace disabled) — it is trace-gated.
func TestPerToolTraceLineInfoGated(t *testing.T) {
	var buf bytes.Buffer
	l := logrus.New()
	l.SetOutput(&buf)
	l.SetFormatter(&logrus.JSONFormatter{})
	l.SetLevel(logrus.InfoLevel)

	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServerWithLogger(t, app, l)

	callTool(t, sess, "pve_vm_list", map[string]any{"node": "pve1"})

	out := buf.String()
	assert.NotContains(t, out, `"level":"trace"`, "per-tool trace line must be trace-gated")
	// The info access line still appears.
	assert.Contains(t, out, `"level":"info"`)
}
