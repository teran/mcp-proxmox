package pve

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/model"
	"github.com/teran/mcp-proxmox/domain/port"
)

// TestGateway_CloneLXC_Success verifies POST .../lxc/{vmid}/clone encodes the
// clone overrides as form params.
func TestGateway_CloneLXC_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":"UPID:pve1:lxcclone:..."}`, rec)
	g, _ := newTestGateway(t, srv)

	task, err := g.CloneLXC(context.Background(), "pve1", 200, model.CloneLXCRequest{
		NewID: 201, Full: true, Storage: "local", Hostname: "ct-clone",
		Description: "d", Pool: "pool1", Snapname: "snap1",
	})
	require.NoError(t, err)
	require.NotNil(t, task)
	assert.Equal(t, "UPID:pve1:lxcclone:...", task.UPID)

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api2/json/nodes/pve1/lxc/200/clone", rec.path)
	assert.Equal(t, "201", rec.form.Get("newid"))
	assert.Equal(t, "1", rec.form.Get("full"))
	assert.Equal(t, "local", rec.form.Get("storage"))
	assert.Equal(t, "ct-clone", rec.form.Get("hostname"))
	assert.Equal(t, "d", rec.form.Get("description"))
	assert.Equal(t, "pool1", rec.form.Get("pool"))
	assert.Equal(t, "snap1", rec.form.Get("snapname"))
}

// TestGateway_ListLXCSnapshots_Success verifies GET .../snapshot returns the
// decoded Snapshot list.
func TestGateway_ListLXCSnapshots_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[{"name":"snap1","snaptime":1704067200}]}`, rec)
	g, _ := newTestGateway(t, srv)

	snaps, err := g.ListLXCSnapshots(context.Background(), "pve1", 200)
	require.NoError(t, err)
	require.Len(t, snaps, 1)
	assert.Equal(t, "snap1", snaps[0].Name)
	assert.Equal(t, int64(1704067200), snaps[0].Snaptime)

	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/api2/json/nodes/pve1/lxc/200/snapshot", rec.path)
}

// TestGateway_LXCSnapshotMutations_Success verifies create/delete/rollback of
// LXC snapshots.
func TestGateway_LXCSnapshotMutations_Success(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, 200, `{"data":"UPID:create:..."}`, rec)
		g, _ := newTestGateway(t, srv)

		_, err := g.CreateLXCSnapshot(context.Background(), "pve1", 200, model.SnapshotCreateRequest{
			Snapname: "snap1", Description: "d",
		})
		require.NoError(t, err)
		assert.Equal(t, http.MethodPost, rec.method)
		assert.Equal(t, "/api2/json/nodes/pve1/lxc/200/snapshot", rec.path)
		assert.Equal(t, "snap1", rec.form.Get("snapname"))
		assert.Equal(t, "d", rec.form.Get("description"))
		// LXC snapshots do not capture a VM state.
		assert.Equal(t, "", rec.form.Get("vmstate"))
	})

	t.Run("delete", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, 200, `{"data":"UPID:delete:..."}`, rec)
		g, _ := newTestGateway(t, srv)

		_, err := g.DeleteLXCSnapshot(context.Background(), "pve1", 200, "snap1")
		require.NoError(t, err)
		assert.Equal(t, http.MethodDelete, rec.method)
		assert.Equal(t, "/api2/json/nodes/pve1/lxc/200/snapshot/snap1", rec.path)
	})

	t.Run("rollback", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, 200, `{"data":"UPID:rollback:..."}`, rec)
		g, _ := newTestGateway(t, srv)

		_, err := g.RollbackLXCSnapshot(context.Background(), "pve1", 200, "snap1")
		require.NoError(t, err)
		assert.Equal(t, http.MethodPost, rec.method)
		assert.Equal(t, "/api2/json/nodes/pve1/lxc/200/snapshot/snap1/rollback", rec.path)
	})
}

// TestGateway_LXCCloneSnapshot_ErrorNoRetry verifies clone/snapshot mutations
// surface errors and are never retried.
func TestGateway_LXCCloneSnapshot_ErrorNoRetry(t *testing.T) {
	t.Run("clone upstream error no retry", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.CloneLXC(context.Background(), "pve1", 200, model.CloneLXCRequest{NewID: 201})
		require.Error(t, err)
		var ue *UpstreamError
		assert.ErrorAs(t, err, &ue)
		assert.Equal(t, 1, rec.count)
	})

	t.Run("snapshot delete unauthorized", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusUnauthorized, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.DeleteLXCSnapshot(context.Background(), "pve1", 200, "snap1")
		require.Error(t, err)
		assert.ErrorIs(t, err, port.ErrUnauthorized)
		assert.Equal(t, 1, rec.count)
	})

	t.Run("snapshot list empty data", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, 200, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		snaps, err := g.ListLXCSnapshots(context.Background(), "pve1", 200)
		require.NoError(t, err)
		assert.Empty(t, snaps)
	})
}
