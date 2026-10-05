package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/port"
)

// TestLXCLifecycleToolSuccess exercises every LXC lifecycle mutation tool
// end-to-end with the gate ON.
func TestLXCLifecycleToolSuccess(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServer(t, app, true)

	cases := []struct {
		name string
		args map[string]any
	}{
		{"pve_lxc_start", map[string]any{"node": "pve1", "vmid": 200}},
		{"pve_lxc_stop", map[string]any{"node": "pve1", "vmid": 200, "skiplock": true, "force_stop": true}},
		{"pve_lxc_shutdown", map[string]any{"node": "pve1", "vmid": 200, "force_stop": true, "timeout": 45}},
		{"pve_lxc_reboot", map[string]any{"node": "pve1", "vmid": 200}},
		{"pve_lxc_delete", map[string]any{"node": "pve1", "vmid": 200, "purge": true, "destroy_unreferenced_disks": true, "force": true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := callTool(t, sess, tc.name, tc.args)
			assert.False(t, res.IsError, "tool %s should succeed", tc.name)
			assert.NotNil(t, res.StructuredContent, "tool %s should return structured output", tc.name)
		})
	}
}

// TestLXCLifecycleToolErrorPropagation verifies IsError on gateway failure.
func TestLXCLifecycleToolErrorPropagation(t *testing.T) {
	gw := &stubPVEGateway{err: port.ErrNotFound}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(gw))
	_, sess := newTestServer(t, app, true)

	cases := []struct {
		name string
		args map[string]any
	}{
		{"pve_lxc_start", map[string]any{"node": "pve1", "vmid": 200}},
		{"pve_lxc_stop", map[string]any{"node": "pve1", "vmid": 200}},
		{"pve_lxc_shutdown", map[string]any{"node": "pve1", "vmid": 200}},
		{"pve_lxc_reboot", map[string]any{"node": "pve1", "vmid": 200}},
		{"pve_lxc_delete", map[string]any{"node": "pve1", "vmid": 200}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := callTool(t, sess, tc.name, tc.args)
			assert.True(t, res.IsError, "tool %s should be flagged IsError on gateway failure", tc.name)
		})
	}
}

// TestLXCLifecycleToolGateOff verifies LXC lifecycle mutation tools are absent
// when the gate is OFF.
func TestLXCLifecycleToolGateOff(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServer(t, app, false)
	names := toolNames(t, sess)

	for _, n := range []string{"pve_lxc_start", "pve_lxc_stop", "pve_lxc_shutdown", "pve_lxc_reboot", "pve_lxc_delete"} {
		assert.False(t, names[n], "mutation tool %s must be gated off", n)
	}
}
