package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/port"
)

// TestPBSReadOnlyToolSuccess exercises the new PBS read-only tools.
func TestPBSReadOnlyToolSuccess(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPBS(&stubPBSGateway{}))
	_, sess := newTestServer(t, app, false)

	cases := []struct {
		name string
		args map[string]any
	}{
		{"pbs_backup_files_list", map[string]any{
			"store": "backup", "backup_type": "vm", "backup_id": "100", "snapshot": "snap1", "path": "/etc",
		}},
		{"pbs_task_status", map[string]any{"upid": "UPID:..."}},
		{"pbs_task_log", map[string]any{"upid": "UPID:...", "limit": 50}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := callTool(t, sess, tc.name, tc.args)
			assert.False(t, res.IsError, "tool %s should succeed", tc.name)
			assert.NotNil(t, res.StructuredContent, "tool %s should return structured output", tc.name)
		})
	}
}

// TestPBSReadOnlyToolErrorPropagation verifies IsError on gateway failure.
func TestPBSReadOnlyToolErrorPropagation(t *testing.T) {
	gw := &stubPBSGateway{err: port.ErrNotFound}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPBS(gw))
	_, sess := newTestServer(t, app, false)

	cases := []struct {
		name string
		args map[string]any
	}{
		{"pbs_backup_files_list", map[string]any{
			"store": "backup", "backup_type": "vm", "backup_id": "100", "snapshot": "snap1",
		}},
		{"pbs_task_status", map[string]any{"upid": "UPID:..."}},
		{"pbs_task_log", map[string]any{"upid": "UPID:..."}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := callTool(t, sess, tc.name, tc.args)
			assert.True(t, res.IsError, "tool %s should be flagged IsError on gateway failure", tc.name)
		})
	}
}
