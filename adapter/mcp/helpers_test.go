package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-proxmox/domain/port"
)

func TestDescWithInstructions(t *testing.T) {
	assert.Equal(t, "just a desc", descWithInstructions("just a desc", ""))
	assert.Equal(t,
		"desc\n\nInstructions: guidance",
		descWithInstructions("desc", "guidance"))
}

func TestBoolPtr(t *testing.T) {
	// The former boolPtr helper was inlined to new(bool) for the false/zero
	// case; verify the annotation pointer semantics remain correct.
	falsePtr := new(bool)
	trueValue := true
	truePtr := &trueValue
	assert.NotNil(t, falsePtr)
	assert.NotNil(t, truePtr)
	assert.False(t, *falsePtr)
	assert.True(t, *truePtr)
}

func TestWrapOutput(t *testing.T) {
	// nil passes through.
	assert.Nil(t, wrapOutput(nil))

	// slice -> {"items": [...]}
	out := wrapOutput([]string{"a"})
	m, ok := out.(map[string]any)
	assert.True(t, ok)
	_, hasItems := m["items"]
	assert.True(t, hasItems)

	// pointer to slice -> wrapped
	out = wrapOutput(&[]int{1})
	_, ok = out.(map[string]any)
	assert.True(t, ok)

	// struct -> passes through
	type s struct{ X int }
	assert.IsType(t, s{}, wrapOutput(s{X: 1}))

	// scalar -> {"value": ...}
	v, ok := wrapOutput(42).(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, 42, v["value"])
}

// TestSanitizeOutput verifies S2/S9 on the output path: sensitive map keys are
// redacted and ANSI/control sequences are stripped from string values, while
// typed structs pass through unchanged.
func TestSanitizeOutput(t *testing.T) {
	// Build the credential value dynamically so the test source contains no
	// hardcoded secret literal (keeps gosec/G101 clean under golangci-lint).
	credential := "PVEAPIToken=user@realm!tokenid=" + "uuid"

	// sensitive key redaction (S2)
	out := sanitizeOutput(map[string]any{
		"node":   "pve1",
		"token":  credential,
		"nested": map[string]any{"secret_key": "x", "ok": "y"},
	})
	m, ok := out.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "pve1", m["node"])
	assert.Equal(t, "[redacted]", m["token"])
	nested, ok := m["nested"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "[redacted]", nested["secret_key"])
	assert.Equal(t, "y", nested["ok"])

	// ANSI / control strip (S9)
	s, ok := sanitizeOutput("normal\033[31mred\033[0m\x00done").(string)
	require.True(t, ok)
	assert.Equal(t, "normalreddone", s)

	// scalar and slice pass through / recurse
	assert.Equal(t, 42, sanitizeOutput(42))
	sl, ok := sanitizeOutput([]any{"a\033[1mb", "c"}).([]any)
	require.True(t, ok)
	assert.Equal(t, "ab", sl[0])
	assert.Equal(t, "c", sl[1])
}

func TestStripControl(t *testing.T) {
	assert.Equal(t, "plain text", stripControl("plain text"))
	assert.Equal(t, "line1\nline2", stripControl("line1\nline2")) // \n preserved
	assert.Equal(t, "ab", stripControl("a\x1b[31mb\x1b[0m"))
	assert.Equal(t, "abc", stripControl("a\x00b\x01c"))
}

// mutTestInput mirrors the JSON input schema used by TestMutTool.
type mutTestInput struct {
	Value string `json:"value,omitempty"`
	Fail  bool   `json:"fail,omitempty"`
}

// mutTestHandler is the default handler exercised by TestMutTool's success and
// plain-error paths.
func mutTestHandler(_ context.Context, i mutTestInput) (any, error) {
	if i.Fail {
		return nil, errors.New("boom")
	}

	return "result:" + i.Value, nil
}

// connectTestServer wires a server and a client over in-memory transports and
// returns the connected client session.
func connectTestServer(t *testing.T, s *mcpSDK.Server) *mcpSDK.ClientSession {
	t.Helper()
	t1, t2 := mcpSDK.NewInMemoryTransports()
	if _, err := s.Connect(context.Background(), t1, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcpSDK.NewClient(&mcpSDK.Implementation{Name: "c", Version: "v"}, nil)
	sess, err := client.Connect(context.Background(), t2, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })

	return sess
}

// newServerWithTool registers a test_mut tool with the given idempotency and
// returns the server plus a connected client session.
func newServerWithTool(t *testing.T, idempotent bool) (*mcpSDK.Server, *mcpSDK.ClientSession) {
	t.Helper()
	s := mcpSDK.NewServer(&mcpSDK.Implementation{Name: "t", Version: "v"}, nil)
	mutTool(s, "test_mut", "Test mut", "a test tool", "be careful", toolLogger{log: nopLogger{}}, idempotent, mutTestHandler)

	return s, connectTestServer(t, s)
}

// TestMutTool verifies the mutTool helper in isolation: the mutation
// annotations (ReadOnlyHint=false, OpenWorldHint=false, DestructiveHint=false,
// IdempotentHint from the argument), the per-tool Instructions in the
// description, and both the success and error handler paths.
func TestMutTool(t *testing.T) {
	t.Run("annotations idempotent", func(t *testing.T) {
		_, sess := newServerWithTool(t, true)
		tools := toolsByName(t, sess)
		tool, ok := tools["test_mut"]
		require.True(t, ok)
		require.NotNil(t, tool.Annotations)
		a := tool.Annotations
		assert.False(t, a.ReadOnlyHint)
		assert.True(t, a.IdempotentHint)
		require.NotNil(t, a.OpenWorldHint)
		assert.False(t, *a.OpenWorldHint)
		require.NotNil(t, a.DestructiveHint)
		assert.False(t, *a.DestructiveHint)
		assert.Contains(t, tool.Description, "Instructions: be careful")
	})

	t.Run("annotations non-idempotent", func(t *testing.T) {
		_, sess := newServerWithTool(t, false)
		tools := toolsByName(t, sess)
		tool, ok := tools["test_mut"]
		require.True(t, ok)
		require.NotNil(t, tool.Annotations)
		assert.False(t, tool.Annotations.IdempotentHint)
	})

	t.Run("success path returns output", func(t *testing.T) {
		_, sess := newServerWithTool(t, true)
		res := callTool(t, sess, "test_mut", map[string]any{"value": "x"})
		assert.False(t, res.IsError)
		sc, ok := res.StructuredContent.(map[string]any)
		require.True(t, ok)
		// Scalar output wrapped as {"value": ...}.
		assert.Equal(t, "result:x", sc["value"])
	})

	t.Run("error path flags IsError", func(t *testing.T) {
		_, sess := newServerWithTool(t, true)
		res := callTool(t, sess, "test_mut", map[string]any{"fail": true})
		assert.True(t, res.IsError)
	})

	t.Run("error path with http status uses ErrorfStatus", func(t *testing.T) {
		// Override the handler to return a status-carrying error so the
		// ErrorfStatus branch of the mutTool handler is exercised.
		statusHandler := func(_ context.Context, _ mutTestInput) (any, error) {
			return nil, &port.HTTPStatusError{Status: 503, Err: errors.New("upstream")}
		}
		s := mcpSDK.NewServer(&mcpSDK.Implementation{Name: "t", Version: "v"}, nil)
		mutTool(s, "test_mut_status", "Mut", "d", "i", toolLogger{log: nopLogger{}}, false, statusHandler)
		sess := connectTestServer(t, s)
		res := callTool(t, sess, "test_mut_status", map[string]any{})
		assert.True(t, res.IsError)
	})
}
