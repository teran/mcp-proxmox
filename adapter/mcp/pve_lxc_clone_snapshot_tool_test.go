package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/port"
)

// TestLXCCloneSnapshotToolSuccess exercises the LXC clone and snapshot tools
// (mutation gate ON) plus the read-only snapshot list tool.
func TestLXCCloneSnapshotToolSuccess(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServer(t, app, true)

	cases := []struct {
		name string
		args map[string]any
	}{
		{"pve_lxc_snapshot_list", map[string]any{"node": "pve1", "vmid": 200}},
		{"pve_lxc_clone", map[string]any{"node": "pve1", "vmid": 200, "newid": 201, "hostname": "ct-clone", "full": true}},
		{"pve_lxc_snapshot_create", map[string]any{"node": "pve1", "vmid": 200, "snapname": "snap1", "description": "d"}},
		{"pve_lxc_snapshot_delete", map[string]any{"node": "pve1", "vmid": 200, "snapname": "snap1"}},
		{"pve_lxc_snapshot_rollback", map[string]any{"node": "pve1", "vmid": 200, "snapname": "snap1"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := callTool(t, sess, tc.name, tc.args)
			assert.False(t, res.IsError, "tool %s should succeed", tc.name)
			assert.NotNil(t, res.StructuredContent, "tool %s should return structured output", tc.name)
		})
	}
}

// TestLXCCloneSnapshotToolErrorPropagation verifies IsError on gateway failure.
func TestLXCCloneSnapshotToolErrorPropagation(t *testing.T) {
	gw := &stubPVEGateway{err: port.ErrNotFound}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(gw))
	_, sess := newTestServer(t, app, true)

	cases := []struct {
		name string
		args map[string]any
	}{
		{"pve_lxc_snapshot_list", map[string]any{"node": "pve1", "vmid": 200}},
		{"pve_lxc_clone", map[string]any{"node": "pve1", "vmid": 200, "newid": 201}},
		{"pve_lxc_snapshot_create", map[string]any{"node": "pve1", "vmid": 200, "snapname": "snap1"}},
		{"pve_lxc_snapshot_delete", map[string]any{"node": "pve1", "vmid": 200, "snapname": "snap1"}},
		{"pve_lxc_snapshot_rollback", map[string]any{"node": "pve1", "vmid": 200, "snapname": "snap1"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := callTool(t, sess, tc.name, tc.args)
			assert.True(t, res.IsError, "tool %s should be flagged IsError on gateway failure", tc.name)
		})
	}
}

// TestLXCCloneSnapshotToolGateOff verifies the mutation clone/snapshot tools are
// absent when the gate is OFF, while the read-only snapshot list stays present.
func TestLXCCloneSnapshotToolGateOff(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServer(t, app, false)
	names := toolNames(t, sess)

	assert.True(t, names["pve_lxc_snapshot_list"], "read-only snapshot list must be present")
	for _, n := range []string{"pve_lxc_clone", "pve_lxc_snapshot_create", "pve_lxc_snapshot_delete", "pve_lxc_snapshot_rollback"} {
		assert.False(t, names[n], "mutation tool %s must be gated off", n)
	}
}
