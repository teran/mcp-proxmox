package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/application"
)

// TestMutationToolDestructiveHints verifies the S12 conformance fix: only the
// truly destructive delete operations carry DestructiveHint=true on their MCP
// ToolAnnotations, while every other mutation (including all PBS mutations —
// there are no PBS delete tools yet) is flagged non-destructive.
func TestMutationToolDestructiveHints(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0",
		application.WithPVE(&stubPVEGateway{}), application.WithPBS(&stubPBSGateway{}))
	_, sess := newTestServer(t, app, true)
	tools := toolsByName(t, sess)

	destructive := []string{
		"pve_vm_delete",
		"pve_vm_snapshot_delete",
		"pve_lxc_delete",
		"pve_lxc_snapshot_delete",
	}

	for _, name := range destructive {
		tl, ok := tools[name]
		require.True(t, ok, "expected tool %s in registry", name)
		require.NotNil(t, tl.Annotations, "tool %s must carry annotations", name)
		require.NotNil(t, tl.Annotations.DestructiveHint, "tool %s must set DestructiveHint", name)
		assert.True(t, *tl.Annotations.DestructiveHint, "delete tool %s must be flagged destructive", name)
	}

	nonDestructive := []string{
		"pve_vm_create",
		"pve_vm_start",
		"pve_vm_stop",
		"pve_vm_backup",
		"pbs_verify_start",
		"pbs_gc_start",
		"pbs_backup_restore_file",
		"pbs_backup_vm_restore",
	}

	for _, name := range nonDestructive {
		tl, ok := tools[name]
		require.True(t, ok, "expected tool %s in registry", name)
		require.NotNil(t, tl.Annotations, "tool %s must carry annotations", name)
		require.NotNil(t, tl.Annotations.DestructiveHint, "tool %s must set DestructiveHint", name)
		assert.False(t, *tl.Annotations.DestructiveHint, "non-delete tool %s must NOT be flagged destructive", name)
	}
}
