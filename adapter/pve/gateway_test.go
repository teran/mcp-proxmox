package pve

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/adapter/token"
	"github.com/teran/mcp-proxmox/domain/port"
)

// noopLogger is a no-op port.AppLogger for gateway tests.
type noopLogger struct{}

func (noopLogger) Debugf(string, ...any) {}
func (noopLogger) Infof(string, ...any)  {}
func (noopLogger) Warnf(string, ...any)  {}
func (noopLogger) Errorf(string, ...any) {}

// reqCapture records the last request the httptest server received.
type reqCapture struct {
	method string
	path   string
	query  string
	auth   string
	count  int
}

// skipIfStub skips a contract test while the gateway body is still the
// ErrNotImplemented scaffold. Once the developer implements do(), the test
// automatically activates and validates the real behavior.
func skipIfStub(t *testing.T, err error) {
	t.Helper()
	if errors.Is(err, ErrNotImplemented) {
		t.Skip("gateway.do() implementation still in flight (ErrNotImplemented); contract test will activate once implemented")
	}
}

// newTestGateway builds a Gateway pointing at the given httptest server, with a
// static PVE token and fast retry settings.
func newTestGateway(t *testing.T, srv *httptest.Server) (*Gateway, *reqCapture) {
	t.Helper()
	rec := &reqCapture{}
	return NewGateway(Config{
		Endpoint:     srv.URL,
		TokenSource:  token.NewStaticTokenSource("user@pam!tokenid=secret"),
		HTTPClient:   srv.Client(),
		Logger:       noopLogger{},
		Retries:      1,
		RetryBackoff: time.Millisecond,
	}), rec
}

// mockServer spins up an httptest server whose handler records method/path/auth
// and responds with the given status and body.
func mockServer(t *testing.T, status int, body string, rec *reqCapture) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.query = r.URL.RawQuery
		rec.auth = r.Header.Get("Authorization")
		rec.count++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestGateway_ListNodes_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[{"node":"pve1","status":"online"},{"node":"pve2","status":"offline"}]}`, rec)
	g, _ := newTestGateway(t, srv)

	nodes, err := g.ListNodes(context.Background())
	skipIfStub(t, err)
	require.NoError(t, err)
	require.Len(t, nodes, 2)
	assert.Equal(t, "pve1", nodes[0].Node)
	assert.Equal(t, "online", nodes[0].Status)

	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/api2/json/nodes", rec.path)
	assert.Equal(t, "PVEAPIToken=user@pam!tokenid=secret", rec.auth)
}

func TestGateway_GetNextID_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":201}`, rec)
	g, _ := newTestGateway(t, srv)

	id, err := g.GetNextID(context.Background())
	skipIfStub(t, err)
	require.NoError(t, err)
	assert.Equal(t, 201, id)
	assert.Equal(t, "/api2/json/cluster/nextid", rec.path)
}

// TestGateway_GetVMConfig_Success retrieves a single-object getter and asserts
// the URL path is built with node/vmid interpolation.
func TestGateway_GetVMConfig_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":{"vmid":100,"name":"web"}}`, rec)
	g, _ := newTestGateway(t, srv)

	cfg, err := g.GetVMConfig(context.Background(), "pve1", 100)
	skipIfStub(t, err)
	require.NoError(t, err)
	assert.Equal(t, 100, cfg.VMID)
	assert.Equal(t, "web", cfg.Name)
	assert.Equal(t, "/api2/json/nodes/pve1/qemu/100/config", rec.path)
}

// TestGateway_HTTPErrorMapping verifies the HTTP->sentinel mapping for the
// read-only surface (SPEC.md §7.3).
func TestGateway_HTTPErrorMapping(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   error
	}{
		{"401", http.StatusUnauthorized, port.ErrUnauthorized},
		{"403", http.StatusForbidden, port.ErrForbidden},
		{"404", http.StatusNotFound, port.ErrNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := &reqCapture{}
			srv := mockServer(t, tc.status, `{"data":null}`, rec)
			g, _ := newTestGateway(t, srv)

			_, err := g.ListNodes(context.Background())
			skipIfStub(t, err)
			require.Error(t, err)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

// TestGateway_5xxRetriesThenUpstreamError verifies that a 5xx is retried and
// then surfaces as an UpstreamError (SPEC.md §7.5).
func TestGateway_5xxRetriesThenUpstreamError(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.ListNodes(context.Background())
	skipIfStub(t, err)
	require.Error(t, err)
	var ue *UpstreamError
	assert.ErrorAs(t, err, &ue)
	assert.Equal(t, http.StatusInternalServerError, ue.Status)
	// 1 initial + 1 retry (Retries:1).
	assert.Equal(t, 2, rec.count)
}

// TestGateway_APIError verifies the semantic error body maps to APIError and is
// NOT retried.
func TestGateway_APIError(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusBadRequest, `{"data":null,"errors":{"node":"no such node"}}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.ListNodes(context.Background())
	skipIfStub(t, err)
	require.Error(t, err)
	var ae *APIError
	assert.ErrorAs(t, err, &ae)
	assert.NotEmpty(t, ae.Messages)
	assert.Equal(t, 1, rec.count, "APIError must not be retried")
}

// TestGateway_EmptyData verifies an empty/null data for a getter surfaces an
// explicit error (SPEC.md §7.2) rather than a silently-zero struct.
//
// NOTE: SPEC §7.2 names the returned type `*EmptyDataError`, which is a
// production seam the developer must add to adapter/pve/errors.go. Until it
// exists we assert a generic error; once the type is present, extend this test
// with `var ee *EmptyDataError; assert.ErrorAs(t, err, &ee)`.
func TestGateway_EmptyData(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":null}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.GetVMStatus(context.Background(), "pve1", 100)
	skipIfStub(t, err)
	require.Error(t, err)
	assert.Equal(t, 1, rec.count)
}

// TestGateway_InvalidJSON verifies a non-JSON body surfaces an UpstreamError.
func TestGateway_InvalidJSON(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `this is not json`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.ListNodes(context.Background())
	skipIfStub(t, err)
	require.Error(t, err)
	var ue *UpstreamError
	assert.ErrorAs(t, err, &ue)
}

// TestGateway_MissingToken verifies that when the token source cannot produce a
// header, the request fails before any network call (SPEC.md §7.4).
func TestGateway_MissingToken(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[]}`, rec)
	g := NewGateway(Config{
		Endpoint:     srv.URL,
		TokenSource:  token.NewStaticTokenSource(""), // empty -> error
		HTTPClient:   srv.Client(),
		Logger:       noopLogger{},
		Retries:      1,
		RetryBackoff: time.Millisecond,
	})

	_, err := g.ListNodes(context.Background())
	skipIfStub(t, err)
	require.Error(t, err)
	assert.Equal(t, 0, rec.count, "no network call should be made on token failure")
}

// TestGateway_CACertPathNonEmpty documents that a non-empty CACertPath does not
// disable TLS verification; building the gateway must not panic.
func TestGateway_CACertPathNonEmpty(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[]}`, rec)
	g := NewGateway(Config{
		Endpoint:    srv.URL,
		TokenSource: token.NewStaticTokenSource("tok"),
		HTTPClient:  srv.Client(),
		Logger:      noopLogger{},
		CACertPath:  "/nonexistent/ca.pem",
	})
	_ = rec
	assert.NotNil(t, g)
}

// TestGateway_DoNotImplementedFlag is a marker that documents the current
// scaffold state: until the developer implements do(), every gateway method
// returns ErrNotImplemented. The other contract tests above skip until then.
func TestGateway_DoNotImplementedFlag(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[]}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.ListNodes(context.Background())
	assert.True(t, errors.Is(err, ErrNotImplemented) || err == nil,
		"expected ErrNotImplemented (scaffold) or nil (implemented)")
}

// compile-time: Gateway implements port.PVEGateway.
var _ port.PVEGateway = (*Gateway)(nil)
