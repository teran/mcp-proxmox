package pbs

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/port"
)

// TestGateway_ListBackupFiles_Success verifies
// GET /admin/datastore/{store}/files?backup-type=...&backup-id=...&backup-time=<epoch>&path=...
// returns the decoded PBSFile list, forwarding the optional path query.
func TestGateway_ListBackupFiles_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[{"filename":"client.logidx","type":"log","size":1024}]}`, rec)
	g, _ := newTestGateway(t, srv)

	files, err := g.ListBackupFiles(context.Background(), "backup", "vm", "100", 1793662080, "/var/lib/vz")
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, "client.logidx", files[0].Filename)
	assert.Equal(t, "log", files[0].Type)
	assert.Equal(t, int64(1024), files[0].Size)

	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/api2/json/admin/datastore/backup/files", rec.path)
	assert.Equal(t, "backup-id=100&backup-time=1793662080&backup-type=vm&path=%2Fvar%2Flib%2Fvz", rec.query)
}

// TestGateway_ListBackupFiles_NoPath verifies the path query is omitted when
// empty, while backup-type/backup-id/backup-time are always present.
func TestGateway_ListBackupFiles_NoPath(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[{"filename":"f"}]}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.ListBackupFiles(context.Background(), "backup", "vm", "100", 1793662080, "")
	require.NoError(t, err)
	assert.Equal(t, "backup-id=100&backup-time=1793662080&backup-type=vm", rec.query)
}

// TestGateway_GetTaskStatus_Success verifies GET /nodes/localhost/tasks/{upid}/status
// decodes a PBSTask.
func TestGateway_GetTaskStatus_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":{"upid":"upid1","type":"restore","status":"running","worker":"w","errors":2}}`, rec)
	g, _ := newTestGateway(t, srv)

	task, err := g.GetTaskStatus(context.Background(), "upid1")
	require.NoError(t, err)
	require.NotNil(t, task)
	assert.Equal(t, "upid1", task.UPID)
	assert.Equal(t, "restore", task.Type)
	assert.Equal(t, "running", task.Status)
	assert.Equal(t, "w", task.Worker)
	assert.Equal(t, 2, task.Errors)

	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/api2/json/nodes/localhost/tasks/upid1/status", rec.path)
}

// TestGateway_GetTaskLog_Success verifies GET /nodes/localhost/tasks/{upid}/log
// decodes the log entries and forwards the limit query.
func TestGateway_GetTaskLog_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[{"n":1,"t":"TASK OK"}]}`, rec)
	g, _ := newTestGateway(t, srv)

	entries, err := g.GetTaskLog(context.Background(), "upid1", 50)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, 1, entries[0].LineNumber)
	assert.Equal(t, "TASK OK", entries[0].Text)

	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/api2/json/nodes/localhost/tasks/upid1/log", rec.path)
	assert.Equal(t, "limit=50", rec.query)
}

// TestGateway_GetTaskLog_NoLimit verifies the limit query is omitted when zero.
func TestGateway_GetTaskLog_NoLimit(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[]}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.GetTaskLog(context.Background(), "upid1", 0)
	require.NoError(t, err)
	assert.Equal(t, "", rec.query)
}

// TestGateway_PBSReadOnlyErrorBranches covers the error/empty paths for the new
// read-only methods.
func TestGateway_PBSReadOnlyErrorBranches(t *testing.T) {
	t.Run("list files upstream error", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.ListBackupFiles(context.Background(), "s", "vm", "100", 1793662080, "")
		require.Error(t, err)
	})

	t.Run("task status unauthorized", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusUnauthorized, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.GetTaskStatus(context.Background(), "upid1")
		require.Error(t, err)
		assert.ErrorIs(t, err, port.ErrUnauthorized)
	})

	t.Run("task log empty data is empty not error", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, 200, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		entries, err := g.GetTaskLog(context.Background(), "upid1", 0)
		require.NoError(t, err)
		assert.Empty(t, entries)
	})
}

// TestGateway_ListBackups_PaginationQuery verifies the pagination options are
// translated into the limit/start/end query parameters, and that zero-valued
// options are omitted.
func TestGateway_ListBackups_PaginationQuery(t *testing.T) {
	cases := []struct {
		name      string
		opts      port.PBSBackupListOptions
		wantLimit string
		wantStart string
		wantEnd   string
	}{
		{"limit only", port.PBSBackupListOptions{Limit: 5}, "5", "", ""},
		{"limit start end", port.PBSBackupListOptions{Limit: 5, Start: 1700000000, End: 1700000100}, "5", "1700000000", "1700000100"},
		{"none", port.PBSBackupListOptions{}, "", "", ""},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			rec := &reqCapture{}
			srv := mockServer(t, 200, `{"data":[]}`, rec)
			g, _ := newTestGateway(t, srv)

			_, err := g.ListBackups(context.Background(), "backup", tt.opts)
			require.NoError(t, err)

			q, err := url.ParseQuery(rec.query)
			require.NoError(t, err)
			assert.Equal(t, tt.wantLimit, q.Get("limit"))
			assert.Equal(t, tt.wantStart, q.Get("start"))
			assert.Equal(t, tt.wantEnd, q.Get("end"))
		})
	}
}

// TestGateway_ListBackups_TrimsToLimit verifies the client-side safety trim: when
// the upstream returns more snapshots than opts.Limit, the result is trimmed to
// exactly Limit elements.
func TestGateway_ListBackups_TrimsToLimit(t *testing.T) {
	rec := &reqCapture{}
	body := `{"data":[` + strings.Repeat(`{"backup-id":"vm/100/x","backup-time":1,"backup-type":"vm"},`, 9) +
		`{"backup-id":"vm/100/x","backup-time":10,"backup-type":"vm"}]}`
	srv := mockServer(t, 200, body, rec)
	g, _ := newTestGateway(t, srv)

	out, err := g.ListBackups(context.Background(), "backup", port.PBSBackupListOptions{Limit: 5})
	require.NoError(t, err)
	require.Len(t, out, 5, "result must be trimmed to opts.Limit")
	assert.Equal(t, "limit=5", rec.query)
}

// TestGateway_GetBackup_ForwardsOpts verifies GetBackup passes the pagination
// options through to the underlying listing while still filtering by backup-id.
func TestGateway_GetBackup_ForwardsOpts(t *testing.T) {
	rec := &reqCapture{}
	body := `{"data":[{"backup-id":"vm/100/x","backup-time":1,"backup-type":"vm"},` +
		`{"backup-id":"vm/101/y","backup-time":2,"backup-type":"vm"}]}`
	srv := mockServer(t, 200, body, rec)
	g, _ := newTestGateway(t, srv)

	out, err := g.GetBackup(context.Background(), "backup", "vm/100/x",
		port.PBSBackupListOptions{Limit: 3, Start: 100, End: 200})
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, "vm/100/x", out[0].BackupID)

	q, err := url.ParseQuery(rec.query)
	require.NoError(t, err)
	assert.Equal(t, "3", q.Get("limit"))
	assert.Equal(t, "100", q.Get("start"))
	assert.Equal(t, "200", q.Get("end"))
}
