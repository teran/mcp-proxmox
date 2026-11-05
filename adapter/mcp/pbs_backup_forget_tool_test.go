package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/port"
)

// TestBackupForgetToolForwardsArgs verifies the pbs_backup_forget mutation tool
// passes store/type/id/time through to PBSGateway.ForgetBackup and returns
// success (no output value).
func TestBackupForgetToolForwardsArgs(t *testing.T) {
	var gotStore, gotType, gotID string
	var gotTime int64
	gateway := &stubPBSGateway{forgetBackup: func(
		_ context.Context, store, backupType, backupID string, backupTime int64,
	) error {
		gotStore, gotType, gotID, gotTime = store, backupType, backupID, backupTime

		return nil
	}}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPBS(gateway))
	_, sess := newTestServer(t, app, true)

	res := callTool(t, sess, "pbs_backup_forget", map[string]any{
		"store": "backup", "backup_type": "vm", "backup_id": "141", "backup_time": int64(1700000000),
	})
	assert.False(t, res.IsError, "tool should succeed")
	assert.Equal(t, "backup", gotStore)
	assert.Equal(t, "vm", gotType)
	assert.Equal(t, "141", gotID)
	assert.Equal(t, int64(1700000000), gotTime)
}

// TestBackupForgetToolErrorPropagation verifies IsError on gateway failure.
func TestBackupForgetToolErrorPropagation(t *testing.T) {
	gateway := &stubPBSGateway{err: port.ErrNotFound}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPBS(gateway))
	_, sess := newTestServer(t, app, true)

	res := callTool(t, sess, "pbs_backup_forget", map[string]any{
		"store": "backup", "backup_type": "vm", "backup_id": "141", "backup_time": int64(1),
	})
	require.NotNil(t, res)
	assert.True(t, res.IsError, "tool should be flagged IsError on gateway failure")
}
