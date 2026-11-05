package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/port"
)

// TestLXCCreateToolSuccess verifies pve_lxc_create succeeds and surfaces the
// numeric vmid in its structured output.
func TestLXCCreateToolSuccess(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServer(t, app, true)

	res := callTool(t, sess, "pve_lxc_create",
		map[string]any{
			"node": "pve1", "vmid": 203, "ostemplate": "local:vztmpl/debian-12.tar.zst",
			"hostname": "ct1", "storage": "local", "rootfs": "local:8",
			"cores": 2, "memory": 1024, "unprivileged": true,
		})
	assert.False(t, res.IsError, "pve_lxc_create should succeed")
	require.NotNil(t, res.StructuredContent)
	sc, ok := res.StructuredContent.(map[string]any)
	require.True(t, ok)
	// The int vmid arrives as a float64 through the JSON transport.
	assert.Equal(t, float64(203), sc["vmid"])
}

// TestLXCCreateToolErrorPropagation verifies IsError on gateway failure for
// pve_lxc_create.
func TestLXCCreateToolErrorPropagation(t *testing.T) {
	gateway := &stubPVEGateway{err: port.ErrForbidden}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(gateway))
	_, sess := newTestServer(t, app, true)

	res := callTool(t, sess, "pve_lxc_create",
		map[string]any{"node": "pve1", "ostemplate": "local:vztmpl/tpl"})
	assert.True(t, res.IsError, "pve_lxc_create should be IsError on gateway failure")
}

// TestLXCCreateToolGateOff verifies pve_lxc_create is a mutation tool absent
// when the gate is OFF.
func TestLXCCreateToolGateOff(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServer(t, app, false)
	names := toolNames(t, sess)

	assert.False(t, names["pve_lxc_create"],
		"pve_lxc_create is a mutation and must be gated off")
}
