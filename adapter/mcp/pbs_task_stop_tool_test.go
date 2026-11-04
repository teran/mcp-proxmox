package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/port"
)

// TestPBSTaskStopToolForwardsUPID verifies the pbs_task_stop mutation tool
// forwards the provided UPID to PBSGateway.StopTask and returns no output.
func TestPBSTaskStopToolForwardsUPID(t *testing.T) {
	gotUPID := ""
	gateway := &stubPBSGateway{stopTask: func(_ context.Context, upid string) error {
		gotUPID = upid

		return nil
	}}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPBS(gateway))
	_, sess := newTestServer(t, app, true)

	res := callTool(t, sess, "pbs_task_stop", map[string]any{"upid": "UPID:pbs:task:..."})
	assert.False(t, res.IsError, "tool should succeed")
	assert.Equal(t, "UPID:pbs:task:...", gotUPID)
}

// TestPBSTaskStopToolErrorPropagation verifies IsError on gateway failure.
func TestPBSTaskStopToolErrorPropagation(t *testing.T) {
	gateway := &stubPBSGateway{err: port.ErrNotFound}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPBS(gateway))
	_, sess := newTestServer(t, app, true)

	res := callTool(t, sess, "pbs_task_stop", map[string]any{"upid": "UPID:pbs:task:..."})
	require.NotNil(t, res)
	assert.True(t, res.IsError, "tool should be flagged IsError on gateway failure")
}
