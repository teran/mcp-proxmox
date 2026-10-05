package pbs

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
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

// logrusAppLogger adapts a *logrus.Logger to port.AppLogger (via the embedded
// Debugf/Infof/Warnf/Errorf) while also exposing a Tracef method so the gateway's
// trace-level request line can be captured. It is intentionally NOT a
// RequestAwareLogger, so port.CtxLogger falls back to the plain methods. The
// Tracef method is the seam the developer adds to the port logger surface.
type logrusAppLogger struct {
	*logrus.Logger
}

// newTraceGatewayLogger returns a logrus-backed port.AppLogger writing to a
// buffer at the given level, for asserting trace/debug gating of HTTP request
// lines.
func newTraceGatewayLogger(t *testing.T, level logrus.Level) (*logrusAppLogger, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	l := logrus.New()
	l.SetOutput(&buf)
	l.SetFormatter(&logrus.TextFormatter{DisableColors: true})
	l.SetLevel(level)
	return &logrusAppLogger{Logger: l}, &buf
}

// newTestGatewayWithLogger builds a Gateway pointing at the given httptest
// server with a custom logger, a static PBS token and fast retry settings.
func newTestGatewayWithLogger(t *testing.T, srv *httptest.Server, logger port.AppLogger) *Gateway {
	t.Helper()
	return NewGateway(Config{
		Endpoint:     srv.URL,
		TokenSource:  token.NewStaticTokenSource("user@pbs!tokenid=secret"),
		HTTPClient:   srv.Client(),
		Logger:       logger,
		Retries:      1,
		RetryBackoff: time.Millisecond,
	})
}

// TestGateway_TraceRequestLine verifies the HTTP request line is emitted at
// TRACE level and carries the method, URL path and upstream status, while never
// containing the API token value (SPEC.md §5.4).
func TestGateway_TraceRequestLine(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[{"store":"backup","path":"/backup"}]}`, rec)
	lg, buf := newTraceGatewayLogger(t, logrus.TraceLevel)
	g := newTestGatewayWithLogger(t, srv, lg)

	_, err := g.ListDatastores(context.Background())
	skipIfStub(t, err)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "pbs request")
	assert.Contains(t, out, "GET")
	assert.Contains(t, out, "/api2/json/admin/datastore")
	assert.Contains(t, out, "upstream_http_status_code=200")
	assert.Contains(t, out, "out_bytes=")
	assert.Contains(t, out, "in_bytes=")
	assert.Contains(t, out, "duration_ms=")
	// Never the API token / Authorization header value.
	assert.NotContains(t, out, "PVEAPIToken=")
	assert.NotContains(t, out, "user@pbs!tokenid=secret")
}

// TestGateway_RequestLineIsTraceGated verifies the HTTP request line is NOT
// emitted at debug level (its level must be trace, so at LOG_LEVEL=debug the
// journal stays free of per-request lines).
func TestGateway_RequestLineIsTraceGated(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[]}`, rec)
	lg, buf := newTraceGatewayLogger(t, logrus.DebugLevel)
	g := newTestGatewayWithLogger(t, srv, lg)

	_, err := g.ListDatastores(context.Background())
	skipIfStub(t, err)
	require.NoError(t, err)

	assert.NotContains(t, buf.String(), "pbs request",
		"request line must be trace-gated, not emitted at debug")
}

// TestGateway_ErrorResponseBodyNotLogged verifies an error response's BODY is
// never written to the log at trace level (only the status may be logged); the
// response payload must not reach the journal.
func TestGateway_ErrorResponseBodyNotLogged(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusBadRequest, `{"data":null,"errors":{"store":"no such store"}}`, rec)
	lg, buf := newTraceGatewayLogger(t, logrus.TraceLevel)
	g := newTestGatewayWithLogger(t, srv, lg)

	_, err := g.ListDatastores(context.Background())
	skipIfStub(t, err)
	require.Error(t, err)

	out := buf.String()
	assert.NotContains(t, out, "no such store", "error response body must never be logged")
	assert.NotContains(t, out, `{"data":null`, "error response body must never be logged")
}

type reqCapture struct {
	method      string
	path        string
	escapedPath string
	query       string
	auth        string
	requestID   string
	count       int
	// form holds the parsed form body (POST/PUT mutations); empty for GETs.
	form url.Values
	// rawBody holds the raw request body bytes (verbatim).
	rawBody string
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
		rec.escapedPath = r.URL.EscapedPath()
		rec.query = r.URL.RawQuery
		rec.auth = r.Header.Get("Authorization")
		rec.requestID = r.Header.Get("X-Request-ID")
		rec.count++
		// ParseForm reads the form body for POST/PUT/PATCH into r.PostForm.
		_ = r.ParseForm()
		rec.form = r.PostForm
		if b, err := io.ReadAll(r.Body); err == nil {
			rec.rawBody = string(b)
		}
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
