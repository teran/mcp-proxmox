package pbs

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/port"
)

// TestGateway_GetDatastoreConfig_Success verifies
// GET /config/datastore/{store} decodes a DatastoreConfig, forwarding the
// Authorization header and using a GET on the escaped store path.
func TestGateway_GetDatastoreConfig_Success(t *testing.T) {
	rec := &reqCapture{}
	body := `{"data":{"path":"/backup","backend":"filesystem","backing-device":"/dev/sdb","keep-daily":7,"verify-new":true,"gc-schedule":"sun","notify-user":"root@pam","maintenance-mode":"off","tuning":"example"}}`
	srv := mockServer(t, 200, body, rec)
	g, _ := newTestGateway(t, srv)

	cfg, err := g.GetDatastoreConfig(context.Background(), "backup")
	require.NoError(t, err)
	require.NotNil(t, cfg)
	assert.Equal(t, "/backup", cfg.Path)
	assert.Equal(t, "filesystem", cfg.Backend)
	assert.Equal(t, "/dev/sdb", cfg.BackingDevice)
	assert.Equal(t, 7, cfg.KeepDaily)
	assert.True(t, cfg.VerifyNew)
	assert.Equal(t, "sun", cfg.GCSchedule)
	assert.Equal(t, "root@pam", cfg.NotifyUser)
	assert.Equal(t, "off", cfg.MaintenanceMode)
	assert.Equal(t, "example", cfg.Tuning)

	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/api2/json/config/datastore/backup", rec.path)
	assert.Equal(t, "PVEAPIToken=user@pbs!tokenid=secret", rec.auth)
}

// TestGateway_GetDatastoreConfig_PathEscapesStore verifies a store name
// containing a reserved path character (e.g. '/') is percent-escaped so it
// cannot inject extra path segments (S11 path-injection fix).
func TestGateway_GetDatastoreConfig_PathEscapesStore(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":{"path":"/x"}}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.GetDatastoreConfig(context.Background(), "a/b")
	require.NoError(t, err)
	// EscapedPath reflects the percent-encoded form actually sent on the wire
	// (Go's http server decodes %2F in r.URL.Path), proving the store value
	// cannot inject extra path segments.
	assert.Equal(t, "/api2/json/config/datastore/a%2Fb", rec.escapedPath)
}

// TestGateway_GetDatastoreConfig_ErrorBranches covers 401/500/data-null for the
// new getter.
func TestGateway_GetDatastoreConfig_ErrorBranches(t *testing.T) {
	t.Run("unauthorized maps to ErrUnauthorized", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusUnauthorized, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)

		_, err := g.GetDatastoreConfig(context.Background(), "backup")
		require.Error(t, err)
		assert.ErrorIs(t, err, port.ErrUnauthorized)
		assert.Equal(t, 1, rec.count, "sentinels are not retried")
	})

	t.Run("500 maps to UpstreamError and is retried", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)

		_, err := g.GetDatastoreConfig(context.Background(), "backup")
		require.Error(t, err)
		var ue *UpstreamError
		assert.ErrorAs(t, err, &ue)
		assert.Equal(t, http.StatusInternalServerError, ue.Status)
		assert.Equal(t, 2, rec.count, "1 initial + 1 retry")
	})

	t.Run("null data maps to EmptyDataError", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, 200, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)

		_, err := g.GetDatastoreConfig(context.Background(), "backup")
		require.Error(t, err)
		var ede *EmptyDataError
		assert.ErrorAs(t, err, &ede)
		assert.Equal(t, 1, rec.count, "EmptyDataError is not retried")
	})
}

// TestGateway_Decode_EmptyBody verifies that a 2xx response with an empty body
// surfaces an explicit UpstreamError naming the empty response body rather than
// the raw "unexpected end of JSON input".
func TestGateway_Decode_EmptyBody(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusOK, "", rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.GetDatastoreConfig(context.Background(), "backup")
	require.Error(t, err)
	var ue *UpstreamError
	assert.ErrorAs(t, err, &ue)
	assert.Equal(t, "decode", ue.Op)
	assert.Contains(t, ue.Error(), "empty response body")
	// decode errors are UpstreamError and thus retried once.
	assert.Equal(t, 2, rec.count, "decode UpstreamError is retried")
}

// TestGateway_Decode_EmptyBody_Direct exercises decode() in isolation so the
// empty-body diagnostic is covered without depending on the retry loop.
func TestGateway_Decode_EmptyBody_Direct(t *testing.T) {
	g := &Gateway{}
	err := g.decode([]byte(""), &struct{}{})
	require.Error(t, err)
	var ue *UpstreamError
	assert.ErrorAs(t, err, &ue)
	assert.True(t, strings.Contains(ue.Error(), "empty response body"), "got: %v", ue.Error())
}
