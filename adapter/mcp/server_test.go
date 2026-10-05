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
