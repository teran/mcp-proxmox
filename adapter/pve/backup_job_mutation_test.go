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

// boolPtr returns a pointer to b so an explicit false can be sent for Enabled.
func boolPtr(b bool) *bool { return &b }

// TestGateway_CreateBackupJob_Success verifies POST /cluster/backup sends the
// job config as form parameters and decodes the returned job ID from `data`.
func TestGateway_CreateBackupJob_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":"backup-100"}`, rec)
	g, _ := newTestGateway(t, srv)

	id, err := g.CreateBackupJob(context.Background(), model.CreateBackupJobRequest{
		ID:            "backup-100",
		All:           true,
		VMID:          "100,101",
		Storage:       "local",
		Schedule:      "00 2 * * 1",
		Mode:          "snapshot",
		Compress:      "zstd",
		Enabled:       boolPtr(true),
		NotesTemplate: "{{guestname}}",
		Comment:       "prod cluster",
		Node:          "pve1",
	})
	require.NoError(t, err)
	assert.Equal(t, "backup-100", id)

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api2/json/cluster/backup", rec.path)
	assert.Equal(t, "PVEAPIToken=user@pam!tokenid=secret", rec.auth)
	assert.Equal(t, "backup-100", rec.form.Get("id"))
	assert.Equal(t, "1", rec.form.Get("all"))
	assert.Equal(t, "100,101", rec.form.Get("vmid"))
	assert.Equal(t, "local", rec.form.Get("storage"))
	assert.Equal(t, "00 2 * * 1", rec.form.Get("schedule"))
	assert.Equal(t, "snapshot", rec.form.Get("mode"))
	assert.Equal(t, "zstd", rec.form.Get("compress"))
	assert.Equal(t, "1", rec.form.Get("enabled"))
	assert.Equal(t, "{{guestname}}", rec.form.Get("notes-template"))
	assert.Equal(t, "prod cluster", rec.form.Get("comment"))
	assert.Equal(t, "pve1", rec.form.Get("node"))
}

// TestGateway_CreateBackupJob_Disabled verifies an explicit Enabled=false is
// sent as enabled=0 (a non-nil pointer overrides the default-kept behaviour).
func TestGateway_CreateBackupJob_Disabled(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":"backup-100"}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.CreateBackupJob(context.Background(), model.CreateBackupJobRequest{
		Storage: "local", Schedule: "00 2 * * 1", Enabled: boolPtr(false),
	})
	require.NoError(t, err)
	assert.Equal(t, "0", rec.form.Get("enabled"))
}

// TestGateway_CreateBackupJob_MinimalForm verifies an omitted optional field is
// not sent, so only storage/schedule (and id when provided) appear in the form.
func TestGateway_CreateBackupJob_MinimalForm(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":"backup-100"}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.CreateBackupJob(context.Background(), model.CreateBackupJobRequest{
		Storage: "local", Schedule: "00 2 * * 1",
	})
	require.NoError(t, err)
	assert.Equal(t, "local", rec.form.Get("storage"))
	assert.Equal(t, "00 2 * * 1", rec.form.Get("schedule"))
	assert.Empty(t, rec.form.Get("id"))
	assert.Empty(t, rec.form.Get("all"))
	assert.Empty(t, rec.form.Get("vmid"))
	assert.Empty(t, rec.form.Get("mode"))
	assert.Empty(t, rec.form.Get("enabled"))
}

// TestGateway_UpdateBackupJob_Success verifies PUT /cluster/backup/{id} with the
// id URL-escaped into the path and the form body carrying the updated fields.
func TestGateway_UpdateBackupJob_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":"backup-100"}`, rec)
	g, _ := newTestGateway(t, srv)

	id, err := g.UpdateBackupJob(context.Background(), "backup-100", model.CreateBackupJobRequest{
		Storage: "nfs-backup", Schedule: "00 3 * * 1", Mode: "suspend",
	})
	require.NoError(t, err)
	assert.Equal(t, "backup-100", id)

	assert.Equal(t, http.MethodPut, rec.method)
	assert.Equal(t, "/api2/json/cluster/backup/backup-100", rec.path)
	assert.Equal(t, "PVEAPIToken=user@pam!tokenid=secret", rec.auth)
	assert.Equal(t, "nfs-backup", rec.form.Get("storage"))
	assert.Equal(t, "00 3 * * 1", rec.form.Get("schedule"))
	assert.Equal(t, "suspend", rec.form.Get("mode"))
}

// TestGateway_BackupJobIDPathEscaped verifies a job id containing characters
// that are unsafe in a URL path is escaped with url.PathEscape for both update
// and delete.
func TestGateway_BackupJobIDPathEscaped(t *testing.T) {
	t.Run("update", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, 200, `{"data":"backup/100"}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.UpdateBackupJob(context.Background(), "backup/100", model.CreateBackupJobRequest{
			Storage: "local", Schedule: "00 2 * * 1",
		})
		require.NoError(t, err)
		// The decoded Path unescapes %2F; the escaped form is in RawPath.
		assert.Equal(t, "/api2/json/cluster/backup/backup/100", rec.path)
		assert.Equal(t, "/api2/json/cluster/backup/backup%2F100", rec.rawPath)
	})

	t.Run("delete", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, 200, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		err := g.DeleteBackupJob(context.Background(), "backup/100")
		require.NoError(t, err)
		assert.Equal(t, "/api2/json/cluster/backup/backup/100", rec.path)
		assert.Equal(t, "/api2/json/cluster/backup/backup%2F100", rec.rawPath)
	})
}

// TestGateway_DeleteBackupJob_Success verifies DELETE /cluster/backup/{id}.
func TestGateway_DeleteBackupJob_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":null}`, rec)
	g, _ := newTestGateway(t, srv)

	err := g.DeleteBackupJob(context.Background(), "backup-100")
	require.NoError(t, err)

	assert.Equal(t, http.MethodDelete, rec.method)
	assert.Equal(t, "/api2/json/cluster/backup/backup-100", rec.path)
	assert.Equal(t, "PVEAPIToken=user@pam!tokenid=secret", rec.auth)
}

// TestGateway_BackupJobErrorMapping verifies backup-job mutation failures map
// through the error family and are never retried (mutations are not replayed):
// 401 -> ErrUnauthorized, 5xx -> UpstreamError with a single request.
func TestGateway_BackupJobErrorMapping(t *testing.T) {
	t.Run("create unauthorized", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusUnauthorized, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.CreateBackupJob(context.Background(), model.CreateBackupJobRequest{Storage: "s", Schedule: "*"})
		require.Error(t, err)
		assert.ErrorIs(t, err, port.ErrUnauthorized)
		assert.Equal(t, 1, rec.count)
	})

	t.Run("create upstream error no retry", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.CreateBackupJob(context.Background(), model.CreateBackupJobRequest{Storage: "s", Schedule: "*"})
		require.Error(t, err)
		var ue *UpstreamError
		assert.ErrorAs(t, err, &ue)
		assert.Equal(t, 1, rec.count, "mutations must not be retried")
	})

	t.Run("update upstream error no retry", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.UpdateBackupJob(context.Background(), "backup-100", model.CreateBackupJobRequest{Storage: "s", Schedule: "*"})
		require.Error(t, err)
		assert.Equal(t, 1, rec.count, "mutations must not be retried")
	})

	t.Run("delete upstream error no retry", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		err := g.DeleteBackupJob(context.Background(), "backup-100")
		require.Error(t, err)
		var ue *UpstreamError
		assert.ErrorAs(t, err, &ue)
		assert.Equal(t, 1, rec.count, "mutations must not be retried")
	})
}
