package pve

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/port"
)

// TestGateway_VMLifecycle_Success verifies each QEMU VM lifecycle mutation
// builds the right method + URL path and encodes its form/query parameters, and
// that the returned Task wraps the UPID decoded from `data`.
func TestGateway_VMLifecycle_Success(t *testing.T) {
	tests := []struct {
		name string
		call func(*Gateway, *reqCapture) error
		// optional assertions on the captured request
		method string
		path   string
		query  string
		check  func(*reqCapture)
	}{
		{
			name:   "StartVM",
			method: http.MethodPost,
			path:   "/api2/json/nodes/pve1/qemu/100/status/start",
			call: func(g *Gateway, _ *reqCapture) error {
				task, err := g.StartVM(context.Background(), "pve1", 100)
				if assert.NoError(t, err) {
					require.NotNil(t, task)
					assert.Equal(t, "UPID:pve1:start:...", task.UPID)
				}
				return err
			},
		},
		{
			name:   "StopVM with skiplock",
			method: http.MethodPost,
			path:   "/api2/json/nodes/pve1/qemu/100/status/stop",
			check:  func(rec *reqCapture) { assert.Equal(t, "1", rec.form.Get("skiplock")) },
			call: func(g *Gateway, _ *reqCapture) error {
				_, err := g.StopVM(context.Background(), "pve1", 100, true)
				return err
			},
		},
		{
			name:   "ShutdownVM",
			method: http.MethodPost,
			path:   "/api2/json/nodes/pve1/qemu/100/status/shutdown",
			check: func(rec *reqCapture) {
				assert.Equal(t, "1", rec.form.Get("forceStop"))
				assert.Equal(t, "30", rec.form.Get("timeout"))
			},
			call: func(g *Gateway, _ *reqCapture) error {
				_, err := g.ShutdownVM(context.Background(), "pve1", 100, true, 30)
				return err
			},
		},
		{
			name:   "RebootVM",
			method: http.MethodPost,
			path:   "/api2/json/nodes/pve1/qemu/100/status/reboot",
			check:  func(rec *reqCapture) { assert.Equal(t, "60", rec.form.Get("timeout")) },
			call: func(g *Gateway, _ *reqCapture) error {
				_, err := g.RebootVM(context.Background(), "pve1", 100, 60)
				return err
			},
		},
		{
			name:   "ResetVM",
			method: http.MethodPost,
			path:   "/api2/json/nodes/pve1/qemu/100/status/reset",
			call: func(g *Gateway, _ *reqCapture) error {
				_, err := g.ResetVM(context.Background(), "pve1", 100)
				return err
			},
		},
		{
			name:   "SuspendVM with todisk",
			method: http.MethodPost,
			path:   "/api2/json/nodes/pve1/qemu/100/status/suspend",
			check:  func(rec *reqCapture) { assert.Equal(t, "1", rec.form.Get("todisk")) },
			call: func(g *Gateway, _ *reqCapture) error {
				_, err := g.SuspendVM(context.Background(), "pve1", 100, true)
				return err
			},
		},
		{
			name:   "ResumeVM",
			method: http.MethodPost,
			path:   "/api2/json/nodes/pve1/qemu/100/status/resume",
			call: func(g *Gateway, _ *reqCapture) error {
				_, err := g.ResumeVM(context.Background(), "pve1", 100)
				return err
			},
		},
		{
			name:   "DeleteVM with query params",
			method: http.MethodDelete,
			path:   "/api2/json/nodes/pve1/qemu/100",
			query:  "destroy-unreferenced-disks=1&purge=1",
			call: func(g *Gateway, _ *reqCapture) error {
				_, err := g.DeleteVM(context.Background(), "pve1", 100, true, true)
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &reqCapture{}
			srv := mockServer(t, 200, `{"data":"UPID:pve1:start:..."}`, rec)
			g, _ := newTestGateway(t, srv)

			err := tt.call(g, rec)
			require.NoError(t, err)

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

// TestGateway_VMLifecycle_ErrorNoRetry verifies VM lifecycle mutations surface
// errors in the sentinel/upstream families and are never retried.
func TestGateway_VMLifecycle_ErrorNoRetry(t *testing.T) {
	t.Run("start unauthorized", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusUnauthorized, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.StartVM(context.Background(), "pve1", 100)
		require.Error(t, err)
		assert.ErrorIs(t, err, port.ErrUnauthorized)
		assert.Equal(t, 1, rec.count)
	})

	t.Run("shutdown upstream error no retry", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.ShutdownVM(context.Background(), "pve1", 100, false, 0)
		require.Error(t, err)
		var ue *UpstreamError
		assert.ErrorAs(t, err, &ue)
		assert.Equal(t, 1, rec.count, "mutations must not be retried")
	})

	t.Run("delete upstream error no retry", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.DeleteVM(context.Background(), "pve1", 100, false, false)
		require.Error(t, err)
		assert.Equal(t, 1, rec.count)
	})
}
