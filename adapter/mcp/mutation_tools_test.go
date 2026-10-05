package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/port"
)

// toolsByName returns the full Tool descriptors (with annotations) keyed by
// name, from a ListTools call.
func toolsByName(t *testing.T, sess *mcpSDK.ClientSession) map[string]*mcpSDK.Tool {
	t.Helper()
	res, err := sess.ListTools(context.Background(), nil)
	require.NoError(t, err)
	out := make(map[string]*mcpSDK.Tool, len(res.Tools))
	for _, tl := range res.Tools {
		out[tl.Name] = tl
	}
	return out
}

// TestMutationToolRegistration verifies mutation tools are registered only when
// enableMutations=true and are absent otherwise.
func TestMutationToolRegistration(t *testing.T) {
	t.Run("enabled registers mutation tools", func(t *testing.T) {
		app := application.New(nopLogger{}, "v1.0.0",
			application.WithPVE(&stubPVEGateway{}), application.WithPBS(&stubPBSGateway{}))
		_, sess := newTestServer(t, app, true)
		names := toolNames(t, sess)

		for _, n := range append(append([]string{}, pveMutationTools...), pbsMutationTools...) {
			assert.True(t, names[n], "expected mutation tool %s registered", n)
		}
	})

	t.Run("disabled does not register mutation tools", func(t *testing.T) {
		app := application.New(nopLogger{}, "v1.0.0",
			application.WithPVE(&stubPVEGateway{}), application.WithPBS(&stubPBSGateway{}))
		_, sess := newTestServer(t, app, false)
		names := toolNames(t, sess)

		for _, n := range append(append([]string{}, pveMutationTools...), pbsMutationTools...) {
			assert.False(t, names[n], "mutation tool %s must be gated off", n)
		}
	})
}

// TestMutationToolSuccess exercises every mutation tool end-to-end with the
// gate ON, asserting a non-error result.
func TestMutationToolSuccess(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0",
		application.WithPVE(&stubPVEGateway{}), application.WithPBS(&stubPBSGateway{}))
	_, sess := newTestServer(t, app, true)

	cases := []struct {
		name       string
		args       map[string]any
		wantOutput bool
	}{
		{"pve_vm_create", map[string]any{"node": "pve1", "name": "web", "cores": 2, "memory": 1024}, true},
		{"pve_vm_resize", map[string]any{"node": "pve1", "vmid": 100, "disk": "scsi0", "size_gb": 20}, false},
		{"pve_vm_migrate", map[string]any{"node": "pve1", "vmid": 100, "target": "pve2", "online": true}, false},
		{"pve_ha_add", map[string]any{"sid": "vm:100", "type": "vm", "nodes": []string{"pve1"}}, false},
		{"pve_vm_backup", map[string]any{"node": "pve1", "vmid": 100, "storage": "backup", "mode": "snapshot"}, true},
		{"pbs_verify_start", map[string]any{"store": "backup"}, true},
		{"pbs_gc_start", map[string]any{"store": "backup"}, true},
		{"pbs_prune_start", map[string]any{"store": "backup"}, true},
		{"pbs_sync_start", map[string]any{"store": "backup", "remote": "r", "remote_store": "s"}, true},
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

// TestMutationToolSuccessOutputValues asserts the specific structured output
// produced by the output-returning mutation tools.
func TestMutationToolSuccessOutputValues(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0",
		application.WithPVE(&stubPVEGateway{}), application.WithPBS(&stubPBSGateway{}))
	_, sess := newTestServer(t, app, true)

	create := callTool(t, sess, "pve_vm_create", map[string]any{"node": "pve1", "name": "web"})
	cm, ok := create.StructuredContent.(map[string]any)
	require.True(t, ok)
	// The int VMID arrives as a float64 through the JSON transport.
	assert.Equal(t, float64(100), cm["vmid"])

	backup := callTool(t, sess, "pve_vm_backup",
		map[string]any{"node": "pve1", "vmid": 100, "storage": "backup", "mode": "snapshot"})
	bm, ok := backup.StructuredContent.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "UPID:pve:backup:...", bm["upid"])

	verify := callTool(t, sess, "pbs_verify_start", map[string]any{"store": "backup"})
	vm, ok := verify.StructuredContent.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "UPID:pbs:verify:...", vm["upid"])
}

// TestMutationToolErrorPropagation verifies IsError on gateway failure for every
// mutation tool (gate ON).
func TestMutationToolErrorPropagation(t *testing.T) {
	gw := &stubPVEGateway{err: port.ErrNotFound}
	gb := &stubPBSGateway{err: port.ErrNotFound}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(gw), application.WithPBS(gb))
	_, sess := newTestServer(t, app, true)

	cases := []struct {
		name string
		args map[string]any
	}{
		{"pve_vm_create", map[string]any{"node": "pve1", "name": "web"}},
		{"pve_vm_resize", map[string]any{"node": "pve1", "vmid": 100, "disk": "scsi0", "size_gb": 20}},
		{"pve_vm_migrate", map[string]any{"node": "pve1", "vmid": 100, "target": "pve2"}},
		{"pve_ha_add", map[string]any{"sid": "vm:100", "type": "vm"}},
		{"pve_vm_backup", map[string]any{"node": "pve1", "vmid": 100, "storage": "backup", "mode": "snapshot"}},
		{"pbs_verify_start", map[string]any{"store": "backup"}},
		{"pbs_gc_start", map[string]any{"store": "backup"}},
		{"pbs_prune_start", map[string]any{"store": "backup"}},
		{"pbs_sync_start", map[string]any{"store": "backup", "remote": "r"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := callTool(t, sess, tc.name, tc.args)
			assert.True(t, res.IsError, "tool %s should be flagged IsError on gateway failure", tc.name)
		})
	}
}

// TestMutationToolAnnotations verifies the MCP ToolAnnotations on the mutation
// tools: ReadOnlyHint=false, OpenWorldHint=false, DestructiveHint=false, and
// IdempotentHint true only for genuinely safe-to-repeat operations
// (resize/migrate/ha-add) and false for everything that starts a fresh
// side-effect — including pve_vm_create, which is NOT idempotent because a
// repeated create provisions a second VM (SPEC.md §2.5 / helpers.go mutTool,
// X02 conformance fix).
func TestMutationToolAnnotations(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0",
		application.WithPVE(&stubPVEGateway{}), application.WithPBS(&stubPBSGateway{}))
	_, sess := newTestServer(t, app, true)
	tools := toolsByName(t, sess)

	idempotent := []string{"pve_vm_resize", "pve_vm_migrate", "pve_ha_add"}
	nonIdempotent := []string{"pve_vm_create", "pve_vm_backup", "pbs_verify_start", "pbs_gc_start", "pbs_prune_start", "pbs_sync_start"}

	for _, name := range append(append([]string{}, idempotent...), nonIdempotent...) {
		tl, ok := tools[name]
		require.True(t, ok, "expected tool %s in registry", name)
		require.NotNil(t, tl.Annotations, "tool %s must carry annotations", name)

		a := tl.Annotations
		assert.False(t, a.ReadOnlyHint, "tool %s must not be read-only", name)
		require.NotNil(t, a.OpenWorldHint)
		assert.False(t, *a.OpenWorldHint, "tool %s must be closed-world", name)
		require.NotNil(t, a.DestructiveHint)
		assert.False(t, *a.DestructiveHint, "tool %s must not be destructive", name)
	}

	for _, name := range idempotent {
		assert.True(t, tools[name].Annotations.IdempotentHint, "tool %s should be idempotent", name)
	}
	for _, name := range nonIdempotent {
		assert.False(t, tools[name].Annotations.IdempotentHint, "tool %s should NOT be idempotent", name)
	}

	// Every mutation tool must carry per-tool Instructions in its Description.
	for _, name := range append(append([]string{}, idempotent...), nonIdempotent...) {
		assert.Contains(t, tools[name].Description, "Instructions:",
			"tool %s must carry per-tool instructions", name)
	}
}
