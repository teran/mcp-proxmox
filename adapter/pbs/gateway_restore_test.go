package pbs

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/model"
	"github.com/teran/mcp-proxmox/domain/port"
)

// TestGateway_RestoreFile_Success verifies
// POST /admin/datastore/{store}/backups/{type}/{id}/{snapshot}/files sends
// path/target as form params and returns no error.
func TestGateway_RestoreFile_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":null}`, rec)
	g, _ := newTestGateway(t, srv)

	err := g.RestoreFile(context.Background(), "backup", "vm", "100", "snap1", model.PBSFileRestoreRequest{
		Path: "/etc/passwd", Target: "/restore/passwd",
	})
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api2/json/admin/datastore/backup/backups/vm/100/snap1/files", rec.path)
	assert.Equal(t, "/etc/passwd", rec.form.Get("path"))
	assert.Equal(t, "/restore/passwd", rec.form.Get("target"))
}

// TestGateway_RestoreVMBackup_Success verifies
// POST .../backups/{type}/{id}/{snapshot}/restore encodes the restore form and
// returns the task UPID.
func TestGateway_RestoreVMBackup_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":"UPID:pbs:restore:..."}`, rec)
	g, _ := newTestGateway(t, srv)

	upid, err := g.RestoreVMBackup(context.Background(), "backup", "vm", "100", "snap1", model.PBSVMRestoreRequest{
		Target: "local", VMID: "next", Host: "pve1", Password: "secret",
		Fingerprint: "AA:BB", Pool: "pool1", Verbose: true, Reload: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "UPID:pbs:restore:...", upid)

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api2/json/admin/datastore/backup/backups/vm/100/snap1/restore", rec.path)
	assert.Equal(t, "local", rec.form.Get("target"))
	assert.Equal(t, "next", rec.form.Get("vmid"))
	assert.Equal(t, "pve1", rec.form.Get("host"))
	assert.Equal(t, "secret", rec.form.Get("password"))
	assert.Equal(t, "AA:BB", rec.form.Get("fingerprint"))
	assert.Equal(t, "pool1", rec.form.Get("pool"))
	assert.Equal(t, "1", rec.form.Get("verbose"))
	assert.Equal(t, "1", rec.form.Get("reload"))
}

// TestGateway_PBSRestore_ErrorNoRetry verifies PBS restore mutations surface
// errors and are never retried.
func TestGateway_PBSRestore_ErrorNoRetry(t *testing.T) {
	t.Run("restore file upstream error no retry", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		err := g.RestoreFile(context.Background(), "backup", "vm", "100", "snap1", model.PBSFileRestoreRequest{
			Path: "/a", Target: "/b",
		})
		require.Error(t, err)
		var ue *UpstreamError
		assert.ErrorAs(t, err, &ue)
		assert.Equal(t, 1, rec.count)
	})

	t.Run("restore vm backup unauthorized", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusUnauthorized, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.RestoreVMBackup(context.Background(), "backup", "vm", "100", "snap1", model.PBSVMRestoreRequest{Target: "local"})
		require.Error(t, err)
		assert.ErrorIs(t, err, port.ErrUnauthorized)
		assert.Equal(t, 1, rec.count)
	})
}
