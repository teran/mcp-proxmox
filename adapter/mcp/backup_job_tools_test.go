package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/port"
)

// backupJobMutationTools is the PVE backup-job mutation surface under test.
var backupJobMutationTools = []string{"pve_backup_job_create", "pve_backup_job_update", "pve_backup_job_delete"}

// TestBackupJobToolRegistration verifies the three backup-job mutation tools
// are registered only when enableMutations=true and are gated off otherwise.
func TestBackupJobToolRegistration(t *testing.T) {
	t.Run("enabled registers backup job tools", func(t *testing.T) {
		app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
		_, sess := newTestServer(t, app, true)
		names := toolNames(t, sess)
		for _, n := range backupJobMutationTools {
			assert.True(t, names[n], "expected backup job tool %s registered", n)
		}
	})

	t.Run("disabled does not register backup job tools", func(t *testing.T) {
		app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
		_, sess := newTestServer(t, app, false)
		names := toolNames(t, sess)
		for _, n := range backupJobMutationTools {
			assert.False(t, names[n], "backup job tool %s must be gated off", n)
		}
	})
}

// TestBackupJobToolsSuccess exercises the happy path of each backup-job tool
// with the gate ON, asserting the structured output values: {"id": ...} for
// create/update and {"deleted": true} for delete.
func TestBackupJobToolsSuccess(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServer(t, app, true)

	t.Run("pve_backup_job_create", func(t *testing.T) {
		res := callTool(t, sess, "pve_backup_job_create", map[string]any{
			"storage": "backup", "schedule": "00 5 * * *", "vmid": "100,101",
		})
		assert.False(t, res.IsError)
		out, ok := res.StructuredContent.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "backup-100", out["id"])
	})

	t.Run("pve_backup_job_update", func(t *testing.T) {
		res := callTool(t, sess, "pve_backup_job_update", map[string]any{
			"id": "backup-100", "storage": "local", "schedule": "00 6 * * *",
		})
		assert.False(t, res.IsError)
		out, ok := res.StructuredContent.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "backup-100", out["id"])
	})

	t.Run("pve_backup_job_delete", func(t *testing.T) {
		res := callTool(t, sess, "pve_backup_job_delete", map[string]any{"id": "backup-100"})
		assert.False(t, res.IsError)
		out, ok := res.StructuredContent.(map[string]any)
		require.True(t, ok)
		assert.Equal(t, true, out["deleted"])
	})
}

// TestBackupJobToolsErrorPropagation verifies IsError is set on the result when
// the underlying gateway call fails (gate ON).
func TestBackupJobToolsErrorPropagation(t *testing.T) {
	gw := &stubPVEGateway{err: port.ErrNotFound}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(gw))
	_, sess := newTestServer(t, app, true)

	cases := []struct {
		name string
		args map[string]any
	}{
		{"pve_backup_job_create", map[string]any{"storage": "backup", "schedule": "00 5 * * *"}},
		{"pve_backup_job_update", map[string]any{"id": "backup-100", "storage": "local", "schedule": "00 6 * * *"}},
		{"pve_backup_job_delete", map[string]any{"id": "backup-100"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := callTool(t, sess, tc.name, tc.args)
			assert.True(t, res.IsError, "tool %s should be flagged IsError on gateway failure", tc.name)
		})
	}
}

// TestBackupJobToolAnnotations verifies the MCP ToolAnnotations on the three
// backup-job tools: create/update are idempotent and non-destructive, while
// delete is destructive and NOT idempotent (S12 / X02).
func TestBackupJobToolAnnotations(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServer(t, app, true)
	tools := toolsByName(t, sess)

	idempotent := []string{"pve_backup_job_create", "pve_backup_job_update"}
	for _, name := range idempotent {
		tl, ok := tools[name]
		require.True(t, ok, "expected tool %s in registry", name)
		require.NotNil(t, tl.Annotations, "tool %s must carry annotations", name)
		a := tl.Annotations
		assert.False(t, a.ReadOnlyHint, "tool %s must not be read-only", name)
		assert.True(t, a.IdempotentHint, "tool %s should be idempotent", name)
		require.NotNil(t, a.OpenWorldHint)
		assert.False(t, *a.OpenWorldHint, "tool %s must be closed-world", name)
		require.NotNil(t, a.DestructiveHint)
		assert.False(t, *a.DestructiveHint, "tool %s must not be destructive", name)
	}

	tl, ok := tools["pve_backup_job_delete"]
	require.True(t, ok, "expected tool pve_backup_job_delete in registry")
	require.NotNil(t, tl.Annotations)
	a := tl.Annotations
	assert.False(t, a.ReadOnlyHint, "delete must not be read-only")
	assert.False(t, a.IdempotentHint, "delete must NOT be idempotent")
	require.NotNil(t, a.OpenWorldHint)
	assert.False(t, *a.OpenWorldHint, "delete must be closed-world")
	require.NotNil(t, a.DestructiveHint)
	assert.True(t, *a.DestructiveHint, "delete must be destructive")

	// Every backup-job mutation tool must carry per-tool Instructions.
	for _, name := range backupJobMutationTools {
		assert.Contains(t, tools[name].Description, "Instructions:",
			"tool %s must carry per-tool instructions", name)
	}
}
