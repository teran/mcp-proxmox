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

// TestGateway_StartVerify_Success verifies POST /admin/datastore/{store}/verify
// returns the UPID decoded from `data`.
func TestGateway_StartVerify_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":"UPID:pbs:0000verify:..."}`, rec)
	g, _ := newTestGateway(t, srv)

	upid, err := g.StartVerify(context.Background(), "backup")
	require.NoError(t, err)
	assert.Equal(t, "UPID:pbs:0000verify:...", upid)

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api2/json/admin/datastore/backup/verify", rec.path)
	assert.Equal(t, "PVEAPIToken=user@pbs!tokenid=secret", rec.auth)
}

// TestGateway_StartGC_Success verifies POST /admin/datastore/{store}/gc.
func TestGateway_StartGC_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":"UPID:pbs:0000gc:..."}`, rec)
	g, _ := newTestGateway(t, srv)

	upid, err := g.StartGC(context.Background(), "backup")
	require.NoError(t, err)
	assert.Equal(t, "UPID:pbs:0000gc:...", upid)

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api2/json/admin/datastore/backup/gc", rec.path)
}

// TestGateway_StartPrune_Success verifies POST /admin/datastore/{store}/prune.
func TestGateway_StartPrune_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":"UPID:pbs:0000prune:..."}`, rec)
	g, _ := newTestGateway(t, srv)

	upid, err := g.StartPrune(context.Background(), "backup")
	require.NoError(t, err)
	assert.Equal(t, "UPID:pbs:0000prune:...", upid)

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api2/json/admin/datastore/backup/prune", rec.path)
}

// TestGateway_StartSync_Success verifies POST /admin/datastore/{store}/sync with
// the sync parameters as a form body.
func TestGateway_StartSync_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":"UPID:pbs:0000sync:..."}`, rec)
	g, _ := newTestGateway(t, srv)

	upid, err := g.StartSync(context.Background(), "backup", model.PBSSyncRequest{
		Remote: "remote1", RemoteStore: "backup", Owner: "root@pam",
		MaxFiles: 3, RemoveVanished: true, RateIn: 1000, RateOut: 2000,
		SkipLost: true, NotifyUser: "admin@example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, "UPID:pbs:0000sync:...", upid)

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api2/json/admin/datastore/backup/sync", rec.path)
	assert.Equal(t, "remote1", rec.form.Get("remote"))
	assert.Equal(t, "backup", rec.form.Get("remote-store"))
	assert.Equal(t, "root@pam", rec.form.Get("owner"))
	assert.Equal(t, "3", rec.form.Get("maxfiles"))
	assert.Equal(t, "1", rec.form.Get("remove-vanished"))
	assert.Equal(t, "1000", rec.form.Get("ratein"))
	assert.Equal(t, "2000", rec.form.Get("rateout"))
	assert.Equal(t, "1", rec.form.Get("skip-lost"))
	assert.Equal(t, "admin@example.com", rec.form.Get("notify-user"))
}

// TestGateway_PBSMutationErrorMapping verifies mutation failures map through the
// same error family as reads and are never retried.
func TestGateway_PBSMutationErrorMapping(t *testing.T) {
	t.Run("verify unauthorized", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusUnauthorized, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.StartVerify(context.Background(), "backup")
		require.Error(t, err)
		assert.ErrorIs(t, err, port.ErrUnauthorized)
		assert.Equal(t, 1, rec.count)
	})

	t.Run("gc upstream error no retry", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.StartGC(context.Background(), "backup")
		require.Error(t, err)
		var ue *UpstreamError
		assert.ErrorAs(t, err, &ue)
		assert.Equal(t, 1, rec.count, "mutations must not be retried")
	})

	t.Run("prune upstream error no retry", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.StartPrune(context.Background(), "backup")
		require.Error(t, err)
		assert.Equal(t, 1, rec.count)
	})

	t.Run("sync upstream error no retry", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.StartSync(context.Background(), "backup", model.PBSSyncRequest{Remote: "r"})
		require.Error(t, err)
		assert.Equal(t, 1, rec.count)
	})
}
