package pbs

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/adapter/token"
	"github.com/teran/mcp-proxmox/domain/model"
	"github.com/teran/mcp-proxmox/domain/port"
)

// runMethod executes fn against a gateway backed by a mock server returning the
// given JSON body, and returns the captured request.
func runMethod(t *testing.T, body string, fn func(*Gateway) error) (*reqCapture, error) {
	t.Helper()
	rec := &reqCapture{}
	srv := mockServer(t, 200, body, rec)
	g, _ := newTestGateway(t, srv)
	err := fn(g)
	return rec, err
}

// TestGateway_AllMethodSuccess exercises every PBS read-only method's happy
// path, asserting the returned struct fields and the request method/path.
func TestGateway_AllMethodSuccess(t *testing.T) {
	tests := []struct {
		name  string
		body  string
		path  string
		query string
		call  func(*Gateway) error
	}{
		{
			name: "ListDatastores",
			body: `{"data":[{"store":"backup","backend-type":"filesystem","mount-status":"mounted","comment":"primary","maintenance":"none"}]}`,
			path: "/api2/json/admin/datastore",
			call: func(g *Gateway) error {
				ds, err := g.ListDatastores(context.Background())
				if assert.NoError(t, err) {
					require.Len(t, ds, 1)
					assert.Equal(t, "backup", ds[0].Store)
					assert.Equal(t, "filesystem", ds[0].BackendType)
					assert.Equal(t, "mounted", ds[0].MountStatus)
					assert.Equal(t, "primary", ds[0].Comment)
					assert.Equal(t, "none", ds[0].Maintenance)
				}
				return err
			},
		},
		{
			name: "GetDatastoreConfig",
			body: `{"data":{"path":"/backup","keep-daily":7,"verify-new":true,"gc-schedule":"sun","notify-user":"root@pam","tuning":"example"}}`,
			path: "/api2/json/config/datastore/backup",
			call: func(g *Gateway) error {
				cfg, err := g.GetDatastoreConfig(context.Background(), "backup")
				if assert.NoError(t, err) {
					assert.Equal(t, "/backup", cfg.Path)
					assert.Equal(t, 7, cfg.KeepDaily)
					assert.True(t, cfg.VerifyNew)
					assert.Equal(t, "sun", cfg.GCSchedule)
					assert.Equal(t, "root@pam", cfg.NotifyUser)
					assert.Equal(t, "example", cfg.Tuning)
				}
				return err
			},
		},
		{
			name: "GetDatastoreStatus",
			body: `{"data":{"store":"backup","total":2000,"used":500,"used_fraction":0.25}}`,
			path: "/api2/json/admin/datastore/backup/status",
			call: func(g *Gateway) error {
				st, err := g.GetDatastoreStatus(context.Background(), "backup")
				if assert.NoError(t, err) {
					assert.Equal(t, "backup", st.Store)
					assert.Equal(t, int64(2000), st.Total)
					assert.Equal(t, int64(500), st.Used)
				}
				return err
			},
		},
		{
			name: "ListBackups",
			body: `{"data":[{"backup-id":"vm/100/2024","backup-time":1704067200,"backup-type":"vm"}]}`,
			path: "/api2/json/admin/datastore/backup/snapshots",
			call: func(g *Gateway) error {
				bs, err := g.ListBackups(context.Background(), "backup")
				if assert.NoError(t, err) {
					require.Len(t, bs, 1)
					assert.Equal(t, "vm/100/2024", bs[0].BackupID)
					assert.Equal(t, "vm", bs[0].BackupType)
				}
				return err
			},
		},
		{
			name: "GetBackup",
			body: `{"data":[{"backup-id":"vm/100/2024","backup-time":1704067200,"backup-type":"vm","owner":"root@pam","size":1024},{"backup-id":"ct/200/2024","backup-time":1704067300,"backup-type":"ct","size":2048}]}`,
			path: "/api2/json/admin/datastore/backup/snapshots",
			call: func(g *Gateway) error {
				bs, err := g.GetBackup(context.Background(), "backup", "vm/100/2024")
				if assert.NoError(t, err) {
					require.Len(t, bs, 1)
					assert.Equal(t, "vm/100/2024", bs[0].BackupID)
					assert.Equal(t, "vm", bs[0].BackupType)
				}
				return err
			},
		},
		{
			name:  "GetBackupNotes",
			body:  `{"data":"keep forever"}`,
			path:  "/api2/json/admin/datastore/backup/group-notes",
			query: "backup-id=vm%2F100%2F2024&backup-type=vm",
			call: func(g *Gateway) error {
				n, err := g.GetBackupNotes(context.Background(), "backup", "vm/100/2024", "vm")
				if assert.NoError(t, err) {
					assert.Equal(t, "keep forever", n.Notes)
				}
				return err
			},
		},
		{
			name: "GetVerifyStatus",
			body: `{"data":{"upid":"upid1","store":"backup","status":"ok","errors":0}}`,
			path: "/api2/json/admin/datastore/backup/verify/upid1",
			call: func(g *Gateway) error {
				st, err := g.GetVerifyStatus(context.Background(), "backup", "upid1")
				if assert.NoError(t, err) {
					assert.Equal(t, "ok", st.Status)
				}
				return err
			},
		},
		{
			name: "GetPruneStatus",
			body: `{"data":{"upid":"upid1","store":"backup","status":"running","duration":5}}`,
			path: "/api2/json/admin/datastore/backup/prune/upid1",
			call: func(g *Gateway) error {
				st, err := g.GetPruneStatus(context.Background(), "backup", "upid1")
				if assert.NoError(t, err) {
					assert.Equal(t, "running", st.Status)
				}
				return err
			},
		},
		{
			name: "GetPBSVersion",
			body: `{"data":{"version":"3.2.3","release":"3.2","repoid":"abc"}}`,
			path: "/api2/json/version",
			call: func(g *Gateway) error {
				v, err := g.GetPBSVersion(context.Background())
				if assert.NoError(t, err) {
					assert.Equal(t, "3.2.3", v.Version)
					assert.Equal(t, "3.2", v.Release)
				}
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec, err := runMethod(t, tt.body, tt.call)
			require.NoError(t, err)
			assert.Equal(t, http.MethodGet, rec.method)
			assert.Equal(t, tt.path, rec.path)
			assert.Equal(t, tt.query, rec.query)
			assert.Equal(t, "PVEAPIToken=user@pbs!tokenid=secret", rec.auth)
		})
	}
}

// TestGateway_GetBackupFiltersByBackupID verifies GetBackup lists /snapshots and
// filters by backup-id, returning a non-nil empty slice (no error) when nothing
// matches (SPEC: PBS has no single-snapshot GET).
func TestGateway_GetBackupFiltersByBackupID(t *testing.T) {
	rec := &reqCapture{}
	body := `{"data":[{"backup-id":"vm/100/2024","backup-type":"vm"},{"backup-id":"ct/200/2024","backup-type":"ct"},{"backup-id":"vm/100/2024","backup-type":"vm"}]}`
	srv := mockServer(t, 200, body, rec)
	g, _ := newTestGateway(t, srv)

	bs, err := g.GetBackup(context.Background(), "backup", "vm/100/2024")
	require.NoError(t, err)
	require.Len(t, bs, 2, "only the two vm/100 snapshots should match")
	for _, b := range bs {
		assert.Equal(t, "vm/100/2024", b.BackupID)
	}
	assert.Equal(t, "/api2/json/admin/datastore/backup/snapshots", rec.path)

	// No match -> empty (non-nil) slice, no error.
	none, err := g.GetBackup(context.Background(), "backup", "does-not-exist")
	require.NoError(t, err)
	assert.Empty(t, none)
	assert.NotNil(t, none, "should return a non-nil empty slice")
}

// TestGateway_ForwardRequestIDHeader verifies that when a per-request ID is
// present in the context (via port.WithRequestID), the outbound upstream request
// carries it as the X-Request-ID header, and that the header is absent when no
// request ID is set in the context (SPEC.md §5.5 / L9 / G11).
func TestGateway_ForwardRequestIDHeader(t *testing.T) {
	tests := []struct {
		name          string
		withRequestID bool
		want          string
	}{
		{
			name:          "request id present is forwarded",
			withRequestID: true,
			want:          "req-abc-123",
		},
		{
			name: "no request id means header is absent",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := &reqCapture{}
			srv := mockServer(t, 200, `{"data":{"store":"backup","path":"/backup"}}`, rec)
			g, _ := newTestGateway(t, srv)

			ctx := context.Background()
			if tt.withRequestID {
				ctx = port.WithRequestID(ctx, tt.want)
			}

			_, err := g.GetDatastoreStatus(ctx, "backup")
			require.NoError(t, err)

			assert.Equal(t, tt.want, rec.requestID)
		})
	}
}

// TestGateway_EmptyDataForGetters verifies null data surfaces EmptyDataError for
// single-entity getters.
func TestGateway_EmptyDataForGetters(t *testing.T) {
	getters := []struct {
		name string
		call func(*Gateway) error
	}{
		{"GetDatastoreStatus", func(g *Gateway) error { _, e := g.GetDatastoreStatus(context.Background(), "backup"); return e }},
		{"GetBackupNotes", func(g *Gateway) error {
			_, e := g.GetBackupNotes(context.Background(), "backup", "vm/100/2024", "vm")
			return e
		}},
		{"GetVerifyStatus", func(g *Gateway) error { _, e := g.GetVerifyStatus(context.Background(), "backup", "upid1"); return e }},
		{"GetPruneStatus", func(g *Gateway) error { _, e := g.GetPruneStatus(context.Background(), "backup", "upid1"); return e }},
		{"GetPBSVersion", func(g *Gateway) error { _, e := g.GetPBSVersion(context.Background()); return e }},
	}

	for _, tt := range getters {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runMethod(t, `{"data":null}`, tt.call)
			require.Error(t, err)
			var ee *EmptyDataError
			assert.ErrorAs(t, err, &ee)
			assert.Contains(t, ee.Error(), "empty data")
		})
	}
}

// TestGateway_EmptyDataForLists verifies null data for a list getter yields an
// empty (non-error) result.
func TestGateway_EmptyDataForLists(t *testing.T) {
	lists := []struct {
		name string
		call func(*Gateway) error
	}{
		{"ListDatastores", func(g *Gateway) error { _, e := g.ListDatastores(context.Background()); return e }},
		{"ListBackups", func(g *Gateway) error { _, e := g.ListBackups(context.Background(), "backup"); return e }},
		{"GetBackup", func(g *Gateway) error { _, e := g.GetBackup(context.Background(), "backup", "vm/100/2024"); return e }},
	}

	for _, tt := range lists {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runMethod(t, `{"data":null}`, tt.call)
			require.NoError(t, err)
		})
	}
}

// TestGateway_AllMethodErrorBranches exercises the error return path of every
// method to reach full statement coverage.
func TestGateway_AllMethodErrorBranches(t *testing.T) {
	calls := []struct {
		name string
		call func(*Gateway) error
	}{
		{"ListDatastores", func(g *Gateway) error { _, e := g.ListDatastores(context.Background()); return e }},
		{"GetDatastoreStatus", func(g *Gateway) error { _, e := g.GetDatastoreStatus(context.Background(), "s"); return e }},
		{"ListBackups", func(g *Gateway) error { _, e := g.ListBackups(context.Background(), "s"); return e }},
		{"GetBackup", func(g *Gateway) error { _, e := g.GetBackup(context.Background(), "s", "snap"); return e }},
		{"GetBackupNotes", func(g *Gateway) error { _, e := g.GetBackupNotes(context.Background(), "s", "id", "vm"); return e }},
		{"GetVerifyStatus", func(g *Gateway) error { _, e := g.GetVerifyStatus(context.Background(), "s", "u"); return e }},
		{"GetPruneStatus", func(g *Gateway) error { _, e := g.GetPruneStatus(context.Background(), "s", "u"); return e }},
		{"GetPBSVersion", func(g *Gateway) error { _, e := g.GetPBSVersion(context.Background()); return e }},
	}

	for _, tt := range calls {
		t.Run(tt.name, func(t *testing.T) {
			rec := &reqCapture{}
			srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
			g, _ := newTestGateway(t, srv)
			require.Error(t, tt.call(g))
		})
	}
}

// TestDecode covers the decode() helper branches.
func TestDecode(t *testing.T) {
	g := &Gateway{}

	t.Run("out nil returns nil", func(t *testing.T) {
		require.NoError(t, g.decode([]byte(`{"data":{}}`), nil))
	})

	t.Run("error envelope in 2xx maps to APIError", func(t *testing.T) {
		err := g.decode([]byte(`{"data":null,"errors":{"store":"missing"},"message":"boom"}`), &model.Datastore{})
		var ae *APIError
		require.ErrorAs(t, err, &ae)
		assert.NotEmpty(t, ae.Messages)
	})

	t.Run("message only maps to APIError", func(t *testing.T) {
		err := g.decode([]byte(`{"data":null,"message":"oops"}`), &model.Datastore{})
		var ae *APIError
		require.ErrorAs(t, err, &ae)
		assert.Contains(t, strings.Join(ae.Messages, " "), "oops")
	})

	t.Run("invalid json", func(t *testing.T) {
		err := g.decode([]byte(`not json`), &model.Datastore{})
		var ue *UpstreamError
		assert.ErrorAs(t, err, &ue)
	})

	t.Run("null data for struct maps to EmptyDataError", func(t *testing.T) {
		err := g.decode([]byte(`{"data":null}`), &model.Datastore{})
		var ee *EmptyDataError
		assert.ErrorAs(t, err, &ee)
	})

	t.Run("null data for slice is empty not error", func(t *testing.T) {
		out := []model.Datastore{}
		require.NoError(t, g.decode([]byte(`{"data":null}`), &out))
		assert.Empty(t, out)
	})

	t.Run("missing data key for struct maps to EmptyDataError", func(t *testing.T) {
		err := g.decode([]byte(`{}`), &model.Datastore{})
		var ee *EmptyDataError
		assert.ErrorAs(t, err, &ee)
	})

	t.Run("unmarshal target error", func(t *testing.T) {
		err := g.decode([]byte(`{"data":123}`), &model.Datastore{})
		var ue *UpstreamError
		assert.ErrorAs(t, err, &ue)
	})
}

// TestMapHTTPError covers the non-sentinel 4xx -> APIError mapping.
func TestMapHTTPError(t *testing.T) {
	g := &Gateway{}
	err := g.mapHTTPError(http.StatusBadRequest, []byte(`{"errors":{"param":"bad"},"message":"nope"}`))
	var ae *APIError
	require.ErrorAs(t, err, &ae)
	assert.NotEmpty(t, ae.Messages)
}

// TestExtractErrorMessages covers the invalid-body branch (returns nil).
func TestExtractErrorMessages(t *testing.T) {
	assert.Nil(t, extractErrorMessages([]byte(`not json`)))
	msgs := extractErrorMessages([]byte(`{"errors":{"a":"1"},"message":"m"}`))
	assert.Contains(t, strings.Join(msgs, " "), "a: 1")
	assert.Contains(t, strings.Join(msgs, " "), "m")
}

// TestErrorMessages covers the empty-map + message and map-only branches.
func TestErrorMessages(t *testing.T) {
	assert.Equal(t, []string{"k: v"}, errorMessages(map[string]string{"k": "v"}, ""))
	assert.Equal(t, []string{"hello"}, errorMessages(nil, "hello"))
	assert.Empty(t, errorMessages(nil, ""))
}

// TestIsSlicePtr covers the helper branches (previously 0% for PBS).
func TestIsSlicePtr(t *testing.T) {
	assert.False(t, isSlicePtr(nil))
	var ds model.Datastore
	assert.False(t, isSlicePtr(&ds))
	dss := []model.Datastore{}
	assert.True(t, isSlicePtr(&dss))
}

// TestBuildClientCACert covers the CA-success and invalid-CA branches.
func TestBuildClientCACert(t *testing.T) {
	t.Run("valid CA file builds client", func(t *testing.T) {
		pemPath := writeTestCA(t)
		c, err := buildClient(Config{
			HTTPClient: &http.Client{Transport: &http.Transport{}},
			CACertPath: pemPath,
		})
		require.NoError(t, err)
		require.NotNil(t, c)
		assert.NotNil(t, c.Transport())
	})

	t.Run("invalid CA file errors", func(t *testing.T) {
		_, err := buildClient(Config{CACertPath: "/nonexistent/ca.pem"})
		require.Error(t, err)
	})

	t.Run("invalid PEM errors", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "notca.pem")
		require.NoError(t, os.WriteFile(path, []byte("not a CA"), 0o600))
		_, err := buildClient(Config{HTTPClient: &http.Client{}, CACertPath: path})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no valid CA certificates")
	})

	t.Run("no CACertPath returns resty client", func(t *testing.T) {
		hc := &http.Client{Transport: &http.Transport{MaxIdleConns: 5}}
		c, err := buildClient(Config{HTTPClient: hc})
		require.NoError(t, err)
		require.NotNil(t, c)
		tr, ok := c.Transport().(*http.Transport)
		require.True(t, ok)
		assert.Equal(t, 5, tr.MaxIdleConns)
	})

	t.Run("nil HTTPClient gets default", func(t *testing.T) {
		c, err := buildClient(Config{})
		require.NoError(t, err)
		assert.NotNil(t, c)
	})
}

// TestBuildClientDerivesTransport covers deriving the resty transport from an
// injected HTTPClient transport and the default-transport path.
func TestBuildClientDerivesTransport(t *testing.T) {
	hc := &http.Client{Transport: &http.Transport{MaxIdleConns: 5}}
	c, err := buildClient(Config{HTTPClient: hc})
	require.NoError(t, err)
	tr, ok := c.Transport().(*http.Transport)
	require.True(t, ok)
	assert.Equal(t, 5, tr.MaxIdleConns)

	c2, err := buildClient(Config{}) // non-*http.Transport default path
	require.NoError(t, err)
	_, ok = c2.Transport().(*http.Transport)
	assert.True(t, ok)
}

// TestDoDefaultBackoff covers the default backoff when RetryBackoff <= 0.
func TestDoDefaultBackoff(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
	g := NewGateway(Config{
		Endpoint:    srv.URL,
		TokenSource: token.NewStaticTokenSource("user@pbs!tokenid=secret"),
		HTTPClient:  srv.Client(),
		Logger:      noopLogger{},
		Retries:     0,
	})
	var out []model.Datastore
	err := g.do(context.Background(), http.MethodGet, "/admin/datastore", &out)
	assert.Error(t, err)
}

// TestDoRetryCtxCanceled verifies the retry loop honors ctx cancellation.
func TestDoRetryCtxCanceled(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
	g, _ := newTestGateway(t, srv)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out []model.Datastore
	err := g.do(ctx, http.MethodGet, "/admin/datastore", &out)
	assert.Error(t, err)
}

// TestDoEmptyEndpoint covers the config-error branch in doOnce.
func TestDoEmptyEndpoint(t *testing.T) {
	g := NewGateway(Config{TokenSource: token.NewStaticTokenSource("tok")})
	var out []model.Datastore
	err := g.do(context.Background(), http.MethodGet, "/admin/datastore", &out)
	require.Error(t, err)
	var ue *UpstreamError
	assert.ErrorAs(t, err, &ue)
	assert.Equal(t, "config", ue.Op)
}

// TestDoOnceTLSErr covers the g.tlsErr branch in doOnce (misconfigured CA).
func TestDoOnceTLSErr(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[]}`, rec)
	g := NewGateway(Config{
		Endpoint:    srv.URL,
		TokenSource: token.NewStaticTokenSource("tok"),
		HTTPClient:  srv.Client(),
		CACertPath:  "/nonexistent/ca.pem",
	})
	var out []model.Datastore
	err := g.do(context.Background(), http.MethodGet, "/admin/datastore", &out)
	require.Error(t, err)
	var ue *UpstreamError
	assert.ErrorAs(t, err, &ue)
	assert.Equal(t, "config", ue.Op)
}

// TestDoNegativeRetries covers the attempts=1 branch when Retries < 0.
func TestDoNegativeRetries(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
	g := NewGateway(Config{
		Endpoint:    srv.URL,
		TokenSource: token.NewStaticTokenSource("tok"),
		HTTPClient:  srv.Client(),
		Logger:      noopLogger{},
		Retries:     -1,
	})
	var out []model.Datastore
	err := g.do(context.Background(), http.MethodGet, "/admin/datastore", &out)
	assert.Error(t, err)
	assert.Equal(t, 1, rec.count)
}

// writeTestCA generates a self-signed certificate and writes its PEM to a temp
// file, returning the path. Deterministic, no network.
func writeTestCA(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	tmpl := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "mcp-proxmox-test-ca"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	path := filepath.Join(t.TempDir(), "ca.pem")
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	require.NoError(t, os.WriteFile(path, pemBytes, 0o600))
	return path
}
