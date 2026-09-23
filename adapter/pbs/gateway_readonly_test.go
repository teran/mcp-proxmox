package pbs

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/port"
)

// TestGateway_ListBackupFiles_Success verifies
// GET /admin/datastore/{store}/backups/{type}/{id}/{snapshot}/files returns the
// decoded PBSFile list, forwarding the optional path query.
func TestGateway_ListBackupFiles_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[{"filename":"client.logidx","type":"log","size":1024}]}`, rec)
	g, _ := newTestGateway(t, srv)

	files, err := g.ListBackupFiles(context.Background(), "backup", "vm", "100", "2024-01-01T00:00:00Z", "/var/lib/vz")
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, "client.logidx", files[0].Filename)
	assert.Equal(t, "log", files[0].Type)
	assert.Equal(t, int64(1024), files[0].Size)

	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/api2/json/admin/datastore/backup/backups/vm/100/2024-01-01T00:00:00Z/files", rec.path)
	assert.Equal(t, "path=%2Fvar%2Flib%2Fvz", rec.query)
}

// TestGateway_ListBackupFiles_NoPath verifies the path query is omitted when
// empty.
func TestGateway_ListBackupFiles_NoPath(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[{"filename":"f"}]}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.ListBackupFiles(context.Background(), "backup", "vm", "100", "snap", "")
	require.NoError(t, err)
	assert.Equal(t, "", rec.query)
}

// TestGateway_GetTaskStatus_Success verifies GET /admin/tasks/{upid} decodes a
// PBSTask.
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
	assert.Equal(t, "/api2/json/admin/tasks/upid1", rec.path)
}

// TestGateway_GetTaskLog_Success verifies GET /admin/tasks/{upid}/log decodes
// the log entries and forwards the limit query.
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
	assert.Equal(t, "/api2/json/admin/tasks/upid1/log", rec.path)
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
		_, err := g.ListBackupFiles(context.Background(), "s", "vm", "100", "snap", "")
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
