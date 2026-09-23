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

// TestGateway_CloneVM_Success verifies POST .../qemu/{vmid}/clone encodes the
// clone overrides as form params and wraps the returned UPID in a Task.
func TestGateway_CloneVM_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":"UPID:pve1:clone:..."}`, rec)
	g, _ := newTestGateway(t, srv)

	task, err := g.CloneVM(context.Background(), "pve1", 100, model.CloneVMRequest{
		NewID: 101, Name: "web-clone", Full: true, Storage: "local",
		Pool: "pool1", Description: "d", Format: "qcow2", Snapname: "snap1",
	})
	require.NoError(t, err)
	require.NotNil(t, task)
	assert.Equal(t, "UPID:pve1:clone:...", task.UPID)

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api2/json/nodes/pve1/qemu/100/clone", rec.path)
	assert.Equal(t, "101", rec.form.Get("newid"))
	assert.Equal(t, "web-clone", rec.form.Get("name"))
	assert.Equal(t, "1", rec.form.Get("full"))
	assert.Equal(t, "local", rec.form.Get("storage"))
	assert.Equal(t, "pool1", rec.form.Get("pool"))
	assert.Equal(t, "d", rec.form.Get("description"))
	assert.Equal(t, "qcow2", rec.form.Get("format"))
	assert.Equal(t, "snap1", rec.form.Get("snapname"))
}

// TestGateway_ListVMSnapshots_Success verifies GET .../snapshot returns the
// decoded Snapshot list.
func TestGateway_ListVMSnapshots_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[{"name":"snap1","snaptime":1704067200,"vmstate":1}]}`, rec)
	g, _ := newTestGateway(t, srv)

	snaps, err := g.ListVMSnapshots(context.Background(), "pve1", 100)
	require.NoError(t, err)
	require.Len(t, snaps, 1)
	assert.Equal(t, "snap1", snaps[0].Name)
	assert.Equal(t, int64(1704067200), snaps[0].Snaptime)
	assert.Equal(t, 1, snaps[0].VMState)

	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/api2/json/nodes/pve1/qemu/100/snapshot", rec.path)
}

// TestGateway_VMSnapshotMutations_Success verifies create/delete/rollback of VM
// snapshots.
func TestGateway_VMSnapshotMutations_Success(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, 200, `{"data":"UPID:create:..."}`, rec)
		g, _ := newTestGateway(t, srv)

		_, err := g.CreateVMSnapshot(context.Background(), "pve1", 100, model.SnapshotCreateRequest{
			Snapname: "snap1", VMState: true, Description: "d",
		})
		require.NoError(t, err)
		assert.Equal(t, http.MethodPost, rec.method)
		assert.Equal(t, "/api2/json/nodes/pve1/qemu/100/snapshot", rec.path)
		assert.Equal(t, "snap1", rec.form.Get("snapname"))
		assert.Equal(t, "1", rec.form.Get("vmstate"))
		assert.Equal(t, "d", rec.form.Get("description"))
	})

	t.Run("delete", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, 200, `{"data":"UPID:delete:..."}`, rec)
		g, _ := newTestGateway(t, srv)

		_, err := g.DeleteVMSnapshot(context.Background(), "pve1", 100, "snap1")
		require.NoError(t, err)
		assert.Equal(t, http.MethodDelete, rec.method)
		assert.Equal(t, "/api2/json/nodes/pve1/qemu/100/snapshot/snap1", rec.path)
	})

	t.Run("rollback", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, 200, `{"data":"UPID:rollback:..."}`, rec)
		g, _ := newTestGateway(t, srv)

		_, err := g.RollbackVMSnapshot(context.Background(), "pve1", 100, "snap1")
		require.NoError(t, err)
		assert.Equal(t, http.MethodPost, rec.method)
		assert.Equal(t, "/api2/json/nodes/pve1/qemu/100/snapshot/snap1/rollback", rec.path)
	})
}

// TestGateway_VMCloneSnapshot_ErrorNoRetry verifies clone/snapshot mutations
// surface errors and are never retried.
func TestGateway_VMCloneSnapshot_ErrorNoRetry(t *testing.T) {
	t.Run("clone upstream error no retry", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.CloneVM(context.Background(), "pve1", 100, model.CloneVMRequest{NewID: 101})
		require.Error(t, err)
		var ue *UpstreamError
		assert.ErrorAs(t, err, &ue)
		assert.Equal(t, 1, rec.count)
	})

	t.Run("snapshot delete unauthorized", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusUnauthorized, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.DeleteVMSnapshot(context.Background(), "pve1", 100, "snap1")
		require.Error(t, err)
		assert.ErrorIs(t, err, port.ErrUnauthorized)
		assert.Equal(t, 1, rec.count)
	})

	t.Run("snapshot list empty data", func(t *testing.T) {
		// A list getter with null data returns an empty (non-error) result.
		rec := &reqCapture{}
		srv := mockServer(t, 200, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		snaps, err := g.ListVMSnapshots(context.Background(), "pve1", 100)
		require.NoError(t, err)
		assert.Empty(t, snaps)
	})
}
