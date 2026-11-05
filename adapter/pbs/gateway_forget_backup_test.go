package pbs

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/port"
)

// These tests cover pbs_backup_forget's gateway method ForgetBackup, the
// cleanup counterpart to StartVerify/StartGC etc. It issues a single DELETE
// /admin/datastore/{store}/snapshots/{type}/{id}/{time} targeting ONE snapshot
// within a group. Each path segment (store/type/id) is url.PathEscape'd (S11);
// backup_time is the plain epoch.

// TestGateway_ForgetBackup_Success verifies DELETE
// /admin/datastore/{store}/snapshots/{type}/{id}/{time} removes a single backup
// snapshot. backup_time is the plain epoch integer; the mutation is never
// retried.
func TestGateway_ForgetBackup_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{}`, rec)
	g, _ := newTestGateway(t, srv)

	const epoch = int64(1700000000)
	err := g.ForgetBackup(context.Background(), "backup", "vm", "141", epoch)
	require.NoError(t, err)

	assert.Equal(t, http.MethodDelete, rec.method)
	assert.Equal(t,
		"/api2/json/admin/datastore/backup/snapshots/vm/141/"+strconv.FormatInt(epoch, 10),
		rec.path)
	assert.Equal(t, "PVEAPIToken=user@pbs!tokenid=secret", rec.auth)
	assert.Equal(t, 1, rec.count, "mutations must not be retried")
}

// TestGateway_ForgetBackup_MultiSnapshotGroup pins that ForgetBackup is a
// per-snapshot DELETE: each epoch in a group (a single snapshot vs a second
// snapshot of the same VM) is a distinct, independent request — the gateway is
// agnostic to how many snapshots share the group.
func TestGateway_ForgetBackup_MultiSnapshotGroup(t *testing.T) {
	for _, epoch := range []int64{1700000000, 1700000100} {
		rec := &reqCapture{}
		srv := mockServer(t, 200, `{}`, rec)
		g, _ := newTestGateway(t, srv)

		err := g.ForgetBackup(context.Background(), "backup", "vm", "141", epoch)
		require.NoError(t, err)
		assert.Equal(t,
			"/api2/json/admin/datastore/backup/snapshots/vm/141/"+strconv.FormatInt(epoch, 10),
			rec.path)
	}
}

// TestGateway_ForgetBackup_UnknownTime404 verifies a 404 for an unknown
// snapshot/time surfaces as the not-found sentinel and is not retried.
func TestGateway_ForgetBackup_UnknownTime404(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusNotFound, `{"data":null}`, rec)
	g, _ := newTestGateway(t, srv)

	err := g.ForgetBackup(context.Background(), "backup", "vm", "141", 1)
	require.Error(t, err)
	assert.ErrorIs(t, err, port.ErrNotFound)
	assert.Equal(t, 1, rec.count, "mutations must not be retried")
}

// TestGateway_ForgetBackup_EscapesPathSegments pins S11 path-injection safety:
// store/type/id containing '/' must arrive as single escaped segments (%2F),
// never as extra literal path segments.
func TestGateway_ForgetBackup_EscapesPathSegments(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{}`, rec)
	g, _ := newTestGateway(t, srv)

	err := g.ForgetBackup(context.Background(), "st/ore", "v/m", "14/1", 5)
	require.NoError(t, err)

	assert.Equal(t, http.MethodDelete, rec.method)
	assert.Equal(t, "/api2/json/admin/datastore/st/ore/snapshots/v/m/14/1/5", rec.path)
	assert.Equal(t, "/api2/json/admin/datastore/st%2Fore/snapshots/v%2Fm/14%2F1/5", rec.escapedPath,
		"store/type/id must be URL-escaped so '/' cannot inject extra path segments")
}
