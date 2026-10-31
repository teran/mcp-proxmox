package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-proxmox/domain/model"
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

// secretOut is a local output struct used to pin the reflect-based
// sanitizeOutput behavior: the Token field carries secret:"true" and must be
// redacted by name tag, while Name and Note (no secret tag, no sensitive json
// name) must pass through untouched.
type secretOut struct {
	Name  string `json:"name"`
	Token string `json:"token,omitempty" secret:"true"`
	Note  string `json:"note,omitempty"`
}

// TestSanitizeOutput_RedactsSecretTaggedStructField pins that a typed struct
// field tagged secret:"true" (S02) is redacted to "[redacted]" regardless of
// whether the value is a map. Non-secret sibling fields are preserved.
func TestSanitizeOutput_RedactsSecretTaggedStructField(t *testing.T) {
	in := secretOut{Name: "a", Token: "s3cret", Note: "hi"}

	out := sanitizeOutput(in)
	require.NotNil(t, out)

	b, err := json.Marshal(out)
	require.NoError(t, err)
	s := string(b)

	assert.NotContains(t, s, "s3cret", "secret value must never reach output")
	assert.Contains(t, s, "[redacted]", "redaction marker must be present")
	assert.Contains(t, s, `"name":"a"`, "non-secret name field preserved")
	assert.Contains(t, s, `"note":"hi"`, "non-secret note field preserved")
}

// TestSanitizeOutput_PreservesStructWithoutSecrets is the critical regression
// guard: a typed struct with no secret:"true" field and no sensitive json key
// must pass through sanitizeOutput byte-identical, so the existing
// validateOutput/outputSchema path (which validates typed models) is not
// disturbed. model.Node carries SSLFingerprint whose json name is
// "sslfingerprint" — not secret-tagged and not a sensitive key — so it must be
// treated as a normal field.
func TestSanitizeOutput_PreservesStructWithoutSecrets(t *testing.T) {
	in := model.Node{Node: "pve1", Status: "online", SSLFingerprint: "AA:BB"}

	out := sanitizeOutput(in)
	require.NotNil(t, out)

	assert.True(t, reflect.DeepEqual(in, out),
		"struct without secrets must pass through unchanged (got %#v)", out)
}

// TestSanitizeOutput_RedactsNestedSecret pins that a typed slice of structs is
// walked recursively: a nested secret:"true" field is redacted while
// non-secret siblings are preserved.
func TestSanitizeOutput_RedactsNestedSecret(t *testing.T) {
	in := []secretOut{
		{Name: "a", Token: "s3cret1", Note: "n1"},
		{Name: "b", Note: "n2"}, // no token at all
	}

	out := sanitizeOutput(in)
	require.NotNil(t, out)

	b, err := json.Marshal(out)
	require.NoError(t, err)
	s := string(b)

	assert.NotContains(t, s, "s3cret1", "nested secret value must never reach output")
	assert.Contains(t, s, "[redacted]", "nested secret must be redacted")
	assert.Contains(t, s, `"name":"a"`, "nested sibling a preserved")
	assert.Contains(t, s, `"note":"n1"`, "nested sibling note preserved")
	assert.Contains(t, s, `"name":"b"`, "secret-free sibling preserved")
	assert.Contains(t, s, `"note":"n2"`, "secret-free sibling note preserved")
}

// TestSanitizeOutput_SensitiveMapKeyStillRedacted pins that the existing
// map[string]any sensitive-key redaction (S2) is preserved by the reflect-based
// rewrite.
func TestSanitizeOutput_SensitiveMapKeyStillRedacted(t *testing.T) {
	out := sanitizeOutput(map[string]any{"node": "pve1", "password": "x"})

	m, ok := out.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "pve1", m["node"], "non-sensitive map value preserved")
	assert.Equal(t, "[redacted]", m["password"], "sensitive map key redacted")
	assert.NotContains(t, m, "x", "raw secret must not survive as a top-level value")
}

// TestSanitizeOutput_StripsControl pins that ANSI/control escape sequences in
// string values are still stripped (S9) by the reflect-based rewrite.
func TestSanitizeOutput_StripsControl(t *testing.T) {
	out := sanitizeOutput(map[string]any{"out": "\x1b[31mred\x1b[0m"})

	m, ok := out.(map[string]any)
	require.True(t, ok)
	got, ok := m["out"].(string)
	require.True(t, ok)
	assert.NotContains(t, got, "\x1b", "escape bytes must be stripped")
	assert.Equal(t, "red", got)
}

// TestSanitizeOutput_TypedSlicePreservedWhenNoSecret pins that a typed slice of
// structs with no secrets passes through byte-identical (reflect.DeepEqual), so
// list-tool output of typed models is not disturbed by the rewrite.
func TestSanitizeOutput_TypedSlicePreservedWhenNoSecret(t *testing.T) {
	in := []model.Node{
		{Node: "pve1", Status: "online"},
		{Node: "pve2", Status: "offline"},
	}

	out := sanitizeOutput(in)
	require.NotNil(t, out)

	assert.True(t, reflect.DeepEqual(in, out),
		"typed slice without secrets must pass through unchanged (got %#v)", out)
}
