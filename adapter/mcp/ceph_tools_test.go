package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/port"
)

// cephReadToolArgs is the input args for each read-only Ceph tool.
var cephReadToolArgs = map[string]map[string]any{
	"ceph_cluster_status":   {},
	"ceph_cluster_config":   {},
	"ceph_cluster_log":      {"limit": 50},
	"ceph_cluster_metadata": {},
	"ceph_node_status":      {"node": "pve1"},
	"ceph_node_config":      {"node": "pve1"},
	"ceph_node_configdb":    {"node": "pve1"},
	"ceph_node_flags":       {"node": "pve1"},
	"ceph_node_metadata":    {"node": "pve1"},
	"ceph_node_crush":       {"node": "pve1"},
	"ceph_node_rules":       {"node": "pve1"},
	"ceph_pool_list":        {"node": "pve1"},
	"ceph_osd_list":         {"node": "pve1"},
	"ceph_mon_list":         {"node": "pve1"},
	"ceph_mgr_list":         {"node": "pve1"},
}

// TestCephReadOnlyToolSuccess exercises every read-only Ceph tool and asserts a
// non-error result. Read-only tools are registered with the mutation gate OFF.
func TestCephReadOnlyToolSuccess(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithCeph(&stubCephGateway{}))
	_, sess := newTestServer(t, app, false)

	for _, name := range cephROTools {
		t.Run(name, func(t *testing.T) {
			res := callTool(t, sess, name, cephReadToolArgs[name])
			assert.False(t, res.IsError, "tool %s should succeed", name)
			assert.NotNil(t, res.StructuredContent, "tool %s should return structured output", name)
		})
	}
}

// TestCephReadOnlyToolErrorPropagation verifies IsError on gateway failure for
// every read-only Ceph tool.
func TestCephReadOnlyToolErrorPropagation(t *testing.T) {
	gateway := &stubCephGateway{err: port.ErrNotFound}
	app := application.New(nopLogger{}, "v1.0.0", application.WithCeph(gateway))
	_, sess := newTestServer(t, app, false)

	for _, name := range cephROTools {
		t.Run(name, func(t *testing.T) {
			res := callTool(t, sess, name, cephReadToolArgs[name])
			assert.True(t, res.IsError, "tool %s should be flagged IsError on gateway failure", name)
		})
	}
}

// cephMutationToolArgs is the input args for each mutation Ceph tool.
var cephMutationToolArgs = map[string]map[string]any{
	"ceph_rule_create": {"node": "pve1", "name": "rule1", "type": "replicated", "min_size": 1, "max_size": 10},
	"ceph_rule_delete": {"node": "pve1", "name": "rule1"},
	"ceph_pool_create": {"node": "pve1", "name": "pool1", "size": 3, "pg_num": 128},
	"ceph_pool_update": {"node": "pve1", "name": "pool1", "size": 3, "pg_num": 256},
	"ceph_pool_delete": {"node": "pve1", "name": "pool1"},
	"ceph_osd_create":  {"node": "pve1", "osd": "0"},
	"ceph_osd_delete":  {"node": "pve1", "osdid": 0},
	"ceph_mon_create":  {"node": "pve1", "mon_addr": "10.0.0.2:6789"},
	"ceph_mon_delete":  {"node": "pve1", "monid": "pve2"},
	"ceph_mgr_create":  {"node": "pve1", "id": "pve2"},
	"ceph_flags_set":   {"node": "pve1", "values": map[string]any{"noout": "true"}},
	"ceph_config_set":  {"node": "pve1", "name": "mon_allow_pool_delete", "value": "true"},
	"ceph_init":        {"node": "pve1", "network": "10.0.0.0/24", "cluster_network": "10.0.1.0/24"},
	"ceph_destroy":     {"node": "pve1", "force": true},
}

// TestCephMutationToolSuccess exercises every mutation Ceph tool end-to-end
// with the gate ON, asserting a non-error result.
func TestCephMutationToolSuccess(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithCeph(&stubCephGateway{}))
	_, sess := newTestServer(t, app, true)

	for _, name := range cephMutationTools {
		t.Run(name, func(t *testing.T) {
			res := callTool(t, sess, name, cephMutationToolArgs[name])
			assert.False(t, res.IsError, "tool %s should succeed", name)
		})
	}
}

// TestCephMutationToolErrorPropagation verifies IsError on gateway failure for
// every mutation Ceph tool (gate ON).
func TestCephMutationToolErrorPropagation(t *testing.T) {
	gateway := &stubCephGateway{err: port.ErrNotFound}
	app := application.New(nopLogger{}, "v1.0.0", application.WithCeph(gateway))
	_, sess := newTestServer(t, app, true)

	for _, name := range cephMutationTools {
		t.Run(name, func(t *testing.T) {
			res := callTool(t, sess, name, cephMutationToolArgs[name])
			assert.True(t, res.IsError, "tool %s should be flagged IsError on gateway failure", name)
		})
	}
}

// TestCephMutationToolDestructiveHints verifies the S12 annotations on the Ceph
// mutation tools: destructive for *_delete and ceph_destroy; idempotent only
// for the genuinely safe-to-repeat operations (pool_update, flags_set,
// config_set).
func TestCephMutationToolDestructiveHints(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithCeph(&stubCephGateway{}))
	_, sess := newTestServer(t, app, true)
	tools := toolsByName(t, sess)

	destructive := []string{
		"ceph_rule_delete", "ceph_pool_delete",
		"ceph_osd_delete", "ceph_mon_delete", "ceph_destroy",
	}
	idempotent := []string{"ceph_pool_update", "ceph_flags_set", "ceph_config_set"}
	nonIdempotent := []string{
		"ceph_rule_create", "ceph_pool_create",
		"ceph_osd_create", "ceph_mon_create", "ceph_mgr_create", "ceph_init",
	}

	for _, name := range destructive {
		tl, ok := tools[name]
		require.True(t, ok, "expected tool %s in registry", name)
		require.NotNil(t, tl.Annotations, "tool %s must carry annotations", name)
		require.NotNil(t, tl.Annotations.DestructiveHint, "tool %s must set DestructiveHint", name)
		assert.True(t, *tl.Annotations.DestructiveHint, "destructive tool %s must be flagged destructive", name)
	}

	for _, name := range idempotent {
		tl, ok := tools[name]
		require.True(t, ok, "expected tool %s in registry", name)
		require.NotNil(t, tl.Annotations, "tool %s must carry annotations", name)
		assert.False(t, tl.Annotations.ReadOnlyHint, "mutation tool %s must not be read-only", name)
		assert.True(t, tl.Annotations.IdempotentHint, "tool %s should be idempotent", name)
	}
	for _, name := range nonIdempotent {
		tl, ok := tools[name]
		require.True(t, ok, "expected tool %s in registry", name)
		require.NotNil(t, tl.Annotations, "tool %s must carry annotations", name)
		assert.False(t, tl.Annotations.IdempotentHint, "tool %s should NOT be idempotent", name)
	}

	// Every mutation tool must carry per-tool Instructions in its Description.
	for _, name := range cephMutationTools {
		assert.Contains(t, tools[name].Description, "Instructions:",
			"tool %s must carry per-tool instructions", name)
	}
}

// TestCephToolsAbsentWhenCephDisabled verifies that no ceph_* tool is registered
// when the Ceph backend is not wired (mirrors the optional-injection pattern).
func TestCephToolsAbsentWhenCephDisabled(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0") // no WithCeph
	_, sess := newTestServer(t, app, true)
	names := toolNames(t, sess)

	for _, name := range append(append([]string{}, cephROTools...), cephMutationTools...) {
		assert.False(t, names[name], "ceph tool %s must be absent when Ceph is disabled", name)
	}
}
