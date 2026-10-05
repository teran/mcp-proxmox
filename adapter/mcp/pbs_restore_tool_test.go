package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/port"
)

// TestPBSRestoreToolSuccess exercises the PBS restore mutation tools with the
// gate ON.
func TestPBSRestoreToolSuccess(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPBS(&stubPBSGateway{}))
	_, sess := newTestServer(t, app, true)

	cases := []struct {
		name       string
		args       map[string]any
		wantOutput bool
	}{
		{"pbs_backup_restore_file", map[string]any{
			"store": "backup", "backup_type": "vm", "backup_id": "100",
			"snapshot": "snap1", "path": "/etc/passwd", "target": "/restore/passwd",
		}, false}, // RestoreFile returns no output (matches pve_vm_resize convention)
		{"pbs_backup_vm_restore", map[string]any{
			"store": "backup", "backup_type": "vm", "backup_id": "100",
			"snapshot": "snap1", "target": "local", "vmid": "next", "verbose": true,
		}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := callTool(t, sess, tc.name, tc.args)
			assert.False(t, res.IsError, "tool %s should succeed", tc.name)
			if tc.wantOutput {
				assert.NotNil(t, res.StructuredContent, "tool %s should return structured output", tc.name)
			}
		})
	}
}

// TestPBSRestoreToolErrorPropagation verifies IsError on gateway failure.
func TestPBSRestoreToolErrorPropagation(t *testing.T) {
	gw := &stubPBSGateway{err: port.ErrNotFound}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPBS(gw))
	_, sess := newTestServer(t, app, true)

	cases := []struct {
		name string
		args map[string]any
	}{
		{"pbs_backup_restore_file", map[string]any{
			"store": "backup", "backup_type": "vm", "backup_id": "100",
			"snapshot": "snap1", "path": "/a", "target": "/b",
		}},
		{"pbs_backup_vm_restore", map[string]any{
			"store": "backup", "backup_type": "vm", "backup_id": "100",
			"snapshot": "snap1", "target": "local",
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := callTool(t, sess, tc.name, tc.args)
			assert.True(t, res.IsError, "tool %s should be flagged IsError on gateway failure", tc.name)
		})
	}
}

// TestPBSRestoreToolGateOff verifies the PBS restore mutation tools are absent
// when the gate is OFF.
func TestPBSRestoreToolGateOff(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPBS(&stubPBSGateway{}))
	_, sess := newTestServer(t, app, false)
	names := toolNames(t, sess)

	for _, n := range []string{"pbs_backup_restore_file", "pbs_backup_vm_restore"} {
		assert.False(t, names[n], "mutation tool %s must be gated off", n)
	}
}
