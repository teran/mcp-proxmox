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

// callTool is a thin helper that calls a tool by name with arguments.
func callTool(t *testing.T, sess *mcpSDK.ClientSession, name string, args map[string]any) *mcpSDK.CallToolResult {
	t.Helper()
	res, err := sess.CallTool(context.Background(), &mcpSDK.CallToolParams{Name: name, Arguments: args})
	require.NoError(t, err, "call %s", name)
	return res
}

func TestSystemToolsAlwaysRegistered(t *testing.T) {
	// No backends at all.
	app := application.New(nopLogger{}, "v1.0.0")
	_, sess := newTestServer(t, app, false)

	names := toolNames(t, sess)
	assert.True(t, names["ping"], "ping must always be registered")
	assert.True(t, names["status"], "status must always be registered")
	assert.False(t, names["pve_node_list"], "pve tool must be absent when PVE disabled")
	assert.False(t, names["pbs_datastore_list"], "pbs tool must be absent when PBS disabled")
}

func TestBackendToolRegistration(t *testing.T) {
	t.Run("pve only", func(t *testing.T) {
		app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
		_, sess := newTestServer(t, app, false)
		names := toolNames(t, sess)

		for _, n := range pveROTools {
			assert.True(t, names[n], "expected %s registered", n)
		}
		for _, n := range pbsROTools {
			assert.False(t, names[n], "did not expect %s registered", n)
		}
	})

	t.Run("pbs only", func(t *testing.T) {
		app := application.New(nopLogger{}, "v1.0.0", application.WithPBS(&stubPBSGateway{}))
		_, sess := newTestServer(t, app, false)
		names := toolNames(t, sess)

		for _, n := range pbsROTools {
			assert.True(t, names[n], "expected %s registered", n)
		}
		for _, n := range pveROTools {
			assert.False(t, names[n], "did not expect %s registered", n)
		}
	})

	t.Run("both", func(t *testing.T) {
		app := application.New(nopLogger{}, "v1.0.0",
			application.WithPVE(&stubPVEGateway{}), application.WithPBS(&stubPBSGateway{}))
		_, sess := newTestServer(t, app, false)
		names := toolNames(t, sess)

		for _, n := range append(pveROTools, pbsROTools...) {
			assert.True(t, names[n], "expected %s registered", n)
		}
	})
}

func TestMutationGate(t *testing.T) {
	// With the gate OFF (milestone default), mutation tools must not be
	// registered even for enabled backends. The scaffold has no mutation tools
	// yet; this pins the invariant that nothing mutation-like leaks through.
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServer(t, app, false)
	names := toolNames(t, sess)

	for _, n := range []string{"pve_vm_migrate", "pve_vm_start", "pbs_backup_restore", "pbs_verify_start"} {
		assert.False(t, names[n], "mutation tool %s must be gated off", n)
	}
}

// TestPVEToolSuccess exercises every PVE read-only tool end-to-end through the
// real MCP stack and asserts a non-error result.
func TestPVEToolSuccess(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServer(t, app, false)

	cases := []struct {
		name string
		args map[string]any
	}{
		{"pve_node_list", nil},
		{"pve_node_status", map[string]any{"node": "pve1"}},
		{"pve_cluster_status", nil},
		{"pve_cluster_resources", nil},
		{"pve_nextid", nil},
		{"pve_vm_list", map[string]any{"node": "pve1"}},
		{"pve_vm_get", map[string]any{"node": "pve1", "vmid": 100}},
		{"pve_vm_status", map[string]any{"node": "pve1", "vmid": 100}},
		{"pve_lxc_list", map[string]any{"node": "pve1"}},
		{"pve_lxc_get", map[string]any{"node": "pve1", "vmid": 200}},
		{"pve_lxc_status", map[string]any{"node": "pve1", "vmid": 200}},
		{"pve_storage_list", nil},
		{"pve_storage_get", map[string]any{"node": "pve1", "storage": "local"}},
		{"pve_network_list", map[string]any{"node": "pve1"}},
		{"pve_task_list", map[string]any{"node": "pve1"}},
		{"pve_task_status", map[string]any{"node": "pve1", "upid": "UPID:..."}},
		{"pve_task_log", map[string]any{"node": "pve1", "upid": "UPID:...", "limit": 50}},
		{"pve_version", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := callTool(t, sess, tc.name, tc.args)
			assert.False(t, res.IsError, "tool %s should succeed", tc.name)
			assert.NotNil(t, res.StructuredContent, "tool %s should return structured output", tc.name)
		})
	}
}

// TestPBSToolSuccess exercises every PBS read-only tool end-to-end.
func TestPBSToolSuccess(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPBS(&stubPBSGateway{}))
	_, sess := newTestServer(t, app, false)

	cases := []struct {
		name string
		args map[string]any
	}{
		{"pbs_datastore_list", nil},
		{"pbs_datastore_status", map[string]any{"store": "backup"}},
		{"pbs_backup_list", map[string]any{"store": "backup"}},
		{"pbs_backup_get", map[string]any{"store": "backup", "backup_id": "vm/100/..."}},
		{"pbs_backup_notes_get", map[string]any{"store": "backup", "backup_id": "vm/100/...", "backup_type": "vm"}},
		{"pbs_verify_status", map[string]any{"store": "backup", "upid": "UPID:..."}},
		{"pbs_prune_status", map[string]any{"store": "backup", "upid": "UPID:..."}},
		{"pbs_version", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := callTool(t, sess, tc.name, tc.args)
			assert.False(t, res.IsError, "tool %s should succeed", tc.name)
			assert.NotNil(t, res.StructuredContent, "tool %s should return structured output", tc.name)
		})
	}
}

// TestPVEToolErrorPropagation verifies that when the gateway returns an error,
// the MCP tool result is flagged IsError (SPEC.md §8.1 / helpers.go).
func TestPVEToolErrorPropagation(t *testing.T) {
	gw := &stubPVEGateway{err: port.ErrNotFound}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(gw))
	_, sess := newTestServer(t, app, false)

	cases := []struct {
		name string
		args map[string]any
	}{
		{"pve_node_list", nil},
		{"pve_node_status", map[string]any{"node": "pve1"}},
		{"pve_cluster_status", nil},
		{"pve_cluster_resources", nil},
		{"pve_nextid", nil},
		{"pve_vm_list", map[string]any{"node": "pve1"}},
		{"pve_vm_get", map[string]any{"node": "pve1", "vmid": 100}},
		{"pve_vm_status", map[string]any{"node": "pve1", "vmid": 100}},
		{"pve_lxc_list", map[string]any{"node": "pve1"}},
		{"pve_lxc_get", map[string]any{"node": "pve1", "vmid": 200}},
		{"pve_lxc_status", map[string]any{"node": "pve1", "vmid": 200}},
		{"pve_storage_list", nil},
		{"pve_storage_get", map[string]any{"node": "pve1", "storage": "local"}},
		{"pve_network_list", map[string]any{"node": "pve1"}},
		{"pve_task_list", map[string]any{"node": "pve1"}},
		{"pve_task_status", map[string]any{"node": "pve1", "upid": "UPID:..."}},
		{"pve_task_log", map[string]any{"node": "pve1", "upid": "UPID:...", "limit": 50}},
		{"pve_version", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := callTool(t, sess, tc.name, tc.args)
			assert.True(t, res.IsError, "tool %s should be flagged IsError on gateway failure", tc.name)
		})
	}
}

// TestPBSToolErrorPropagation verifies IsError on PBS gateway failure.
func TestPBSToolErrorPropagation(t *testing.T) {
	gw := &stubPBSGateway{err: port.ErrNotFound}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPBS(gw))
	_, sess := newTestServer(t, app, false)

	cases := []struct {
		name string
		args map[string]any
	}{
		{"pbs_datastore_list", nil},
		{"pbs_datastore_status", map[string]any{"store": "backup"}},
		{"pbs_backup_list", map[string]any{"store": "backup"}},
		{"pbs_backup_get", map[string]any{"store": "backup", "backup_id": "vm/100/..."}},
		{"pbs_backup_notes_get", map[string]any{"store": "backup", "backup_id": "vm/100/...", "backup_type": "vm"}},
		{"pbs_verify_status", map[string]any{"store": "backup", "upid": "UPID:..."}},
		{"pbs_prune_status", map[string]any{"store": "backup", "upid": "UPID:..."}},
		{"pbs_version", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := callTool(t, sess, tc.name, tc.args)
			assert.True(t, res.IsError, "tool %s should be flagged IsError on gateway failure", tc.name)
		})
	}
}

// TestPingAndStatusContent verifies ping/status return the expected payloads.
func TestPingAndStatusContent(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0",
		application.WithPVE(&stubPVEGateway{}), application.WithPBS(&stubPBSGateway{}))
	_, sess := newTestServer(t, app, false)

	ping := callTool(t, sess, "ping", nil)
	assert.False(t, ping.IsError)
	pm, ok := ping.StructuredContent.(map[string]any)
	require.True(t, ok, "ping output should be a map")
	assert.Equal(t, "ok", pm["status"])
	assert.Equal(t, map[string]any{"pve": true, "pbs": true}, pm["backends"])

	status := callTool(t, sess, "status", nil)
	assert.False(t, status.IsError)
	st, ok := status.StructuredContent.(map[string]any)
	require.True(t, ok, "status output should be a map")
	assert.Equal(t, "v1.0.0", st["version"])
	assert.Equal(t, "stdio", st["transport"])
}

// TestTaskListOptionsForwarded documents that pve_task_list forwards its filter
// fields; the full propagation is covered at the service layer
// (application/pve_service_test.go).
func TestTaskListOptionsForwarded(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServer(t, app, false)
	res := callTool(t, sess, "pve_task_list", map[string]any{"node": "pve1", "vmid": 100, "limit": 5})
	assert.False(t, res.IsError)
}
