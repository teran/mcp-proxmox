package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/port"
)

// TestHARemoveToolForwardsSID verifies the pve_ha_remove mutation tool forwards
// the provided SID to PVEGateway.RemoveHAResource and returns success (no
// output value, mirroring pve_task_stop).
func TestHARemoveToolForwardsSID(t *testing.T) {
	gotSID := ""
	gateway := &stubPVEGateway{removeHAResource: func(_ context.Context, sid string) error {
		gotSID = sid

		return nil
	}}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(gateway))
	_, sess := newTestServer(t, app, true)

	res := callTool(t, sess, "pve_ha_remove", map[string]any{"sid": "vm:100"})
	assert.False(t, res.IsError, "tool should succeed")
	assert.Equal(t, "vm:100", gotSID)
}

// TestHARemoveToolErrorPropagation verifies IsError on gateway failure.
func TestHARemoveToolErrorPropagation(t *testing.T) {
	gateway := &stubPVEGateway{err: port.ErrNotFound}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(gateway))
	_, sess := newTestServer(t, app, true)

	res := callTool(t, sess, "pve_ha_remove", map[string]any{"sid": "vm:100"})
	require.NotNil(t, res)
	assert.True(t, res.IsError, "tool should be flagged IsError on gateway failure")
}
