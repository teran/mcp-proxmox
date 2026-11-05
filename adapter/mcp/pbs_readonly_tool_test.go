package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/model"
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
			"store": "backup", "backup_type": "vm", "backup_id": "100", "backup_time": 1793662080, "path": "/etc",
		}},
		{"pbs_task_status", map[string]any{"upid": "UPID:..."}},
		{"pbs_task_log", map[string]any{"upid": "UPID:...", "limit": 50}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			res := callTool(t, sess, testCase.name, testCase.args)
			assert.False(t, res.IsError, "tool %s should succeed", testCase.name)
			assert.NotNil(t, res.StructuredContent, "tool %s should return structured output", testCase.name)
		})
	}
}

// TestPBSReadOnlyToolErrorPropagation verifies IsError on gateway failure.
func TestPBSReadOnlyToolErrorPropagation(t *testing.T) {
	gateway := &stubPBSGateway{err: port.ErrNotFound}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPBS(gateway))
	_, sess := newTestServer(t, app, false)

	cases := []struct {
		name string
		args map[string]any
	}{
		{"pbs_backup_files_list", map[string]any{
			"store": "backup", "backup_type": "vm", "backup_id": "100", "backup_time": 1793662080,
		}},
		{"pbs_task_status", map[string]any{"upid": "UPID:..."}},
		{"pbs_task_log", map[string]any{"upid": "UPID:..."}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			res := callTool(t, sess, testCase.name, testCase.args)
			assert.True(t, res.IsError, "tool %s should be flagged IsError on gateway failure", testCase.name)
		})
	}
}

// TestPBSBackupListToolForwardsOptions verifies pbs_backup_list decodes its
// limit/start/end inputs into a port.PBSBackupListOptions passed to the gateway.
func TestPBSBackupListToolForwardsOptions(t *testing.T) {
	var gotOpts port.PBSBackupListOptions
	gateway := &stubPBSGateway{listBackups: func(
		_ context.Context, _ string, opts port.PBSBackupListOptions,
	) ([]model.Backup, error) {
		gotOpts = opts

		return []model.Backup{{BackupID: "vm/100/..."}}, nil
	}}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPBS(gateway))
	_, sess := newTestServer(t, app, false)

	res := callTool(t, sess, "pbs_backup_list", map[string]any{
		"store": "backup", "limit": 5, "start": 1700000000, "end": 1700000100,
	})
	assert.False(t, res.IsError, "pbs_backup_list should succeed")
	assert.Equal(t, port.PBSBackupListOptions{Limit: 5, Start: 1700000000, End: 1700000100}, gotOpts)
}

// TestPBSBackupGetToolForwardsOptions verifies pbs_backup_get decodes its
// limit/start/end inputs into a port.PBSBackupListOptions passed to the gateway.
func TestPBSBackupGetToolForwardsOptions(t *testing.T) {
	var gotOpts port.PBSBackupListOptions
	gateway := &stubPBSGateway{getBackup: func(
		_ context.Context, _, _ string, opts port.PBSBackupListOptions,
	) ([]model.Backup, error) {
		gotOpts = opts

		return []model.Backup{{BackupID: "vm/100/..."}}, nil
	}}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPBS(gateway))
	_, sess := newTestServer(t, app, false)

	res := callTool(t, sess, "pbs_backup_get", map[string]any{
		"store": "backup", "backup_id": "100", "limit": 3, "start": 1, "end": 2,
	})
	assert.False(t, res.IsError, "pbs_backup_get should succeed")
	assert.Equal(t, port.PBSBackupListOptions{Limit: 3, Start: 1, End: 2}, gotOpts)
}
