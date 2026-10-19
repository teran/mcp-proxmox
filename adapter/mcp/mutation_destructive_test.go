package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/application"
)

// TestMutationToolDestructiveHints verifies the S12 conformance fix: truly
// destructive mutations — deletes, restores, rollbacks, and destructive PBS
// jobs (prune, GC) — carry DestructiveHint=true on their MCP ToolAnnotations,
// while non-destructive mutations (create/start/stop/set/sync/verify, etc.) are
// flagged non-destructive.
func TestMutationToolDestructiveHints(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0",
		application.WithPVE(&stubPVEGateway{}), application.WithPBS(&stubPBSGateway{}))
	_, sess := newTestServer(t, app, true)
	tools := toolsByName(t, sess)

	destructive := []string{
		"pve_vm_delete",
		"pve_vm_snapshot_delete",
		"pve_vm_snapshot_rollback",
		"pve_vm_restore",
		"pve_lxc_delete",
		"pve_lxc_snapshot_delete",
		"pve_lxc_snapshot_rollback",
		"pbs_gc_start",
		"pbs_prune_start",
		"pbs_backup_restore_file",
		"pbs_backup_vm_restore",
	}

	for _, name := range destructive {
		tl, ok := tools[name]
		require.True(t, ok, "expected tool %s in registry", name)
		require.NotNil(t, tl.Annotations, "tool %s must carry annotations", name)
		require.NotNil(t, tl.Annotations.DestructiveHint, "tool %s must set DestructiveHint", name)
		assert.True(t, *tl.Annotations.DestructiveHint, "destructive tool %s must be flagged destructive", name)
	}

	nonDestructive := []string{
		"pve_vm_create",
		"pve_vm_start",
		"pve_vm_stop",
		"pve_vm_backup",
		"pve_vm_set",
		"pve_lxc_set",
		"pbs_verify_start",
		"pbs_sync_start",
	}

	for _, name := range nonDestructive {
		tl, ok := tools[name]
		require.True(t, ok, "expected tool %s in registry", name)
		require.NotNil(t, tl.Annotations, "tool %s must carry annotations", name)
		require.NotNil(t, tl.Annotations.DestructiveHint, "tool %s must set DestructiveHint", name)
		assert.False(t, *tl.Annotations.DestructiveHint, "non-destructive tool %s must NOT be flagged destructive", name)
	}
}
