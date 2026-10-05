package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/port"
)

// TestVMLifecycleToolSuccess exercises every QEMU VM lifecycle mutation tool
// end-to-end with the mutation gate ON, asserting a non-error result.
func TestVMLifecycleToolSuccess(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServer(t, app, true)

	cases := []struct {
		name string
		args map[string]any
	}{
		{"pve_vm_start", map[string]any{"node": "pve1", "vmid": 100}},
		{"pve_vm_stop", map[string]any{"node": "pve1", "vmid": 100, "skiplock": true}},
		{"pve_vm_shutdown", map[string]any{"node": "pve1", "vmid": 100, "force_stop": true, "timeout": 30}},
		{"pve_vm_reboot", map[string]any{"node": "pve1", "vmid": 100, "timeout": 60}},
		{"pve_vm_reset", map[string]any{"node": "pve1", "vmid": 100}},
		{"pve_vm_suspend", map[string]any{"node": "pve1", "vmid": 100, "to_disk": true}},
		{"pve_vm_resume", map[string]any{"node": "pve1", "vmid": 100}},
		{"pve_vm_delete", map[string]any{"node": "pve1", "vmid": 100, "purge": true, "destroy_unreferenced_disks": true}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := callTool(t, sess, tc.name, tc.args)
			assert.False(t, res.IsError, "tool %s should succeed", tc.name)
			assert.NotNil(t, res.StructuredContent, "tool %s should return structured output", tc.name)
		})
	}
}

// TestVMLifecycleToolErrorPropagation verifies IsError on gateway failure for
// every VM lifecycle mutation tool (gate ON).
func TestVMLifecycleToolErrorPropagation(t *testing.T) {
	gw := &stubPVEGateway{err: port.ErrNotFound}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(gw))
	_, sess := newTestServer(t, app, true)

	cases := []struct {
		name string
		args map[string]any
	}{
		{"pve_vm_start", map[string]any{"node": "pve1", "vmid": 100}},
		{"pve_vm_stop", map[string]any{"node": "pve1", "vmid": 100}},
		{"pve_vm_shutdown", map[string]any{"node": "pve1", "vmid": 100}},
		{"pve_vm_reboot", map[string]any{"node": "pve1", "vmid": 100}},
		{"pve_vm_reset", map[string]any{"node": "pve1", "vmid": 100}},
		{"pve_vm_suspend", map[string]any{"node": "pve1", "vmid": 100}},
		{"pve_vm_resume", map[string]any{"node": "pve1", "vmid": 100}},
		{"pve_vm_delete", map[string]any{"node": "pve1", "vmid": 100}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := callTool(t, sess, tc.name, tc.args)
			assert.True(t, res.IsError, "tool %s should be flagged IsError on gateway failure", tc.name)
		})
	}
}

// TestVMLifecycleToolGateOff verifies VM lifecycle mutation tools are absent
// when the mutation gate is OFF.
func TestVMLifecycleToolGateOff(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServer(t, app, false)
	names := toolNames(t, sess)

	for _, n := range []string{"pve_vm_start", "pve_vm_stop", "pve_vm_shutdown", "pve_vm_reboot",
		"pve_vm_reset", "pve_vm_suspend", "pve_vm_resume", "pve_vm_delete"} {
		assert.False(t, names[n], "mutation tool %s must be gated off", n)
	}
}
