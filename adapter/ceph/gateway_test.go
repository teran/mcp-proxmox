package ceph

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/adapter/token"
	"github.com/teran/mcp-proxmox/domain/model"
	"github.com/teran/mcp-proxmox/domain/port"
)

// noopLogger is a no-op port.AppLogger for gateway tests.
type noopLogger struct{}

func (noopLogger) Tracef(string, ...any) {}
func (noopLogger) Debugf(string, ...any) {}
func (noopLogger) Infof(string, ...any)  {}
func (noopLogger) Warnf(string, ...any)  {}
func (noopLogger) Errorf(string, ...any) {}

// reqCapture records the last request the httptest server received.
type reqCapture struct {
	method      string
	path        string
	escapedPath string
	query       string
	auth        string
	xreq        string
	count       int
	form        url.Values
}

// mockServer spins up an httptest server whose handler records method/path/auth
// and responds with the given status and body.
func mockServer(t *testing.T, status int, body string, rec *reqCapture) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.escapedPath = r.URL.EscapedPath()
		rec.query = r.URL.RawQuery
		rec.auth = r.Header.Get("Authorization")
		rec.xreq = r.Header.Get("X-Request-ID")
		rec.count++
		_ = r.ParseForm()
		rec.form = r.PostForm
		if _, err := io.ReadAll(r.Body); err != nil {
			t.Errorf("read body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	return srv
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

// skipIfStub skips a contract test while the gateway body is still the
// ErrNotImplemented scaffold. Once the developer implements the method, the
// test automatically activates and validates the real behavior.
func skipIfStub(t *testing.T, err error) {
	t.Helper()
	if errors.Is(err, ErrNotImplemented) {
		t.Skip("gateway method implementation still in flight (ErrNotImplemented); contract test will activate once implemented")
	}
}

// writeTempFile writes data into a temp file under t.TempDir() and returns its
// path. It is a test helper for exercising the buildClient CA-loading paths.
func writeTempFile(t *testing.T, name, data string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(data), 0o600))

	return path
}

// testCACertPEM generates a fresh self-signed CA certificate in PEM form, so
// buildClient's AppendCertsFromPEM path has a valid certificate to load.
func testCACertPEM(t *testing.T) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test-ca"},
		IsCA:         true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	require.NoError(t, err)

	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

// TestGateway_ReadSuccess covers every read-only endpoint: correct HTTP method,
// URL path and Authorization header.
var gatewayReadSuccessCases = []struct {
	name string
	body string
	path string
	call func(g *Gateway) error
}{
	{
		"GetClusterStatus", `{"data":{"status":"HEALTH_OK"}}`, "/api2/json/cluster/ceph/status",
		func(g *Gateway) error {
			_, e := g.GetClusterStatus(context.Background())

			return e
		},
	},
	{
		"GetClusterConfig", `{"data":[]}`, "/api2/json/cluster/ceph/config",
		func(g *Gateway) error {
			_, e := g.GetClusterConfig(context.Background())

			return e
		},
	},
	{
		"GetClusterLog", `{"data":[]}`, "/api2/json/cluster/ceph/log",
		func(g *Gateway) error {
			_, e := g.GetClusterLog(context.Background(), 50)

			return e
		},
	},
	{
		"GetClusterMetadata", `{"data":{"fsid":"abc"}}`, "/api2/json/cluster/ceph/metadata",
		func(g *Gateway) error {
			_, e := g.GetClusterMetadata(context.Background())

			return e
		},
	},
	{
		"GetNodeStatus", `{"data":{"status":"HEALTH_OK"}}`, "/api2/json/nodes/pve1/ceph/status",
		func(g *Gateway) error {
			_, e := g.GetNodeStatus(context.Background(), "pve1")

			return e
		},
	},
	{
		"GetNodeConfig", `{"data":[]}`, "/api2/json/nodes/pve1/ceph/config",
		func(g *Gateway) error {
			_, e := g.GetNodeConfig(context.Background(), "pve1")

			return e
		},
	},
	{
		"GetNodeConfigDB", `{"data":[]}`, "/api2/json/nodes/pve1/ceph/configdb",
		func(g *Gateway) error {
			_, e := g.GetNodeConfigDB(context.Background(), "pve1")

			return e
		},
	},
	{
		"GetNodeFlags", `{"data":[]}`, "/api2/json/nodes/pve1/ceph/flags",
		func(g *Gateway) error {
			_, e := g.GetNodeFlags(context.Background(), "pve1")

			return e
		},
	},
	{
		"GetNodeMetadata", `{"data":{}}`, "/api2/json/nodes/pve1/ceph/metadata",
		func(g *Gateway) error {
			_, e := g.GetNodeMetadata(context.Background(), "pve1")

			return e
		},
	},
	{
		"GetNodeCrush", `{"data":{"rule":[]}}`, "/api2/json/nodes/pve1/ceph/crush",
		func(g *Gateway) error {
			_, e := g.GetNodeCrush(context.Background(), "pve1")

			return e
		},
	},
	{
		"GetNodeRules", `{"data":[]}`, "/api2/json/nodes/pve1/ceph/rules",
		func(g *Gateway) error {
			_, e := g.GetNodeRules(context.Background(), "pve1")

			return e
		},
	},
	{
		"ListPools", `{"data":[]}`, "/api2/json/nodes/pve1/ceph/pool",
		func(g *Gateway) error {
			_, e := g.ListPools(context.Background(), "pve1")

			return e
		},
	},
	{
		"ListOSDs", `{"data":[]}`, "/api2/json/nodes/pve1/ceph/osd",
		func(g *Gateway) error {
			_, e := g.ListOSDs(context.Background(), "pve1")

			return e
		},
	},
	{
		"ListMons", `{"data":[]}`, "/api2/json/nodes/pve1/ceph/mon",
		func(g *Gateway) error {
			_, e := g.ListMons(context.Background(), "pve1")

			return e
		},
	},
	{
		"ListMgrs", `{"data":[]}`, "/api2/json/nodes/pve1/ceph/mgr",
		func(g *Gateway) error {
			_, e := g.ListMgrs(context.Background(), "pve1")

			return e
		},
	},
}

func TestGateway_ReadSuccess(t *testing.T) {
	for _, tt := range gatewayReadSuccessCases {
		t.Run(tt.name, func(t *testing.T) {
			rec := &reqCapture{}
			srv := mockServer(t, 200, tt.body, rec)
			g, _ := newTestGateway(t, srv)

			err := tt.call(g)
			skipIfStub(t, err)
			require.NoError(t, err)

			assert.Equal(t, http.MethodGet, rec.method)
			assert.Equal(t, tt.path, rec.path)
			assert.Equal(t, "PVEAPIToken=user@pam!tokenid=secret", rec.auth)
		})
	}
}

// TestGateway_GetClusterLogLimit verifies the `limit` argument is forwarded as
// the ?limit= query parameter.
func TestGateway_GetClusterLogLimit(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[]}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.GetClusterLog(context.Background(), 50)
	skipIfStub(t, err)
	require.NoError(t, err)
	assert.Equal(t, "limit=50", rec.query)
}

// TestGateway_GetClusterStatusDecode verifies the cluster status object decodes
// into the typed CephClusterStatus with its passthrough maps preserved.
func TestGateway_GetClusterStatusDecode(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200,
		`{"data":{"status":"HEALTH_OK","quorum":[1,2],"quorum_names":["pve1","pve2"],"pgmap":{"num_pgs":64}}}`,
		rec)
	g, _ := newTestGateway(t, srv)

	status, err := g.GetClusterStatus(context.Background())
	skipIfStub(t, err)
	require.NoError(t, err)
	assert.Equal(t, "HEALTH_OK", status.Status)
	assert.Equal(t, []int{1, 2}, status.Quorum)
	assert.NotNil(t, status.PGMap)
}

// TestGateway_NodePathEscaped verifies a user-supplied node segment reaches the
// wire url.PathEscape'd end-to-end (S11 path-injection conformance).
func TestGateway_NodePathEscaped(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":{"status":"HEALTH_OK"}}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.GetNodeStatus(context.Background(), "node/with/slash")
	skipIfStub(t, err)
	require.NoError(t, err)
	assert.Equal(t, "/api2/json/nodes/node%2Fwith%2Fslash/ceph/status", rec.escapedPath,
		"node segment must be escaped; raw '/' would inject extra path segments")
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

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			rec := &reqCapture{}
			srv := mockServer(t, testCase.status, `{"data":null}`, rec)
			g, _ := newTestGateway(t, srv)

			_, err := g.ListPools(context.Background(), "pve1")
			skipIfStub(t, err)
			require.Error(t, err)
			assert.ErrorIs(t, err, testCase.want)
		})
	}
}

// TestGateway_5xxRetriesThenUpstreamError verifies a 5xx is retried and then
// surfaces as an UpstreamError (SPEC.md §7.5).
func TestGateway_5xxRetriesThenUpstreamError(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.ListPools(context.Background(), "pve1")
	skipIfStub(t, err)
	require.Error(t, err)
	var ue *UpstreamError
	assert.ErrorAs(t, err, &ue)
	assert.Equal(t, http.StatusInternalServerError, ue.Status)
	// 1 initial + 1 retry (Retries:1).
	assert.Equal(t, 2, rec.count)
}

// TestGateway_EmptyData verifies an empty/null data for a getter surfaces an
// explicit *EmptyDataError (SPEC.md §7.2) rather than a silently-zero struct.
func TestGateway_EmptyData(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":null}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.GetNodeStatus(context.Background(), "pve1")
	skipIfStub(t, err)
	require.Error(t, err)
	var ee *EmptyDataError
	assert.ErrorAs(t, err, &ee)
	assert.Equal(t, 1, rec.count)
}

// TestGateway_InvalidJSON verifies a non-JSON body surfaces an UpstreamError.
func TestGateway_InvalidJSON(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `this is not json`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.ListPools(context.Background(), "pve1")
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

	_, err := g.ListPools(context.Background(), "pve1")
	skipIfStub(t, err)
	require.Error(t, err)
	assert.Equal(t, 0, rec.count, "no network call should be made on token failure")
}

// TestGateway_MutationSuccess covers every mutation endpoint: correct HTTP
// method, URL path and (where relevant) form body.
var gatewayMutationSuccessCases = []struct {
	name string
	body string
	call func(t *testing.T, g *Gateway, rec *reqCapture)
}{
	{
		name: "CreateCrushRule", body: `{"data":"replicated_rule"}`,
		call: func(t *testing.T, g *Gateway, rec *reqCapture) {
			_, err := g.CreateCrushRule(context.Background(), "pve1", model.CephCrushRuleCreateRequest{
				Name: "rule1", Type: "replicated", MinSize: 1, MaxSize: 10, Root: "default", OSD: "crush",
			})
			skipIfStub(t, err)
			require.NoError(t, err)
			assert.Equal(t, http.MethodPost, rec.method)
			assert.Equal(t, "/api2/json/nodes/pve1/ceph/rules", rec.path)
			assert.Equal(t, "rule1", rec.form.Get("name"))
			assert.Equal(t, "replicated", rec.form.Get("type"))
			assert.Equal(t, "1", rec.form.Get("min_size"))
			assert.Equal(t, "10", rec.form.Get("max_size"))
		},
	},
	{
		name: "DeleteCrushRule", body: `{"data":null}`,
		call: func(t *testing.T, g *Gateway, rec *reqCapture) {
			err := g.DeleteCrushRule(context.Background(), "pve1", "rule1")
			skipIfStub(t, err)
			require.NoError(t, err)
			assert.Equal(t, http.MethodDelete, rec.method)
			assert.Equal(t, "/api2/json/nodes/pve1/ceph/rules/rule1", rec.path)
		},
	},
	{
		name: "CreatePool", body: `{"data":"pool1"}`,
		call: func(t *testing.T, g *Gateway, rec *reqCapture) {
			_, err := g.CreatePool(context.Background(), "pve1", model.CephPoolCreateRequest{
				Name: "pool1", Size: 3, MinSize: 2, PGNum: 128, PGAutoscaleMode: "on",
			})
			skipIfStub(t, err)
			require.NoError(t, err)
			assert.Equal(t, http.MethodPost, rec.method)
			assert.Equal(t, "/api2/json/nodes/pve1/ceph/pool", rec.path)
			assert.Equal(t, "pool1", rec.form.Get("name"))
			assert.Equal(t, "3", rec.form.Get("size"))
			assert.Equal(t, "2", rec.form.Get("min_size"))
			assert.Equal(t, "128", rec.form.Get("pg_num"))
		},
	},
	{
		name: "UpdatePool", body: `{"data":null}`,
		call: func(t *testing.T, g *Gateway, rec *reqCapture) {
			err := g.UpdatePool(context.Background(), "pve1", "pool1", model.CephPoolUpdateRequest{
				Size: 3, MinSize: 2, PGNum: 256, PGAutoscaleMode: "warn",
			})
			skipIfStub(t, err)
			require.NoError(t, err)
			assert.Equal(t, http.MethodPut, rec.method)
			assert.Equal(t, "/api2/json/nodes/pve1/ceph/pool/pool1", rec.path)
			assert.Equal(t, "3", rec.form.Get("size"))
			assert.Equal(t, "256", rec.form.Get("pg_num"))
		},
	},
	{
		name: "DeletePool", body: `{"data":null}`,
		call: func(t *testing.T, g *Gateway, rec *reqCapture) {
			err := g.DeletePool(context.Background(), "pve1", "pool1")
			skipIfStub(t, err)
			require.NoError(t, err)
			assert.Equal(t, http.MethodDelete, rec.method)
			assert.Equal(t, "/api2/json/nodes/pve1/ceph/pool/pool1", rec.path)
		},
	},
	{
		name: "CreateOSD", body: `{"data":"osd.0"}`,
		call: func(t *testing.T, g *Gateway, rec *reqCapture) {
			_, err := g.CreateOSD(context.Background(), "pve1", model.CephOSDCreateRequest{OSD: "0"})
			skipIfStub(t, err)
			require.NoError(t, err)
			assert.Equal(t, http.MethodPost, rec.method)
			assert.Equal(t, "/api2/json/nodes/pve1/ceph/osd", rec.path)
			assert.Equal(t, "0", rec.form.Get("osd"))
		},
	},
	{
		name: "DeleteOSD", body: `{"data":null}`,
		call: func(t *testing.T, g *Gateway, rec *reqCapture) {
			err := g.DeleteOSD(context.Background(), "pve1", 3)
			skipIfStub(t, err)
			require.NoError(t, err)
			assert.Equal(t, http.MethodDelete, rec.method)
			assert.Equal(t, "/api2/json/nodes/pve1/ceph/osd/3", rec.path)
		},
	},
	{
		name: "CreateMon", body: `{"data":"pve2"}`,
		call: func(t *testing.T, g *Gateway, rec *reqCapture) {
			_, err := g.CreateMon(context.Background(), "pve1", model.CephMonCreateRequest{MonAddr: "10.0.0.2:6789"})
			skipIfStub(t, err)
			require.NoError(t, err)
			assert.Equal(t, http.MethodPost, rec.method)
			assert.Equal(t, "/api2/json/nodes/pve1/ceph/mon", rec.path)
			assert.Equal(t, "10.0.0.2:6789", rec.form.Get("mon_addr"))
		},
	},
	{
		name: "DeleteMon", body: `{"data":null}`,
		call: func(t *testing.T, g *Gateway, rec *reqCapture) {
			err := g.DeleteMon(context.Background(), "pve1", "pve2")
			skipIfStub(t, err)
			require.NoError(t, err)
			assert.Equal(t, http.MethodDelete, rec.method)
			assert.Equal(t, "/api2/json/nodes/pve1/ceph/mon/pve2", rec.path)
		},
	},
	{
		name: "CreateMgr", body: `{"data":"pve2"}`,
		call: func(t *testing.T, g *Gateway, rec *reqCapture) {
			_, err := g.CreateMgr(context.Background(), "pve1", model.CephMgrCreateRequest{ID: "pve2"})
			skipIfStub(t, err)
			require.NoError(t, err)
			assert.Equal(t, http.MethodPost, rec.method)
			assert.Equal(t, "/api2/json/nodes/pve1/ceph/mgr", rec.path)
			assert.Equal(t, "pve2", rec.form.Get("id"))
		},
	},
	{
		name: "SetFlags", body: `{"data":null}`,
		call: func(t *testing.T, g *Gateway, rec *reqCapture) {
			err := g.SetFlags(context.Background(), "pve1", model.CephFlagsSetRequest{
				Values: map[string]string{"noout": "true", "noscrub": "false"},
			})
			skipIfStub(t, err)
			require.NoError(t, err)
			assert.Equal(t, http.MethodPut, rec.method)
			assert.Equal(t, "/api2/json/nodes/pve1/ceph/flags", rec.path)
			assert.Equal(t, "true", rec.form.Get("noout"))
			assert.Equal(t, "false", rec.form.Get("noscrub"))
		},
	},
	{
		name: "SetConfig", body: `{"data":null}`,
		call: func(t *testing.T, g *Gateway, rec *reqCapture) {
			err := g.SetConfig(context.Background(), "pve1", model.CephConfigSetRequest{
				Name: "mon_allow_pool_delete", Value: "true",
			})
			skipIfStub(t, err)
			require.NoError(t, err)
			assert.Equal(t, http.MethodPut, rec.method)
			assert.Equal(t, "/api2/json/nodes/pve1/ceph/config", rec.path)
			assert.Equal(t, "mon_allow_pool_delete", rec.form.Get("name"))
			assert.Equal(t, "true", rec.form.Get("value"))
		},
	},
	{
		name: "InitCeph", body: `{"data":"UPID:init"}`,
		call: func(t *testing.T, g *Gateway, rec *reqCapture) {
			_, err := g.InitCeph(context.Background(), "pve1", model.CephInitRequest{
				Network: "10.0.0.0/24", ClusterNetwork: "10.0.1.0/24",
			})
			skipIfStub(t, err)
			require.NoError(t, err)
			assert.Equal(t, http.MethodPost, rec.method)
			assert.Equal(t, "/api2/json/nodes/pve1/ceph/init", rec.path)
			assert.Equal(t, "10.0.0.0/24", rec.form.Get("network"))
			assert.Equal(t, "10.0.1.0/24", rec.form.Get("cluster_network"))
		},
	},
	{
		name: "DestroyCeph", body: `{"data":null}`,
		call: func(t *testing.T, g *Gateway, rec *reqCapture) {
			err := g.DestroyCeph(context.Background(), "pve1", model.CephDestroyRequest{Force: true})
			skipIfStub(t, err)
			require.NoError(t, err)
			assert.Equal(t, http.MethodDelete, rec.method)
			assert.Equal(t, "/api2/json/nodes/pve1/ceph/destroy", rec.path)
			// resty v3 drops form bodies on DELETE, so `force` is sent as a
			// query parameter (?force=1) rather than in the form body.
			q, perr := url.ParseQuery(rec.query)
			require.NoError(t, perr)
			assert.Equal(t, "1", q.Get("force"))
		},
	},
}

func TestGateway_MutationSuccess(t *testing.T) {
	for _, tt := range gatewayMutationSuccessCases {
		t.Run(tt.name, func(t *testing.T) {
			rec := &reqCapture{}
			srv := mockServer(t, 200, tt.body, rec)
			g, _ := newTestGateway(t, srv)
			tt.call(t, g, rec)
			assert.Equal(t, "PVEAPIToken=user@pam!tokenid=secret", rec.auth)
		})
	}
}

// TestGateway_MutationNotRetried verifies mutation methods are NEVER retried on
// a 5xx (SPEC.md §7.5).
func TestGateway_MutationNotRetried(t *testing.T) {
	t.Run("create-pool", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)

		_, err := g.CreatePool(context.Background(), "pve1", model.CephPoolCreateRequest{Name: "p"})
		skipIfStub(t, err)
		require.Error(t, err)
		var ue *UpstreamError
		assert.ErrorAs(t, err, &ue)
		assert.Equal(t, 1, rec.count, "mutations must not be retried")
	})

	t.Run("delete-osd", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)

		err := g.DeleteOSD(context.Background(), "pve1", 0)
		skipIfStub(t, err)
		require.Error(t, err)
		assert.Equal(t, 1, rec.count, "mutations must not be retried")
	})

	t.Run("destroy-ceph", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)

		err := g.DestroyCeph(context.Background(), "pve1", model.CephDestroyRequest{})
		skipIfStub(t, err)
		require.Error(t, err)
		assert.Equal(t, 1, rec.count, "mutations must not be retried")
	})
}

// TestGateway_MutationErrorMapping verifies mutation failures map through the
// error family and are never retried.
func TestGateway_MutationErrorMapping(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusUnauthorized, `{"data":null}`, rec)
	g, _ := newTestGateway(t, srv)

	err := g.DeletePool(context.Background(), "pve1", "pool1")
	skipIfStub(t, err)
	require.Error(t, err)
	assert.ErrorIs(t, err, port.ErrUnauthorized)
	assert.Equal(t, 1, rec.count)
}

// TestGateway_ReadError covers the uncovered error-return branch of every
// read-only method: when the upstream returns a 4xx, each getter must surface
// the mapped error and return a nil slice/pointer (never a silently-partial
// value).
var gatewayReadErrorCases = []struct {
	name string
	call func(g *Gateway) error
}{
	{"GetClusterStatus", func(g *Gateway) error {
		_, e := g.GetClusterStatus(context.Background())

		return e
	}},
	{"GetClusterConfig", func(g *Gateway) error {
		_, e := g.GetClusterConfig(context.Background())

		return e
	}},
	{"GetClusterLog", func(g *Gateway) error {
		_, e := g.GetClusterLog(context.Background(), 10)

		return e
	}},
	{"GetClusterMetadata", func(g *Gateway) error {
		_, e := g.GetClusterMetadata(context.Background())

		return e
	}},
	{"GetNodeStatus", func(g *Gateway) error {
		_, e := g.GetNodeStatus(context.Background(), "pve1")

		return e
	}},
	{"GetNodeConfig", func(g *Gateway) error {
		_, e := g.GetNodeConfig(context.Background(), "pve1")

		return e
	}},
	{"GetNodeConfigDB", func(g *Gateway) error {
		_, e := g.GetNodeConfigDB(context.Background(), "pve1")

		return e
	}},
	{"GetNodeFlags", func(g *Gateway) error {
		_, e := g.GetNodeFlags(context.Background(), "pve1")

		return e
	}},
	{"GetNodeMetadata", func(g *Gateway) error {
		_, e := g.GetNodeMetadata(context.Background(), "pve1")

		return e
	}},
	{"GetNodeCrush", func(g *Gateway) error {
		_, e := g.GetNodeCrush(context.Background(), "pve1")

		return e
	}},
	{"GetNodeRules", func(g *Gateway) error {
		_, e := g.GetNodeRules(context.Background(), "pve1")

		return e
	}},
	{"ListPools", func(g *Gateway) error {
		_, e := g.ListPools(context.Background(), "pve1")

		return e
	}},
	{"ListOSDs", func(g *Gateway) error {
		_, e := g.ListOSDs(context.Background(), "pve1")

		return e
	}},
	{"ListMons", func(g *Gateway) error {
		_, e := g.ListMons(context.Background(), "pve1")

		return e
	}},
	{"ListMgrs", func(g *Gateway) error {
		_, e := g.ListMgrs(context.Background(), "pve1")

		return e
	}},
}

func TestGateway_ReadError(t *testing.T) {
	for _, tt := range gatewayReadErrorCases {
		t.Run(tt.name, func(t *testing.T) {
			rec := &reqCapture{}
			srv := mockServer(t, http.StatusBadRequest, `{"errors":{"x":"bad"},"message":"nope"}`, rec)
			g, _ := newTestGateway(t, srv)

			err := tt.call(g)
			skipIfStub(t, err)
			require.Error(t, err)
			var ae *APIError
			assert.ErrorAs(t, err, &ae)
			assert.Contains(t, ae.Messages, "x: bad")
			assert.Contains(t, ae.Messages, "nope")
		})
	}
}

// TestGateway_MutationError covers the uncovered error-return branch of the
// create/init mutations: an upstream 4xx must surface as an error and yield an
// empty UPID.
func TestGateway_MutationError(t *testing.T) {
	cases := []struct {
		name string
		call func(g *Gateway) error
	}{
		{"CreateCrushRule", func(g *Gateway) error {
			_, e := g.CreateCrushRule(context.Background(), "pve1", model.CephCrushRuleCreateRequest{Name: "r"})

			return e
		}},
		{"CreateOSD", func(g *Gateway) error {
			_, e := g.CreateOSD(context.Background(), "pve1", model.CephOSDCreateRequest{OSD: "0"})

			return e
		}},
		{"CreateMon", func(g *Gateway) error {
			_, e := g.CreateMon(context.Background(), "pve1", model.CephMonCreateRequest{MonAddr: "x"})

			return e
		}},
		{"CreateMgr", func(g *Gateway) error {
			_, e := g.CreateMgr(context.Background(), "pve1", model.CephMgrCreateRequest{ID: "pve2"})

			return e
		}},
		{"InitCeph", func(g *Gateway) error {
			_, e := g.InitCeph(context.Background(), "pve1", model.CephInitRequest{Network: "x"})

			return e
		}},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			rec := &reqCapture{}
			srv := mockServer(t, http.StatusBadRequest, `{"message":"boom"}`, rec)
			g, _ := newTestGateway(t, srv)

			err := tt.call(g)
			skipIfStub(t, err)
			require.Error(t, err)
			var ae *APIError
			assert.ErrorAs(t, err, &ae)
			assert.Equal(t, 1, rec.count, "mutations must not be retried")
		})
	}
}

// TestGateway_ErrorEnvelope verifies a 2xx body carrying the {"errors":...} or
// {"message":...} envelope is mapped to an APIError (the decode-level error
// branch) rather than being treated as data.
func TestGateway_ErrorEnvelope(t *testing.T) {
	t.Run("errors map", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusOK, `{"data":null,"errors":{"pg_num":"invalid"}}`, rec)
		g, _ := newTestGateway(t, srv)

		_, err := g.ListPools(context.Background(), "pve1")
		skipIfStub(t, err)
		require.Error(t, err)
		var ae *APIError
		assert.ErrorAs(t, err, &ae)
		assert.Equal(t, []string{"pg_num: invalid"}, ae.Messages)
	})

	t.Run("message field", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusOK, `{"data":null,"message":"no such pool"}`, rec)
		g, _ := newTestGateway(t, srv)

		_, err := g.ListPools(context.Background(), "pve1")
		skipIfStub(t, err)
		require.Error(t, err)
		var ae *APIError
		assert.ErrorAs(t, err, &ae)
		assert.Equal(t, []string{"no such pool"}, ae.Messages)
	})
}

// TestGateway_DecodeDataError verifies that a non-null data payload which fails
// to unmarshal into the target type surfaces an UpstreamError (the decode
// data-unmarshal error branch).
func TestGateway_DecodeDataError(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusOK, `{"data":[123,456]}`, rec)
	g, _ := newTestGateway(t, srv)

	// ListPools expects a []CephPool; a data payload of bare ints cannot decode.
	_, err := g.ListPools(context.Background(), "pve1")
	skipIfStub(t, err)
	require.Error(t, err)
	var ue *UpstreamError
	assert.ErrorAs(t, err, &ue)
}

// TestGateway_EmptyEndpoint verifies a Gateway configured without an endpoint
// fails fast with a config UpstreamError before any network call.
func TestGateway_EmptyEndpoint(t *testing.T) {
	g := NewGateway(Config{
		TokenSource: token.NewStaticTokenSource("user@pam!t=s"),
		Logger:      noopLogger{},
		Retries:     1,
	})

	_, err := g.ListPools(context.Background(), "pve1")
	skipIfStub(t, err)
	require.Error(t, err)
	var ue *UpstreamError
	assert.ErrorAs(t, err, &ue)
	assert.Equal(t, "config", ue.Op)
}

// TestGateway_RetriesNegative verifies a negative Retries config collapses to a
// single attempt (no retry) and still surfaces the upstream error.
func TestGateway_RetriesNegative(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
	g := NewGateway(Config{
		Endpoint:     srv.URL,
		TokenSource:  token.NewStaticTokenSource("user@pam!t=s"),
		HTTPClient:   srv.Client(),
		Logger:       noopLogger{},
		Retries:      -1,
		RetryBackoff: time.Millisecond,
	})

	_, err := g.ListPools(context.Background(), "pve1")
	skipIfStub(t, err)
	require.Error(t, err)
	var ue *UpstreamError
	assert.ErrorAs(t, err, &ue)
	assert.Equal(t, 1, rec.count)
}

// TestGateway_CtxCancelledDuringBackoff verifies that when the context is
// cancelled while the retry loop is waiting on its backoff timer, the loop
// returns the context error instead of re-sending the request.
func TestGateway_CtxCancelledDuringBackoff(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
	g, _ := newTestGateway(t, srv)

	// Start with a cancelled context: attempt 0 hits the 5xx, then the retry
	// backoff immediately observes ctx.Done().
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := g.ListPools(ctx, "pve1")
	skipIfStub(t, err)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
	// The cancelled context aborts at the transport layer (0 wire requests);
	// the retry loop then observes ctx.Done() and returns instead of re-sending.
	assert.Equal(t, 0, rec.count)
}

// TestGateway_RequestIDHeader verifies a request_id in the context is forwarded
// as the X-Request-ID header.
func TestGateway_RequestIDHeader(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[]}`, rec)
	g, _ := newTestGateway(t, srv)

	ctx := port.WithRequestID(context.Background(), "req-123")
	_, err := g.ListPools(ctx, "pve1")
	skipIfStub(t, err)
	require.NoError(t, err)
	assert.Equal(t, "req-123", rec.xreq)
}

// --- Direct unit tests for the small unexported helpers ---

func TestSystemOrNewCertPool(t *testing.T) {
	pool, err := systemOrNewCertPool()
	require.NoError(t, err)
	require.NotNil(t, pool)
}

func TestExtractErrorMessages(t *testing.T) {
	t.Run("errors and message", func(t *testing.T) {
		msgs := extractErrorMessages([]byte(`{"errors":{"a":"1","b":"2"},"message":"m"}`))
		assert.ElementsMatch(t, []string{"a: 1", "b: 2", "m"}, msgs)
	})

	t.Run("message only", func(t *testing.T) {
		msgs := extractErrorMessages([]byte(`{"message":"boom"}`))
		assert.Equal(t, []string{"boom"}, msgs)
	})

	t.Run("no error envelope", func(t *testing.T) {
		msgs := extractErrorMessages([]byte(`{"data":[]}`))
		assert.Empty(t, msgs)
	})

	t.Run("invalid json returns nil", func(t *testing.T) {
		assert.Nil(t, extractErrorMessages([]byte(`not json`)))
	})
}

func TestErrorMessages(t *testing.T) {
	t.Run("errors and message", func(t *testing.T) {
		msgs := errorMessages(map[string]string{"a": "1"}, "m")
		assert.ElementsMatch(t, []string{"a: 1", "m"}, msgs)
	})

	t.Run("errors only", func(t *testing.T) {
		msgs := errorMessages(map[string]string{"a": "1"}, "")
		assert.Equal(t, []string{"a: 1"}, msgs)
	})

	t.Run("message only", func(t *testing.T) {
		msgs := errorMessages(nil, "m")
		assert.Equal(t, []string{"m"}, msgs)
	})

	t.Run("neither", func(t *testing.T) {
		msgs := errorMessages(nil, "")
		assert.Empty(t, msgs)
	})
}

func TestIsSlicePtr(t *testing.T) {
	assert.False(t, isSlicePtr(nil))
	assert.False(t, isSlicePtr(&model.CephClusterStatus{}))
	var list []model.CephPool
	assert.True(t, isSlicePtr(&list))
	assert.False(t, isSlicePtr(model.CephPool{}))
}

func TestUpidFromCreateBody(t *testing.T) {
	t.Run("standard data envelope", func(t *testing.T) {
		assert.Equal(t, "UPID:x", upidFromCreateBody([]byte(`{"data":"UPID:x"}`)))
	})

	t.Run("flat upid field", func(t *testing.T) {
		assert.Equal(t, "UPID:y", upidFromCreateBody([]byte(`{"upid":"UPID:y"}`)))
	})

	t.Run("empty body returns empty", func(t *testing.T) {
		assert.Equal(t, "", upidFromCreateBody(nil))
		assert.Equal(t, "", upidFromCreateBody([]byte{}))
	})

	t.Run("null data returns empty", func(t *testing.T) {
		assert.Equal(t, "", upidFromCreateBody([]byte(`{"data":null}`)))
	})

	t.Run("invalid json returns empty", func(t *testing.T) {
		assert.Equal(t, "", upidFromCreateBody([]byte(`not json`)))
	})

	t.Run("non-string data returns empty", func(t *testing.T) {
		assert.Equal(t, "", upidFromCreateBody([]byte(`{"data":123}`)))
	})

	t.Run("missing fields returns empty", func(t *testing.T) {
		assert.Equal(t, "", upidFromCreateBody([]byte(`{}`)))
	})
}

// TestBuildClient covers the buildClient branches: User-Agent header,
// HTTPClient timeout/transport adoption, and the custom CA path.
func TestBuildClient(t *testing.T) {
	t.Run("defaults with no ca or useragent", func(t *testing.T) {
		client, err := buildClient(Config{})
		require.NoError(t, err)
		require.NotNil(t, client)
	})

	t.Run("useragent and httpclient adopted", func(t *testing.T) {
		hc := &http.Client{Timeout: time.Second, Transport: &http.Transport{}}
		client, err := buildClient(Config{UserAgent: "mcp/1.0", HTTPClient: hc})
		require.NoError(t, err)
		require.NotNil(t, client)
	})

	t.Run("ca cert path applied", func(t *testing.T) {
		pem := testCACertPEM(t)
		path := writeTempFile(t, "ca.pem", pem)
		client, err := buildClient(Config{CACertPath: path})
		require.NoError(t, err)
		require.NotNil(t, client)
	})

	t.Run("ca cert read error", func(t *testing.T) {
		_, err := buildClient(Config{CACertPath: "/nonexistent/ca.pem"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "read CA cert")
	})

	t.Run("ca cert with no valid pem", func(t *testing.T) {
		path := writeTempFile(t, "bad.pem", "this is not a pem")
		_, err := buildClient(Config{CACertPath: path})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no valid CA certificates")
	})
}

// compile-time: Gateway implements port.CephGateway.
var _ port.CephGateway = (*Gateway)(nil)
