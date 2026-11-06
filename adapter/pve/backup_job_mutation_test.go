package pve

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/model"
	"github.com/teran/mcp-proxmox/domain/port"
)

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
		Enabled:       new(true),
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

// TestGateway_CreateBackupJob_ExplicitIDEmptyResponse is the regression test for
// Defect 1 `pve_backup_job_create`. PVE does NOT echo an explicitly-provided job
// ID back in the POST /cluster/backup response body — it replies with empty data
// (e.g. `{"data":null}` or `{"data":""}`) while the job is still created.
// CreateBackupJob must return the caller-supplied ID verbatim rather than
// extracting "" from the empty response data. Before the fix this returned ""
// even with ID: "backup-200".
func TestGateway_CreateBackupJob_ExplicitIDEmptyResponse(t *testing.T) {
	for _, body := range []string{`{"data":null}`, `{"data":""}`} {
		t.Run(strings.ReplaceAll(body, "\"", "q"), func(t *testing.T) {
			rec := &reqCapture{}
			srv := mockServer(t, 200, body, rec)
			g, _ := newTestGateway(t, srv)

			id, err := g.CreateBackupJob(context.Background(), model.CreateBackupJobRequest{
				ID: "backup-200", Storage: "local", Schedule: "00 2 * * 1",
			})
			require.NoError(t, err)
			assert.Equal(t, "backup-200", id, "explicit ID must be returned verbatim even with empty data in the PVE response")

			assert.Equal(t, http.MethodPost, rec.method)
			assert.Equal(t, "/api2/json/cluster/backup", rec.path)
			assert.Equal(t, "backup-200", rec.form.Get("id"))
		})
	}
}

// TestGateway_CreateBackupJob_NoIDFallsBackToResponse verifies that when the
// caller supplies no ID (ID=""), CreateBackupJob falls back to extracting the job
// ID from the response `data` via backupJobIDFromData. A real PVE echoes the
// generated ID as a bare JSON string in `data`.
func TestGateway_CreateBackupJob_NoIDFallsBackToResponse(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":"backup-300"}`, rec)
	g, _ := newTestGateway(t, srv)

	id, err := g.CreateBackupJob(context.Background(), model.CreateBackupJobRequest{
		Storage: "local", Schedule: "00 2 * * 1",
	})
	require.NoError(t, err)
	assert.Equal(t, "backup-300", id, "ID must fall back to the value echoed in the response data")

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api2/json/cluster/backup", rec.path)
	assert.Empty(t, rec.form.Get("id"))
}

// TestGateway_CreateBackupJob_Disabled verifies an explicit Enabled=false is
// sent as enabled=0 (a non-nil pointer overrides the default-kept behaviour).
func TestGateway_CreateBackupJob_Disabled(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":"backup-100"}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.CreateBackupJob(context.Background(), model.CreateBackupJobRequest{
		Storage: "local", Schedule: "00 2 * * 1", Enabled: new(false),
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

// TestGateway_UpdateBackupJob_EmptyDataNoError verifies UpdateBackupJob tolerates
// a PVE response whose `data` is empty (`null`) or an empty object `{}` — PVE
// reports a successful job update with no echoed job ID, mirroring
// CreateBackupJob. These must NOT surface an EmptyDataError (the current bug).
func TestGateway_UpdateBackupJob_EmptyDataNoError(t *testing.T) {
	for _, body := range []string{`{"data":null}`, `{}`} {
		t.Run(body, func(t *testing.T) {
			rec := &reqCapture{}
			srv := mockServer(t, 200, body, rec)
			g, _ := newTestGateway(t, srv)

			_, err := g.UpdateBackupJob(context.Background(), "backup-100", model.CreateBackupJobRequest{
				Storage: "nfs-backup", Schedule: "00 3 * * 1",
			})
			require.NoError(t, err)
			assert.Equal(t, http.MethodPut, rec.method)
			assert.Equal(t, "/api2/json/cluster/backup/backup-100", rec.path)
		})
	}
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

// TestGateway_DeleteBackupJob_EmptyBodyNoError is the RED reproducer for the
// Defect 1 `pve_backup_job_delete` HTTP 500-on-success. A real PVE deletes the
// job and replies with a 200 and an EMPTY body (no JSON envelope at all), which
// the envelope decode (json.Unmarshal) cannot parse. DeleteBackupJob must treat
// ANY 2xx as success (raw body, no envelope decode), so an empty body, a null
// data and an empty object must ALL succeed with no error. Today the empty body
// case fails with UpstreamError{Op:"decode"}.
func TestGateway_DeleteBackupJob_EmptyBodyNoError(t *testing.T) {
	for _, body := range []string{``, `{"data":null}`, `{}`} {
		t.Run(strings.ReplaceAll(body, "\"", "q"), func(t *testing.T) {
			rec := &reqCapture{}
			srv := mockServer(t, 200, body, rec)
			g, _ := newTestGateway(t, srv)

			err := g.DeleteBackupJob(context.Background(), "backup-100")
			require.NoError(t, err, "DeleteBackupJob must treat body %q as success", body)

			assert.Equal(t, http.MethodDelete, rec.method)
			assert.Equal(t, "/api2/json/cluster/backup/backup-100", rec.path)
			assert.Equal(t, "PVEAPIToken=user@pam!tokenid=secret", rec.auth)
		})
	}
}

// TestGateway_DeleteBackupJob_NotFound verifies a 404 for an unknown job maps
// to the ErrNotFound sentinel (not an upstream error).
func TestGateway_DeleteBackupJob_NotFound(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusNotFound, `{"data":null}`, rec)
	g, _ := newTestGateway(t, srv)

	err := g.DeleteBackupJob(context.Background(), "backup-999")
	require.Error(t, err)
	assert.ErrorIs(t, err, port.ErrNotFound)
	assert.Equal(t, 1, rec.count)
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

// TestGateway_BackupJobIDFromData covers the lenient extraction of a backup-job
// ID from a create/update response `data`. A real PVE returns the ID in several
// shapes — a bare JSON string, an object with an `id` key, an object wrapping a
// nested `data.id`, or nothing at all (`null`, `{}`, empty). backupJobIDFromData
// must resolve the ID from each form, returning "" when none is present.
func TestGateway_BackupJobIDFromData(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"bare string", `"backup-100"`, "backup-100"},
		{"object with id", `{"id":"backup-100"}`, "backup-100"},
		{"nested data id", `{"data":{"id":"x"}}`, "x"},
		{"null", `null`, ""},
		{"empty object", `{}`, ""},
		{"empty string", `""`, ""},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got := backupJobIDFromData(json.RawMessage(tt.raw))
			assert.Equal(t, tt.want, got)
		})
	}
}
