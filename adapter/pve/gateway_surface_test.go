package pve

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

// TestGateway_AllMethodSuccess exercises every PVE read-only method's happy
// path, asserting the returned struct fields and the request method/path.
func TestGateway_AllMethodSuccess(t *testing.T) {
	tests := []struct {
		name string
		body string
		path string
		call func(*Gateway) error
	}{
		{
			name: "ListNodes",
			body: `{"data":[{"node":"pve1","status":"online","cpu":0.5}]}`,
			path: "/api2/json/nodes",
			call: func(g *Gateway) error {
				ns, err := g.ListNodes(context.Background())
				if assert.NoError(t, err) {
					require.Len(t, ns, 1)
					assert.Equal(t, "pve1", ns[0].Node)
					assert.Equal(t, "online", ns[0].Status)
				}
				return err
			},
		},
		{
			name: "GetNodeStatus",
			body: `{"data":{"node":"pve1","status":"online","mem":1024}}`,
			path: "/api2/json/nodes/pve1/status",
			call: func(g *Gateway) error {
				ns, err := g.GetNodeStatus(context.Background(), "pve1")
				if assert.NoError(t, err) {
					assert.Equal(t, "online", ns.Status)
					assert.Equal(t, int64(1024), ns.Mem)
				}
				return err
			},
		},
		{
			name: "GetClusterStatus",
			body: `{"data":[{"name":"cluster","type":"cluster","quorate":1,"nodes":3}]}`,
			path: "/api2/json/cluster/status",
			call: func(g *Gateway) error {
				cs, err := g.GetClusterStatus(context.Background())
				if assert.NoError(t, err) {
					require.Len(t, cs, 1)
					assert.Equal(t, 1, cs[0].Quorum)
				}
				return err
			},
		},
		{
			name: "GetClusterResources",
			body: `{"data":[{"id":"qemu/100","type":"qemu","vmid":100,"status":"running"}]}`,
			path: "/api2/json/cluster/resources",
			call: func(g *Gateway) error {
				rs, err := g.GetClusterResources(context.Background())
				if assert.NoError(t, err) {
					require.Len(t, rs, 1)
					assert.Equal(t, "qemu/100", rs[0].ID)
				}
				return err
			},
		},
		{
			name: "GetNextID",
			body: `{"data":"201"}`,
			path: "/api2/json/cluster/nextid",
			call: func(g *Gateway) error {
				id, err := g.GetNextID(context.Background())
				if assert.NoError(t, err) {
					assert.Equal(t, "201", id)
				}
				return err
			},
		},
		{
			name: "ListVMs",
			body: `{"data":[{"vmid":100,"name":"web","status":"running"}]}`,
			path: "/api2/json/nodes/pve1/qemu",
			call: func(g *Gateway) error {
				vms, err := g.ListVMs(context.Background(), "pve1")
				if assert.NoError(t, err) {
					require.Len(t, vms, 1)
					assert.Equal(t, "web", vms[0].Name)
				}
				return err
			},
		},
		{
			name: "GetVMConfig",
			body: `{"data":{"vmid":100,"name":"web","cores":2,"memory":1024}}`,
			path: "/api2/json/nodes/pve1/qemu/100/config",
			call: func(g *Gateway) error {
				cfg, err := g.GetVMConfig(context.Background(), "pve1", 100)
				if assert.NoError(t, err) {
					assert.Equal(t, 100, cfg.VMID)
					assert.Equal(t, 2, cfg.Cores)
				}
				return err
			},
		},
		{
			name: "GetVMStatus",
			body: `{"data":{"vmid":100,"status":"running","cpu":0.25}}`,
			path: "/api2/json/nodes/pve1/qemu/100/status/current",
			call: func(g *Gateway) error {
				st, err := g.GetVMStatus(context.Background(), "pve1", 100)
				if assert.NoError(t, err) {
					assert.Equal(t, "running", st.Status)
				}
				return err
			},
		},
		{
			name: "ListLXCs",
			body: `{"data":[{"vmid":200,"name":"ct1","status":"running"}]}`,
			path: "/api2/json/nodes/pve1/lxc",
			call: func(g *Gateway) error {
				ls, err := g.ListLXCs(context.Background(), "pve1")
				if assert.NoError(t, err) {
					require.Len(t, ls, 1)
					assert.Equal(t, 200, ls[0].VMID)
				}
				return err
			},
		},
		{
			name: "GetLXCConfig",
			body: `{"data":{"vmid":200,"hostname":"ct1","cores":1}}`,
			path: "/api2/json/nodes/pve1/lxc/200/config",
			call: func(g *Gateway) error {
				cfg, err := g.GetLXCConfig(context.Background(), "pve1", 200)
				if assert.NoError(t, err) {
					assert.Equal(t, "ct1", cfg.Hostname)
				}
				return err
			},
		},
		{
			name: "GetLXCStatus",
			body: `{"data":{"vmid":200,"status":"running"}}`,
			path: "/api2/json/nodes/pve1/lxc/200/status/current",
			call: func(g *Gateway) error {
				st, err := g.GetLXCStatus(context.Background(), "pve1", 200)
				if assert.NoError(t, err) {
					assert.Equal(t, "running", st.Status)
				}
				return err
			},
		},
		{
			name: "ListStorage",
			body: `{"data":[{"storage":"local","type":"dir","active":true}]}`,
			path: "/api2/json/storage",
			call: func(g *Gateway) error {
				ss, err := g.ListStorage(context.Background())
				if assert.NoError(t, err) {
					require.Len(t, ss, 1)
					assert.Equal(t, "local", ss[0].Storage)
				}
				return err
			},
		},
		{
			name: "GetStorageStatus",
			body: `{"data":{"storage":"local","type":"dir","total":1000,"used":400}}`,
			path: "/api2/json/nodes/pve1/storage/local/status",
			call: func(g *Gateway) error {
				st, err := g.GetStorageStatus(context.Background(), "pve1", "local")
				if assert.NoError(t, err) {
					assert.Equal(t, int64(1000), st.Total)
					assert.Equal(t, int64(400), st.Used)
				}
				return err
			},
		},
		{
			name: "ListNetwork",
			body: `{"data":[{"iface":"eth0","type":"eth","bridge":"vmbr0"}]}`,
			path: "/api2/json/nodes/pve1/network",
			call: func(g *Gateway) error {
				ns, err := g.ListNetwork(context.Background(), "pve1")
				if assert.NoError(t, err) {
					require.Len(t, ns, 1)
					assert.Equal(t, "eth0", ns[0].Iface)
				}
				return err
			},
		},
		{
			name: "ListTasks",
			body: `{"data":[{"upid":"u","type":"qmstart","status":"stopped"}]}`,
			path: "/api2/json/nodes/pve1/tasks",
			call: func(g *Gateway) error {
				ts, err := g.ListTasks(context.Background(), "pve1", port.TaskListOptions{})
				if assert.NoError(t, err) {
					require.Len(t, ts, 1)
					assert.Equal(t, "qmstart", ts[0].Type)
				}
				return err
			},
		},
		{
			name: "GetTaskStatus",
			body: `{"data":{"upid":"u","status":"stopped","exitstatus":"OK"}}`,
			path: "/api2/json/nodes/pve1/tasks/UPID:1/status",
			call: func(g *Gateway) error {
				st, err := g.GetTaskStatus(context.Background(), "pve1", "UPID:1")
				if assert.NoError(t, err) {
					assert.Equal(t, "stopped", st.Status)
				}
				return err
			},
		},
		{
			name: "GetTaskLog",
			body: `{"data":[{"n":1,"t":"TASK OK"}]}`,
			path: "/api2/json/nodes/pve1/tasks/UPID:1/log",
			call: func(g *Gateway) error {
				ls, err := g.GetTaskLog(context.Background(), "pve1", "UPID:1", 50)
				if assert.NoError(t, err) {
					require.Len(t, ls, 1)
					assert.Equal(t, "TASK OK", ls[0].Text)
				}
				return err
			},
		},
		{
			name: "GetPVEVersion",
			body: `{"data":{"version":"8.2.2","release":"8.2","repoid":"abc"}}`,
			path: "/api2/json/version",
			call: func(g *Gateway) error {
				v, err := g.GetPVEVersion(context.Background())
				if assert.NoError(t, err) {
					assert.Equal(t, "8.2.2", v.Version)
					assert.Equal(t, "8.2", v.Release)
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
			assert.Equal(t, "PVEAPIToken=user@pam!tokenid=secret", rec.auth)
		})
	}
}

// TestGateway_ListTasksQueryParams verifies non-zero TaskListOptions serialize
// into query parameters, and zero options produce no query.
func TestGateway_ListTasksQueryParams(t *testing.T) {
	t.Run("all filters", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, 200, `{"data":[]}`, rec)
		g, _ := newTestGateway(t, srv)

		opts := port.TaskListOptions{
			VMD: 100, TypeFilter: "qmstart", Since: 111, Until: 222,
			Source: "pve1", Limit: 7,
		}
		_, err := g.ListTasks(context.Background(), "pve1", opts)
		require.NoError(t, err)
		assert.Equal(t, "/api2/json/nodes/pve1/tasks", rec.path)
		assert.Equal(t, "limit=7&since=111&source=pve1&typefilter=qmstart&until=222&vmid=100", rec.query)
	})

	t.Run("single filter", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, 200, `{"data":[]}`, rec)
		g, _ := newTestGateway(t, srv)

		_, err := g.ListTasks(context.Background(), "pve1", port.TaskListOptions{VMD: 100})
		require.NoError(t, err)
		assert.Equal(t, "vmid=100", rec.query)
	})

	t.Run("zero options no query", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, 200, `{"data":[]}`, rec)
		g, _ := newTestGateway(t, srv)

		_, err := g.ListTasks(context.Background(), "pve1", port.TaskListOptions{})
		require.NoError(t, err)
		assert.Equal(t, "", rec.query)
	})
}

// TestGateway_GetTaskLogLimitQuery verifies the limit query parameter.
func TestGateway_GetTaskLogLimitQuery(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[]}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.GetTaskLog(context.Background(), "pve1", "u", 50)
	require.NoError(t, err)
	assert.Equal(t, "limit=50", rec.query)

	// limit <= 0 -> no query.
	_, err = g.GetTaskLog(context.Background(), "pve1", "u", 0)
	require.NoError(t, err)
	assert.Equal(t, "", rec.query)
}

// TestGateway_EmptyDataForGetters verifies null data surfaces EmptyDataError for
// single-entity getters.
func TestGateway_EmptyDataForGetters(t *testing.T) {
	getters := []struct {
		name string
		call func(*Gateway) error
	}{
		{"GetNodeStatus", func(g *Gateway) error { _, e := g.GetNodeStatus(context.Background(), "pve1"); return e }},
		{"GetVMConfig", func(g *Gateway) error { _, e := g.GetVMConfig(context.Background(), "pve1", 100); return e }},
		{"GetVMStatus", func(g *Gateway) error { _, e := g.GetVMStatus(context.Background(), "pve1", 100); return e }},
		{"GetLXCConfig", func(g *Gateway) error { _, e := g.GetLXCConfig(context.Background(), "pve1", 200); return e }},
		{"GetLXCStatus", func(g *Gateway) error { _, e := g.GetLXCStatus(context.Background(), "pve1", 200); return e }},
		{"GetStorageStatus", func(g *Gateway) error { _, e := g.GetStorageStatus(context.Background(), "pve1", "local"); return e }},
		{"GetTaskStatus", func(g *Gateway) error { _, e := g.GetTaskStatus(context.Background(), "pve1", "u"); return e }},
		{"GetPVEVersion", func(g *Gateway) error { _, e := g.GetPVEVersion(context.Background()); return e }},
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
		{"ListNodes", func(g *Gateway) error { _, e := g.ListNodes(context.Background()); return e }},
		{"GetClusterStatus", func(g *Gateway) error { _, e := g.GetClusterStatus(context.Background()); return e }},
		{"GetClusterResources", func(g *Gateway) error { _, e := g.GetClusterResources(context.Background()); return e }},
		{"ListVMs", func(g *Gateway) error { _, e := g.ListVMs(context.Background(), "pve1"); return e }},
		{"ListLXCs", func(g *Gateway) error { _, e := g.ListLXCs(context.Background(), "pve1"); return e }},
		{"ListStorage", func(g *Gateway) error { _, e := g.ListStorage(context.Background()); return e }},
		{"ListNetwork", func(g *Gateway) error { _, e := g.ListNetwork(context.Background(), "pve1"); return e }},
		{"ListTasks", func(g *Gateway) error {
			_, e := g.ListTasks(context.Background(), "pve1", port.TaskListOptions{})
			return e
		}},
		{"GetTaskLog", func(g *Gateway) error { _, e := g.GetTaskLog(context.Background(), "pve1", "u", 0); return e }},
	}

	for _, tt := range lists {
		t.Run(tt.name, func(t *testing.T) {
			_, err := runMethod(t, `{"data":null}`, tt.call)
			require.NoError(t, err)
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
		err := g.decode([]byte(`{"data":null,"errors":{"node":"missing"},"message":"boom"}`), &model.Node{})
		var ae *APIError
		require.ErrorAs(t, err, &ae)
		assert.NotEmpty(t, ae.Messages)
	})

	t.Run("message only maps to APIError", func(t *testing.T) {
		err := g.decode([]byte(`{"data":null,"message":"oops"}`), &model.Node{})
		var ae *APIError
		require.ErrorAs(t, err, &ae)
		assert.Contains(t, strings.Join(ae.Messages, " "), "oops")
	})

	t.Run("invalid json", func(t *testing.T) {
		err := g.decode([]byte(`not json`), &model.Node{})
		var ue *UpstreamError
		assert.ErrorAs(t, err, &ue)
	})

	t.Run("null data for struct maps to EmptyDataError", func(t *testing.T) {
		err := g.decode([]byte(`{"data":null}`), &model.Node{})
		var ee *EmptyDataError
		assert.ErrorAs(t, err, &ee)
	})

	t.Run("null data for slice is empty not error", func(t *testing.T) {
		out := []model.Node{}
		require.NoError(t, g.decode([]byte(`{"data":null}`), &out))
		assert.Empty(t, out)
	})

	t.Run("missing data key for struct maps to EmptyDataError", func(t *testing.T) {
		err := g.decode([]byte(`{}`), &model.Node{})
		var ee *EmptyDataError
		assert.ErrorAs(t, err, &ee)
	})

	t.Run("unmarshal target error", func(t *testing.T) {
		err := g.decode([]byte(`{"data":123}`), &model.Node{})
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

// TestIsSlicePtr covers the helper branches.
func TestIsSlicePtr(t *testing.T) {
	assert.False(t, isSlicePtr(nil))
	var node model.Node
	assert.False(t, isSlicePtr(&node))
	nodes := []model.Node{}
	assert.True(t, isSlicePtr(&nodes))
}

// TestDoRetryCtxCanceled verifies the retry loop honors ctx cancellation.
func TestDoRetryCtxCanceled(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
	g, _ := newTestGateway(t, srv)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out []model.Node
	err := g.do(ctx, http.MethodGet, "/nodes", nil, &out)
	assert.Error(t, err)
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
		assert.NotNil(t, c.Transport)
	})

	t.Run("invalid CA file errors", func(t *testing.T) {
		_, err := buildClient(Config{CACertPath: "/nonexistent/ca.pem"})
		require.Error(t, err)
	})

	t.Run("no CACertPath returns unchanged client", func(t *testing.T) {
		hc := &http.Client{}
		c, err := buildClient(Config{HTTPClient: hc})
		require.NoError(t, err)
		assert.Same(t, hc, c)
	})

	t.Run("nil HTTPClient gets default", func(t *testing.T) {
		c, err := buildClient(Config{})
		require.NoError(t, err)
		assert.NotNil(t, c)
	})
}

// TestCloneTransport covers cloning a custom transport.
func TestCloneTransport(t *testing.T) {
	cl := cloneTransport(&http.Client{Transport: &http.Transport{MaxIdleConns: 5}})
	assert.NotNil(t, cl)
	cl2 := cloneTransport(&http.Client{}) // non-*http.Transport default path
	assert.NotNil(t, cl2)
}

// TestDoDefaultBackoff covers the default backoff when RetryBackoff <= 0.
func TestDoDefaultBackoff(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
	g := NewGateway(Config{
		Endpoint:    srv.URL,
		TokenSource: token.NewStaticTokenSource("user@pam!tokenid=secret"),
		HTTPClient:  srv.Client(),
		Logger:      noopLogger{},
		Retries:     0, // backoff <= 0 triggers the default
	})
	var out []model.Node
	err := g.do(context.Background(), http.MethodGet, "/nodes", nil, &out)
	assert.Error(t, err)
}

// TestDoEmptyEndpoint covers the config-error branch in doOnce.
func TestDoEmptyEndpoint(t *testing.T) {
	g := NewGateway(Config{TokenSource: token.NewStaticTokenSource("tok")})
	var out []model.Node
	err := g.do(context.Background(), http.MethodGet, "/nodes", nil, &out)
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
	var out []model.Node
	err := g.do(context.Background(), http.MethodGet, "/nodes", nil, &out)
	assert.Error(t, err)
	assert.Equal(t, 1, rec.count)
}

// TestGateway_ListMethodErrorBranches exercises the error return path of every
// method that otherwise only succeeds, to reach full statement coverage.
func TestGateway_ListMethodErrorBranches(t *testing.T) {
	calls := []struct {
		name string
		call func(*Gateway) error
	}{
		{"GetClusterStatus", func(g *Gateway) error { _, e := g.GetClusterStatus(context.Background()); return e }},
		{"GetClusterResources", func(g *Gateway) error { _, e := g.GetClusterResources(context.Background()); return e }},
		{"GetNextID", func(g *Gateway) error { _, e := g.GetNextID(context.Background()); return e }},
		{"ListVMs", func(g *Gateway) error { _, e := g.ListVMs(context.Background(), "pve1"); return e }},
		{"ListLXCs", func(g *Gateway) error { _, e := g.ListLXCs(context.Background(), "pve1"); return e }},
		{"ListStorage", func(g *Gateway) error { _, e := g.ListStorage(context.Background()); return e }},
		{"ListNetwork", func(g *Gateway) error { _, e := g.ListNetwork(context.Background(), "pve1"); return e }},
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

// TestDoOnceTLSErr covers the g.tlsErr branch in doOnce (misconfigured CA).
func TestDoOnceTLSErr(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[]}`, rec)
	g := NewGateway(Config{
		Endpoint:    srv.URL,
		TokenSource: token.NewStaticTokenSource("tok"),
		HTTPClient:  srv.Client(),
		CACertPath:  "/nonexistent/ca.pem", // sets g.tlsErr at construction
	})
	var out []model.Node
	err := g.do(context.Background(), http.MethodGet, "/nodes", nil, &out)
	require.Error(t, err)
	var ue *UpstreamError
	assert.ErrorAs(t, err, &ue)
	assert.Equal(t, "config", ue.Op)
}

// TestBuildClientInvalidPEM covers the "no valid CA certificates" branch.
func TestBuildClientInvalidPEM(t *testing.T) {
	path := filepath.Join(t.TempDir(), "notca.pem")
	require.NoError(t, os.WriteFile(path, []byte("this is not a PEM CA"), 0o600))
	_, err := buildClient(Config{HTTPClient: &http.Client{}, CACertPath: path})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no valid CA certificates")
}

// TestGateway_GetTaskLogErrorBranch covers GetTaskLog's error return.
func TestGateway_GetTaskLogErrorBranch(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
	g, _ := newTestGateway(t, srv)
	_, err := g.GetTaskLog(context.Background(), "pve1", "u", 10)
	require.Error(t, err)
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
