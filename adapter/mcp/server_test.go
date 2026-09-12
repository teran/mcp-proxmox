package mcp

import (
	"bytes"
	"context"
	"errors"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"

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
	for _, k := range []string{"token", "password", "secret_key", "auth", "api_key", "cookie", "credential"} {
		assert.True(t, isSensitiveKey(k), "expected %q sensitive", k)
	}
	for _, k := range []string{"node", "vmid", "store", "upid", "backup_id"} {
		assert.False(t, isSensitiveKey(k), "expected %q not sensitive", k)
	}
}

// TestLogIncomingRequest verifies the per-request structured log is emitted
// only when debug is enabled, carries tool/source/duration/outcome fields and
// the session/request IDs, and redacts args.
func TestLogIncomingRequest(t *testing.T) {
	var buf bytes.Buffer
	l := logrus.New()
	l.SetOutput(&buf)
	l.SetFormatter(&logrus.JSONFormatter{})
	l.SetLevel(logrus.DebugLevel)

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

// TestLogIncomingRequestDisabled verifies nothing is logged when debug is off or
// the logger is nil.
func TestLogIncomingRequestDisabled(t *testing.T) {
	var buf bytes.Buffer
	l := logrus.New()
	l.SetOutput(&buf)
	l.SetLevel(logrus.InfoLevel) // debug disabled

	deps := Deps{Log: l}
	ctx := context.Background()
	req := &mcpSDK.ServerRequest[*mcpSDK.CallToolParams]{Params: &mcpSDK.CallToolParams{Name: "x"}}
	logIncomingRequest(deps, ctx, "tools/call", req, nil, time.Millisecond, nil)
	assert.Empty(t, buf.String())

	// nil logger is a no-op.
	assert.NotPanics(t, func() {
		logIncomingRequest(Deps{}, ctx, "tools/call", req, nil, time.Millisecond, nil)
	})
}
