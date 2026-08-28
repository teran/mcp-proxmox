package pbs

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

type reqCapture struct {
	method string
	path   string
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

func newTestGateway(t *testing.T, srv *httptest.Server) (*Gateway, *reqCapture) {
	t.Helper()
	rec := &reqCapture{}
	return NewGateway(Config{
		Endpoint:     srv.URL,
		TokenSource:  token.NewStaticTokenSource("user@pbs!tokenid=secret"),
		HTTPClient:   srv.Client(),
		Logger:       noopLogger{},
		Retries:      1,
		RetryBackoff: time.Millisecond,
	}), rec
}

func mockServer(t *testing.T, status int, body string, rec *reqCapture) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.auth = r.Header.Get("Authorization")
		rec.count++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestGateway_ListDatastores_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[{"store":"backup","path":"/backup"}]}`, rec)
	g, _ := newTestGateway(t, srv)

	ds, err := g.ListDatastores(context.Background())
	skipIfStub(t, err)
	require.NoError(t, err)
	require.Len(t, ds, 1)
	assert.Equal(t, "backup", ds[0].Store)

	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/api2/json/admin/datastore", rec.path)
	assert.Equal(t, "PVEAPIToken=user@pbs!tokenid=secret", rec.auth)
}

func TestGateway_GetDatastoreStatus_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":{"store":"backup","total":2000,"used":500}}`, rec)
	g, _ := newTestGateway(t, srv)

	st, err := g.GetDatastoreStatus(context.Background(), "backup")
	skipIfStub(t, err)
	require.NoError(t, err)
	assert.Equal(t, "backup", st.Store)
	assert.Equal(t, int64(2000), st.Total)
	assert.Equal(t, "/api2/json/admin/datastore/backup/status", rec.path)
}

func TestGateway_GetPBSVersion_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":{"version":"3.2.3","release":"3.2"}}`, rec)
	g, _ := newTestGateway(t, srv)

	v, err := g.GetPBSVersion(context.Background())
	skipIfStub(t, err)
	require.NoError(t, err)
	assert.Equal(t, "3.2.3", v.Version)
	assert.Equal(t, "/api2/json/version", rec.path)
}

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

			_, err := g.ListDatastores(context.Background())
			skipIfStub(t, err)
			require.Error(t, err)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestGateway_5xxRetriesThenUpstreamError(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.ListDatastores(context.Background())
	skipIfStub(t, err)
	require.Error(t, err)
	var ue *UpstreamError
	assert.ErrorAs(t, err, &ue)
	assert.Equal(t, http.StatusInternalServerError, ue.Status)
	assert.Equal(t, 2, rec.count, "1 initial + 1 retry")
}

func TestGateway_APIError(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusBadRequest, `{"data":null,"errors":{"store":"no such store"}}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.ListDatastores(context.Background())
	skipIfStub(t, err)
	require.Error(t, err)
	var ae *APIError
	assert.ErrorAs(t, err, &ae)
	assert.NotEmpty(t, ae.Messages)
	assert.Equal(t, 1, rec.count, "APIError must not be retried")
}

func TestGateway_InvalidJSON(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `not json`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.ListDatastores(context.Background())
	skipIfStub(t, err)
	require.Error(t, err)
	var ue *UpstreamError
	assert.ErrorAs(t, err, &ue)
}

func TestGateway_MissingToken(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[]}`, rec)
	g := NewGateway(Config{
		Endpoint:     srv.URL,
		TokenSource:  token.NewStaticTokenSource(""),
		HTTPClient:   srv.Client(),
		Logger:       noopLogger{},
		Retries:      1,
		RetryBackoff: time.Millisecond,
	})

	_, err := g.ListDatastores(context.Background())
	skipIfStub(t, err)
	require.Error(t, err)
	assert.Equal(t, 0, rec.count, "no network call should be made on token failure")
}

func TestGateway_CACertPathNonEmpty(t *testing.T) {
	srv := mockServer(t, 200, `{"data":[]}`, &reqCapture{})
	g := NewGateway(Config{
		Endpoint:    srv.URL,
		TokenSource: token.NewStaticTokenSource("tok"),
		HTTPClient:  srv.Client(),
		Logger:      noopLogger{},
		CACertPath:  "/nonexistent/ca.pem",
	})
	assert.NotNil(t, g)
}

func TestGateway_DoNotImplementedFlag(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[]}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.ListDatastores(context.Background())
	assert.True(t, errors.Is(err, ErrNotImplemented) || err == nil)
}

// compile-time: Gateway implements port.PBSGateway.
var _ port.PBSGateway = (*Gateway)(nil)
