package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/model"
	"github.com/teran/mcp-proxmox/domain/port"
)

// TestPBSDatastoreConfigToolSuccess verifies pbs_datastore_config_get succeeds
// and returns the DatastoreConfig as structured output.
func TestPBSDatastoreConfigToolSuccess(t *testing.T) {
	gw := &stubPBSGateway{
		getDatastoreConfig: func(_ context.Context, store string) (*model.DatastoreConfig, error) {
			return &model.DatastoreConfig{
				Path: "/backup", Backend: "filesystem", KeepDaily: 7, VerifyNew: true,
			}, nil
		},
	}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPBS(gw))
	_, sess := newTestServer(t, app, false)

	res := callTool(t, sess, "pbs_datastore_config_get", map[string]any{"store": "backup"})
	assert.False(t, res.IsError, "tool should succeed")
	require.NotNil(t, res.StructuredContent, "tool should return structured output")
	sc, ok := res.StructuredContent.(map[string]any)
	require.True(t, ok, "structured content should be a map")
	assert.Equal(t, "/backup", sc["path"])
	assert.Equal(t, "filesystem", sc["backend"])
	assert.Equal(t, float64(7), sc["keep-daily"])
	assert.Equal(t, true, sc["verify-new"])
}

// TestPBSDatastoreConfigToolErrorPropagation verifies IsError on gateway
// failure.
func TestPBSDatastoreConfigToolErrorPropagation(t *testing.T) {
	gw := &stubPBSGateway{err: port.ErrNotFound}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPBS(gw))
	_, sess := newTestServer(t, app, false)

	res := callTool(t, sess, "pbs_datastore_config_get", map[string]any{"store": "missing"})
	assert.True(t, res.IsError, "tool should be flagged IsError on gateway failure")
}

// TestPBSDatastoreConfigToolDisabled verifies pbs_datastore_config_get is NOT
// registered when the PBS backend is disabled.
func TestPBSDatastoreConfigToolDisabled(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0")
	_, sess := newTestServer(t, app, false)

	names := toolNames(t, sess)
	assert.False(t, names["pbs_datastore_config_get"], "pbs tool must be absent when PBS disabled")
}
