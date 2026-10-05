package pve

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/port"
)

// TestGateway_LXCLifecycle_Success verifies each LXC lifecycle mutation builds
// the right method + URL path and encodes its form/query parameters.
func TestGateway_LXCLifecycle_Success(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		query  string
		check  func(*reqCapture)
		call   func(*Gateway) error
	}{
		{
			name:   "StartLXC",
			method: http.MethodPost,
			path:   "/api2/json/nodes/pve1/lxc/200/status/start",
			call: func(g *Gateway) error {
				_, err := g.StartLXC(context.Background(), "pve1", 200)
				return err
			},
		},
		{
			name:   "StopLXC with flags",
			method: http.MethodPost,
			path:   "/api2/json/nodes/pve1/lxc/200/status/stop",
			check: func(rec *reqCapture) {
				assert.Equal(t, "1", rec.form.Get("skiplock"))
				assert.Equal(t, "1", rec.form.Get("forceStop"))
			},
			call: func(g *Gateway) error {
				_, err := g.StopLXC(context.Background(), "pve1", 200, true, true)
				return err
			},
		},
		{
			name:   "ShutdownLXC",
			method: http.MethodPost,
			path:   "/api2/json/nodes/pve1/lxc/200/status/shutdown",
			check: func(rec *reqCapture) {
				assert.Equal(t, "1", rec.form.Get("forceStop"))
				assert.Equal(t, "45", rec.form.Get("timeout"))
			},
			call: func(g *Gateway) error {
				_, err := g.ShutdownLXC(context.Background(), "pve1", 200, true, 45)
				return err
			},
		},
		{
			name:   "RebootLXC",
			method: http.MethodPost,
			path:   "/api2/json/nodes/pve1/lxc/200/status/reboot",
			check:  func(rec *reqCapture) { assert.Equal(t, "30", rec.form.Get("timeout")) },
			call: func(g *Gateway) error {
				_, err := g.RebootLXC(context.Background(), "pve1", 200, 30)
				return err
			},
		},
		{
			name:   "DeleteLXC with query params",
			method: http.MethodDelete,
			path:   "/api2/json/nodes/pve1/lxc/200",
			query:  "destroy-unreferenced-disks=1&force=1&purge=1",
			call: func(g *Gateway) error {
				_, err := g.DeleteLXC(context.Background(), "pve1", 200, true, true, true)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &reqCapture{}
			srv := mockServer(t, 200, `{"data":"UPID:pve1:lxc:..."}`, rec)
			g, _ := newTestGateway(t, srv)

			require.NoError(t, tt.call(g))

			assert.Equal(t, tt.method, rec.method)
			assert.Equal(t, tt.path, rec.path)
			assert.Equal(t, "PVEAPIToken=user@pam!tokenid=secret", rec.auth)
			if tt.query != "" {
				assert.Equal(t, tt.query, rec.query)
			}
			if tt.check != nil {
				tt.check(rec)
			}
		})
	}
}

// TestGateway_LXCLifecycle_ErrorNoRetry verifies LXC lifecycle mutations surface
// errors and are never retried.
func TestGateway_LXCLifecycle_ErrorNoRetry(t *testing.T) {
	t.Run("start unauthorized", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusUnauthorized, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.StartLXC(context.Background(), "pve1", 200)
		require.Error(t, err)
		assert.ErrorIs(t, err, port.ErrUnauthorized)
		assert.Equal(t, 1, rec.count)
	})

	t.Run("stop upstream error no retry", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.StopLXC(context.Background(), "pve1", 200, false, false)
		require.Error(t, err)
		var ue *UpstreamError
		assert.ErrorAs(t, err, &ue)
		assert.Equal(t, 1, rec.count)
	})

	t.Run("delete upstream error no retry", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.DeleteLXC(context.Background(), "pve1", 200, false, false, false)
		require.Error(t, err)
		assert.Equal(t, 1, rec.count)
	})
}
