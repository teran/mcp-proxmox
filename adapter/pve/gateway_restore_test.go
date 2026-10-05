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

// TestGateway_RestoreVM_WithVMID verifies POST /nodes/{node}/qemu with a VMID
// encodes the restore overrides as form params (no restore flag).
func TestGateway_RestoreVM_WithVMID(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":"UPID:pve1:restore:..."}`, rec)
	g, _ := newTestGateway(t, srv)

	task, err := g.RestoreVM(context.Background(), "pve1", model.PVERestoreRequest{
		Archive: "/var/lib/vz/dump/vzdump-qemu-100.vma.zst", VMID: 100,
		Storage: "local", Unique: true, Force: true, Pool: "pool1", BwLimit: 4096,
	})
	require.NoError(t, err)
	require.NotNil(t, task)
	assert.Equal(t, "UPID:pve1:restore:...", task.UPID)

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api2/json/nodes/pve1/qemu", rec.path)
	assert.Equal(t, "/var/lib/vz/dump/vzdump-qemu-100.vma.zst", rec.form.Get("archive"))
	assert.Equal(t, "100", rec.form.Get("vmid"))
	assert.Equal(t, "", rec.form.Get("restore"))
	assert.Equal(t, "local", rec.form.Get("storage"))
	assert.Equal(t, "1", rec.form.Get("unique"))
	assert.Equal(t, "1", rec.form.Get("force"))
	assert.Equal(t, "pool1", rec.form.Get("pool"))
	assert.Equal(t, "4096", rec.form.Get("bwlimit"))
}

// TestGateway_RestoreVM_NoVMID verifies that when no VMID is supplied the
// restore flag is set so PVE restores to the original ID.
func TestGateway_RestoreVM_NoVMID(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":"UPID:pve1:restore:..."}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.RestoreVM(context.Background(), "pve1", model.PVERestoreRequest{
		Archive: "/var/lib/vz/dump/vzdump-qemu-100.vma.zst",
	})
	require.NoError(t, err)

	assert.Equal(t, "1", rec.form.Get("restore"))
	assert.Equal(t, "", rec.form.Get("vmid"))
}

// TestGateway_RestoreVM_ErrorNoRetry verifies restore surfaces errors and is
// never retried.
func TestGateway_RestoreVM_ErrorNoRetry(t *testing.T) {
	t.Run("unauthorized", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusUnauthorized, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.RestoreVM(context.Background(), "pve1", model.PVERestoreRequest{Archive: "/a.bak"})
		require.Error(t, err)
		assert.ErrorIs(t, err, port.ErrUnauthorized)
		assert.Equal(t, 1, rec.count)
	})

	t.Run("upstream error no retry", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.RestoreVM(context.Background(), "pve1", model.PVERestoreRequest{Archive: "/a.bak"})
		require.Error(t, err)
		var ue *UpstreamError
		assert.ErrorAs(t, err, &ue)
		assert.Equal(t, 1, rec.count)
	})
}
