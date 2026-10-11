package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/port"
)

// TestVMSetToolSuccess exercises pve_vm_set and pve_lxc_set end-to-end with the
// mutation gate ON, asserting a non-error result for a config map input.
func TestVMSetToolSuccess(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServer(t, app, true)

	cases := []struct {
		name string
		args map[string]any
	}{
		{"pve_vm_set", map[string]any{"node": "pve1", "vmid": 100, "config": map[string]any{"onboot": "0"}}},
		{"pve_lxc_set", map[string]any{"node": "pve1", "vmid": 200, "config": map[string]any{"onboot": "0"}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := callTool(t, sess, tc.name, tc.args)
			assert.False(t, res.IsError, "tool %s should succeed", tc.name)
		})
	}
}

// TestVMSetToolErrorPropagation verifies IsError on gateway failure for both set
// tools (gate ON).
func TestVMSetToolErrorPropagation(t *testing.T) {
	gw := &stubPVEGateway{err: port.ErrNotFound}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(gw))
	_, sess := newTestServer(t, app, true)

	cases := []struct {
		name string
		args map[string]any
	}{
		{"pve_vm_set", map[string]any{"node": "pve1", "vmid": 100, "config": map[string]any{"onboot": "0"}}},
		{"pve_lxc_set", map[string]any{"node": "pve1", "vmid": 200, "config": map[string]any{"onboot": "0"}}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := callTool(t, sess, tc.name, tc.args)
			assert.True(t, res.IsError, "tool %s should be flagged IsError on gateway failure", tc.name)
		})
	}
}

// TestVMSetToolGateOff verifies pve_vm_set and pve_lxc_set are absent when the
// mutation gate is OFF.
func TestVMSetToolGateOff(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServer(t, app, false)
	names := toolNames(t, sess)

	for _, n := range []string{"pve_vm_set", "pve_lxc_set"} {
		assert.False(t, names[n], "mutation tool %s must be gated off", n)
	}
}
