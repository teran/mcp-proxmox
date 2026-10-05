package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/port"
)

// TestPVMReRestoreToolSuccess exercises the pve_vm_restore tool with the gate ON.
func TestPVMReRestoreToolSuccess(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServer(t, app, true)

	res := callTool(t, sess, "pve_vm_restore", map[string]any{
		"node": "pve1", "archive": "/var/lib/vz/dump/vzdump-qemu-100.vma.zst",
		"vmid": 100, "storage": "local", "unique": true, "force": true, "bwlimit": 4096,
	})
	assert.False(t, res.IsError, "tool pve_vm_restore should succeed")
	assert.NotNil(t, res.StructuredContent, "tool pve_vm_restore should return structured output")
}

// TestPVMReRestoreToolErrorPropagation verifies IsError on gateway failure.
func TestPVMReRestoreToolErrorPropagation(t *testing.T) {
	gw := &stubPVEGateway{err: port.ErrNotFound}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(gw))
	_, sess := newTestServer(t, app, true)

	res := callTool(t, sess, "pve_vm_restore", map[string]any{"node": "pve1", "archive": "/a.bak"})
	assert.True(t, res.IsError, "tool pve_vm_restore should be flagged IsError on gateway failure")
}

// TestPVMReRestoreToolGateOff verifies pve_vm_restore is absent when the gate
// is OFF.
func TestPVMReRestoreToolGateOff(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServer(t, app, false)
	names := toolNames(t, sess)
	assert.False(t, names["pve_vm_restore"], "mutation tool pve_vm_restore must be gated off")
}
