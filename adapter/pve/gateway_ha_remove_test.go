package pve

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/port"
)

// These tests cover pve_ha_remove's gateway method RemoveHAResource, the
// cleanup counterpart to AddHAResource. It issues a single DELETE
// /cluster/ha/resources/{sid} with the SID as one path segment. Per Go's
// url.PathEscape, the ':' in a SID (e.g. vm:100) is left literal while any '/'
// is escaped to %2F (S11 path-injection safety).

// TestGateway_RemoveHAResource_Success verifies DELETE /cluster/ha/resources/{sid}
// removes a HA resource. The SID's ':' stays literal (url.PathEscape does not
// escape ':'); auth header is attached and the mutation is never retried.
func TestGateway_RemoveHAResource_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{}`, rec)
	g, _ := newTestGateway(t, srv)

	err := g.RemoveHAResource(context.Background(), "vm:141")
	require.NoError(t, err)

	assert.Equal(t, http.MethodDelete, rec.method)
	assert.Equal(t, "/api2/json/cluster/ha/resources/vm:141", rec.path)
	assert.Equal(t, "PVEAPIToken=user@pam!tokenid=secret", rec.auth)
	assert.Equal(t, 1, rec.count, "mutations must not be retried")
}

// TestGateway_RemoveHAResource_EscapesSlash pins S11 path-injection safety for
// a SID that contains a '/': it must arrive as a single escaped segment (%2F),
// never as extra literal path segments. The ':' remains literal.
func TestGateway_RemoveHAResource_EscapesSlash(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{}`, rec)
	g, _ := newTestGateway(t, srv)

	err := g.RemoveHAResource(context.Background(), "ct:200/foo")
	require.NoError(t, err)

	assert.Equal(t, http.MethodDelete, rec.method)
	assert.Equal(t, "/api2/json/cluster/ha/resources/ct:200/foo", rec.path)
	assert.Equal(t, "/api2/json/cluster/ha/resources/ct:200%2Ffoo", rec.escapedPath,
		"a '/' in the sid must be URL-escaped so it cannot inject extra path segments")
}

// TestGateway_RemoveHAResource_NotFound verifies a 404 for an unknown SID
// surfaces as the not-found sentinel and is not retried.
func TestGateway_RemoveHAResource_NotFound(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusNotFound, `{"data":null}`, rec)
	g, _ := newTestGateway(t, srv)

	err := g.RemoveHAResource(context.Background(), "vm:999")
	require.Error(t, err)
	assert.ErrorIs(t, err, port.ErrNotFound)
	assert.Equal(t, 1, rec.count, "mutations must not be retried")
}
