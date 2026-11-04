package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-proxmox/domain/port"
)

// outStruct is a small concrete output model used by the output-schema tests.
type outStruct struct {
	Name string `json:"name"`
	ID   int    `json:"id,omitempty"`
}

// recordingLogger captures Errorf lines for asserting that non-conforming
// output is logged (S09/N22).
type recordingLogger struct {
	errors []string
}

func (r *recordingLogger) Tracef(string, ...any) {}
func (r *recordingLogger) Debugf(string, ...any) {}
func (r *recordingLogger) Infof(string, ...any)  {}
func (r *recordingLogger) Warnf(string, ...any)  {}
func (r *recordingLogger) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

var _ port.AppLogger = (*recordingLogger)(nil)

func TestOutputSchema(t *testing.T) {
	tests := []struct {
		name     string
		build    func() *jsonschema.Schema
		wantType string
		wantProp string
	}{
		{
			name:     "struct passes through",
			build:    func() *jsonschema.Schema { return outputSchema[outStruct]() },
			wantType: "object",
			wantProp: "name",
		},
		{
			name:     "pointer to struct dereferenced",
			build:    func() *jsonschema.Schema { return outputSchema[*outStruct]() },
			wantType: "object",
			wantProp: "name",
		},
		{
			name:     "slice wrapped as items",
			build:    func() *jsonschema.Schema { return outputSchema[[]outStruct]() },
			wantType: "object",
			wantProp: "items",
		},
		{
			name:     "scalar wrapped as value",
			build:    func() *jsonschema.Schema { return outputSchema[int]() },
			wantType: "object",
			wantProp: "value",
		},
		{
			// The `any` branch must produce a concrete object schema, never an
			// empty one: the go-sdk serializes an empty *jsonschema.Schema as the
			// boolean `true`, which MCP clients (e.g. opencode) reject ("expected
			// object, received boolean") and drop the tool. {"type":"object"} is
			// permissive yet a valid object.
			name:     "any is permissive object schema",
			build:    func() *jsonschema.Schema { return outputSchema[any]() },
			wantType: "object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := tt.build()
			require.NotNil(t, schema)
			assert.Equal(t, tt.wantType, schema.Type)
			if tt.wantProp != "" {
				require.Contains(t, schema.Properties, tt.wantProp)
			} else {
				assert.Empty(t, schema.Properties, "permissive schema should carry no properties")
			}
		})
	}
}

func TestValidateOutput(t *testing.T) {
	schema := outputSchema[outStruct]()

	t.Run("conforming output passes", func(t *testing.T) {
		assert.NoError(t, validateOutput(schema, outStruct{Name: "x"}))
	})

	t.Run("non-conforming output rejected", func(t *testing.T) {
		// "name" must be a string; a wrong-typed value must be rejected.
		err := validateOutput(schema, map[string]any{"name": 123})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "outputSchema")
	})

	t.Run("nil output short-circuits", func(t *testing.T) {
		assert.NoError(t, validateOutput(schema, nil))
	})

	t.Run("nil schema short-circuits", func(t *testing.T) {
		assert.NoError(t, validateOutput(nil, outStruct{Name: "x"}))
	})
}

func TestWrapAndValidate(t *testing.T) {
	t.Run("conforming output is wrapped and returned", func(t *testing.T) {
		log := &recordingLogger{}
		schema := outputSchema[[]outStruct]()
		wrapped, err := wrapAndValidate(context.Background(), "test_tool", toolLogger{log: log}, schema, []outStruct{{Name: "a"}})
		require.NoError(t, err)
		m, ok := wrapped.(map[string]any)
		require.True(t, ok)
		_, hasItems := m["items"]
		assert.True(t, hasItems)
		assert.Empty(t, log.errors, "no error expected for conforming output")
	})

	t.Run("non-conforming output rejected and logged", func(t *testing.T) {
		log := &recordingLogger{}
		// A schema that requires a "name" string property.
		schema := &jsonschema.Schema{
			Type:     "object",
			Required: []string{"name"},
			Properties: map[string]*jsonschema.Schema{
				"name": {Type: "string"},
			},
		}
		wrapped, err := wrapAndValidate(context.Background(), "test_tool", toolLogger{log: log}, schema, map[string]any{"other": "x"})
		require.Error(t, err)
		assert.Nil(t, wrapped)
		assert.Contains(t, err.Error(), "outputSchema")
		require.Len(t, log.errors, 1, "non-conforming output must be logged")
		assert.Contains(t, log.errors[0], "test_tool")
	})
}

// TestToolExposesOutputSchema verifies that a tool registered through roTool with
// a concrete output type advertises a derived outputSchema and that a conforming
// call passes (a/b/d).
func TestToolExposesOutputSchema(t *testing.T) {
	ro := func(_ context.Context, _ emptyIn) (outStruct, error) {
		return outStruct{Name: "web"}, nil
	}

	s := mcpSDK.NewServer(&mcpSDK.Implementation{Name: "t", Version: "v"}, nil)
	roTool(s, "test_ro_schema", "Test RO", "a test tool", "instructions", toolLogger{log: nopLogger{}}, ro)

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

	tools := toolsByName(t, sess)
	tl, ok := tools["test_ro_schema"]
	require.True(t, ok)
	require.NotNil(t, tl.OutputSchema, "tool must expose an outputSchema")

	res := callTool(t, sess, "test_ro_schema", map[string]any{})
	assert.False(t, res.IsError)
	sc, ok := res.StructuredContent.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "web", sc["name"])
}

// TestAnyOutputSchemaIsObjectNotBoolean is a regression guard for the opencode
// ingestion bug: when a tool's handler returns `any` (the shape used by every
// mcp-proxmox tool), the published outputSchema must serialize to a JSON OBJECT
// (e.g. {"type":"object"}), never the boolean `true`. opencode rejects a boolean
// outputSchema ("expected object, received boolean") and silently drops the
// whole server's tools, so this guards the exact wire contract that keeps the
// tools visible to opencode agent sessions.
func TestAnyOutputSchemaIsObjectNotBoolean(t *testing.T) {
	ro := func(_ context.Context, _ emptyIn) (any, error) {
		return map[string]any{"status": "ok"}, nil
	}

	s := mcpSDK.NewServer(&mcpSDK.Implementation{Name: "t", Version: "v"}, nil)
	roTool(s, "test_any_out", "Any", "a tool returning any", "instructions", toolLogger{log: nopLogger{}}, ro)

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

	tools := toolsByName(t, sess)
	tl, ok := tools["test_any_out"]
	require.True(t, ok, "tool must be published")
	require.NotNil(t, tl.OutputSchema, "tool must expose an outputSchema")

	// Marshal the wire outputSchema and assert it is a JSON object, not `true`.
	raw, err := json.Marshal(tl.OutputSchema)
	require.NoError(t, err)
	require.NotEqual(t, "true", string(raw), "outputSchema must never serialize to boolean true (opencode rejects it)")
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m), "outputSchema must be a JSON object")
	assert.Equal(t, "object", m["type"], "permissive outputSchema should be {\"type\":\"object\"}")
}
