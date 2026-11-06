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

// TestGateway_ListBackups_NoPaginationQuery verifies the limit/start/end options
// are NOT forwarded to PBS 4.2, which rejects GET /admin/datastore/{store}/snapshots
// with limit/start/end (it has no server-side pagination for this endpoint). The
// query must contain none of these keys; the path is the plural snapshots
// endpoint. (limit/start/end are applied purely client-side.)
func TestGateway_ListBackups_NoPaginationQuery(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[]}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.ListBackups(context.Background(), "backup", port.PBSBackupListOptions{Limit: 5, Start: 1700000000, End: 1700000100})
	require.NoError(t, err)

	assert.Equal(t, "/api2/json/admin/datastore/backup/snapshots", rec.path)

	q, err := url.ParseQuery(rec.query)
	require.NoError(t, err)
	assert.NotContains(t, q, "limit", "PBS rejects the limit query parameter; it must not be sent")
	assert.NotContains(t, q, "start", "PBS rejects the start query parameter; it must not be sent")
	assert.NotContains(t, q, "end", "PBS rejects the end query parameter; it must not be sent")
}

// TestGateway_ListBackups_TrimsToLimit verifies the client-side limit: when the
// upstream returns more snapshots than opts.Limit, the result is trimmed to
// exactly Limit elements. No limit query is sent to PBS.
func TestGateway_ListBackups_TrimsToLimit(t *testing.T) {
	rec := &reqCapture{}
	body := `{"data":[` + strings.Repeat(`{"backup-id":"vm/100/x","backup-time":1,"backup-type":"vm"},`, 9) +
		`{"backup-id":"vm/100/x","backup-time":10,"backup-type":"vm"}]}`
	srv := mockServer(t, 200, body, rec)
	g, _ := newTestGateway(t, srv)

	out, err := g.ListBackups(context.Background(), "backup", port.PBSBackupListOptions{Limit: 5})
	require.NoError(t, err)
	require.Len(t, out, 5, "result must be trimmed to opts.Limit client-side")

	q, err := url.ParseQuery(rec.query)
	require.NoError(t, err)
	assert.NotContains(t, q, "limit", "limit must be applied client-side, not sent to PBS")
}

// TestGateway_ListBackups_StartEndFilter verifies the start/end options filter
// snapshots by their BackupTime (Unix epoch) purely client-side, inclusively.
// The upstream query carries no start/end keys.
func TestGateway_ListBackups_StartEndFilter(t *testing.T) {
	rec := &reqCapture{}
	body := `{"data":[` +
		`{"backup-id":"vm/100/a","backup-time":100,"backup-type":"vm"},` +
		`{"backup-id":"vm/100/b","backup-time":150,"backup-type":"vm"},` +
		`{"backup-id":"vm/100/c","backup-time":200,"backup-type":"vm"}]}`
	srv := mockServer(t, 200, body, rec)
	g, _ := newTestGateway(t, srv)

	out, err := g.ListBackups(context.Background(), "backup", port.PBSBackupListOptions{Start: 120, End: 200})
	require.NoError(t, err)
	require.Len(t, out, 2, "only snapshots with BackupTime in [120, 200] are kept")
	assert.Equal(t, int64(150), out[0].BackupTime)
	assert.Equal(t, int64(200), out[1].BackupTime)

	q, err := url.ParseQuery(rec.query)
	require.NoError(t, err)
	assert.NotContains(t, q, "start", "start must be applied client-side, not sent to PBS")
	assert.NotContains(t, q, "end", "end must be applied client-side, not sent to PBS")
}

// TestGateway_GetBackup_ForwardsBackupID verifies GetBackup forwards the
// backup-id as a server-side query parameter (so it does not pull the entire
// list and filter locally), while never sending limit/start/end to PBS 4.2.
func TestGateway_GetBackup_ForwardsBackupID(t *testing.T) {
	rec := &reqCapture{}
	body := `{"data":[{"backup-id":"vm/100/x","backup-time":150,"backup-type":"vm"},` +
		`{"backup-id":"vm/101/y","backup-time":160,"backup-type":"vm"}]}`
	srv := mockServer(t, 200, body, rec)
	g, _ := newTestGateway(t, srv)

	out, err := g.GetBackup(context.Background(), "backup", "vm/100/x",
		port.PBSBackupListOptions{Limit: 3, Start: 100, End: 200})
	require.NoError(t, err)
	require.Len(t, out, 1)
	assert.Equal(t, "vm/100/x", out[0].BackupID)

	assert.Equal(t, "/api2/json/admin/datastore/backup/snapshots", rec.path)

	q, err := url.ParseQuery(rec.query)
	require.NoError(t, err)
	assert.Equal(t, "vm/100/x", q.Get("backup-id"), "GetBackup must forward backup-id in the query, not pull the whole list")
	assert.NotContains(t, q, "limit", "limit must not be sent to PBS")
	assert.NotContains(t, q, "start", "start must not be sent to PBS")
	assert.NotContains(t, q, "end", "end must not be sent to PBS")
}
