package pve

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"resty.dev/v3"

	"github.com/teran/mcp-proxmox/domain/model"
	"github.com/teran/mcp-proxmox/domain/port"
)

// apiPrefix is the path prefix of the Proxmox VE REST API. It is appended to
// cfg.Endpoint (which is the bare host, e.g. https://pve.example.com:8006) to
// build the full request URL. See SPEC.md §1 / §4.5.
const apiPrefix = "/api2/json"

// defaultTimeout is applied when cfg.HTTPClient is nil.
const defaultTimeout = 30 * time.Second

// Config is the configuration of the Proxmox VE HTTP client. See SPEC.md §4.5.
type Config struct {
	Endpoint     string           // e.g. https://pve.example.com:8006
	TokenSource  port.TokenSource // returns the Authorization header
	HTTPClient   *http.Client     // timeout 30s; transport replaceable for tests (Resty derives its transport from this)
	CACertPath   string           // optional custom CA (PEM); empty = system roots
	Logger       port.AppLogger
	Retries      int           // 2 (read-only idempotent requests)
	RetryBackoff time.Duration // 200ms
}

// Gateway is an implementation of port.PVEGateway on top of the PVE REST API.
type Gateway struct {
	cfg    Config
	client *resty.Client  // outbound HTTP via resty.dev/v3 (G9); nil-safe via NewGateway
	logger port.CtxLogger // ctx-aware (session_id/request_id) logging; nil-safe
	tlsErr error          // set when CACertPath is configured but fails to load/parse
}

// NewGateway creates a Gateway. When CACertPath is set, a client whose
// transport carries the custom CA in its TLS RootCAs is prepared (TLS
// verification is always on — there is no insecure_skip_verify switch).
func NewGateway(cfg Config) *Gateway {
	g := &Gateway{cfg: cfg, logger: port.NewCtxLogger(cfg.Logger)}
	g.client, g.tlsErr = buildClient(cfg)
	return g
}

// buildClient returns the effective resty client to use. Resty is the only
// outbound HTTP mechanism (SPEC.md G9/N23 — never a bare http.Client{} /
// http.NewRequestWithContext). When CACertPath is empty the provided
// HTTPClient's timeout/transport (or a default 30s transport) is used, so
// tests can inject an httptest client. When CACertPath is set, the transport's
// TLS RootCAs are replaced by a pool that includes the custom CA (TLS
// verification stays on).
func buildClient(cfg Config) (*resty.Client, error) {
	rc := resty.New()
	timeout := defaultTimeout
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.HTTPClient != nil {
		if cfg.HTTPClient.Timeout != 0 {
			timeout = cfg.HTTPClient.Timeout
		}
		if t, ok := cfg.HTTPClient.Transport.(*http.Transport); ok && t != nil {
			tr = t.Clone()
		}
	}
	rc.SetTimeout(timeout)
	rc.SetTransport(tr)

	if cfg.CACertPath != "" {
		pool, err := systemOrNewCertPool()
		if err != nil {
			return nil, err
		}
		// #nosec G304 -- cfg.CACertPath is a configuration-supplied CA path, not
		// attacker-controlled input. Its content is never logged.
		pem, err := os.ReadFile(cfg.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("pve: read CA cert %q: %w", cfg.CACertPath, err)
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("pve: no valid CA certificates found in %q", cfg.CACertPath)
		}

		tlsConf := tr.TLSClientConfig
		if tlsConf == nil {
			tlsConf = &tls.Config{MinVersion: tls.VersionTLS12}
		}
		tlsConf = tlsConf.Clone()
		tlsConf.RootCAs = pool
		tlsConf.MinVersion = tls.VersionTLS12
		tr.TLSClientConfig = tlsConf
		rc.SetTransport(tr)
	}
	return rc, nil
}

// systemOrNewCertPool returns the system cert pool, or a fresh empty pool when
// the system pool is unavailable.
func systemOrNewCertPool() (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		return x509.NewCertPool(), nil
	}
	return pool, nil
}

// do executes a PVE request, decoding the JSON envelope into out. Query filters
// are appended when query is non-empty. Reads are retried (idempotent) per
// SPEC.md §7.5: Retries additional attempts after the first with linear
// backoff, retrying only *UpstreamError; sentinels, *APIError and
// *EmptyDataError abort immediately. The loop honors ctx cancellation.
func (g *Gateway) do(ctx context.Context, method, path string, query url.Values, out any) error {
	attempts := g.cfg.Retries + 1
	if g.cfg.Retries < 0 {
		attempts = 1
	}
	backoff := g.cfg.RetryBackoff
	if backoff <= 0 {
		backoff = 200 * time.Millisecond
	}

	var lastErr error
	for i := 0; i < attempts; i++ {
		if i > 0 {
			timer := time.NewTimer(backoff * time.Duration(i))
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		lastErr = g.doOnce(ctx, method, path, query, out)
		if lastErr == nil {
			return nil
		}
		var upErr *UpstreamError
		if !errors.As(lastErr, &upErr) {
			// Sentinel / API / empty-data errors are not retried.
			return lastErr
		}
	}
	return lastErr
}

// doOnce performs a single HTTP request (no retry). It builds the URL from
// cfg.Endpoint + apiPrefix + path, injects the Authorization header via the
// token source, honors the configured TLS, and maps the response per SPEC.md
// §7.3. The request is made via resty.dev/v3 (SPEC.md G9).
func (g *Gateway) doOnce(ctx context.Context, method, path string, query url.Values, out any) error {
	if g.cfg.Endpoint == "" {
		return &UpstreamError{Op: "config", Err: errors.New("pve: empty endpoint")}
	}
	if g.tlsErr != nil {
		return &UpstreamError{Op: "config", Err: g.tlsErr}
	}

	// Token source failure is a misconfiguration surfaced before any request is
	// sent (SPEC.md §7.4) and is reported in the ErrUnauthorized family.
	header, err := g.cfg.TokenSource.AuthorizationHeader(ctx)
	if err != nil {
		return fmt.Errorf("%w: pve: token source: %v", port.ErrUnauthorized, err)
	}

	u := strings.TrimRight(g.cfg.Endpoint, "/") + apiPrefix + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	req := g.client.R().SetContext(ctx).
		SetHeader("Authorization", header).
		SetHeader("Accept", "application/json")
	// Forward the per-request correlation ID to the upstream (SPEC.md §5.5);
	// never the token.
	if rid, ok := port.RequestIDFromContext(ctx); ok && rid != "" {
		req.SetHeader("X-Request-ID", rid)
	}

	resp, err := req.Execute(method, u)
	if err != nil {
		return &UpstreamError{Op: "transport", Err: err}
	}
	body := resp.Bytes()

	if g.cfg.Logger != nil {
		// Request logging carries the upstream HTTP status code plus request
		// metrics so both the success (2xx) and error paths show them. Method +
		// URL path only; the Authorization header value is never logged (SPEC.md
		// §5.4). Logged via the ctx-aware logger so lines carry the same
		// session_id/request_id as the tool and error lines.
		g.logger.Debugf(ctx, "pve request: %s %s upstream_http_status_code=%d duration_ms=%d in_bytes=%d out_bytes=%d",
			method, path, resp.StatusCode(), resp.Duration().Milliseconds(), 0, len(body))
	}

	if resp.StatusCode() >= http.StatusBadRequest {
		// Surface the real Proxmox error message at debug level (truncated) so
		// the journal reveals the exact reason (e.g. an auth/privilege issue).
		// The body of an error response never contains the API token.
		g.logger.Debugf(ctx, "pve error response %d: %s", resp.StatusCode(), truncate(body))
		return g.mapHTTPError(resp.StatusCode(), body)
	}
	return g.decode(body, out)
}

// decode parses the {"data": ...} envelope into out, mapping empty/null data for
// single-entity getters to EmptyDataError and an error envelope to APIError.
func (g *Gateway) decode(body []byte, out any) error {
	var env struct {
		Data    json.RawMessage   `json:"data"`
		Errors  map[string]string `json:"errors"`
		Message string            `json:"message"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return &UpstreamError{Op: "decode", Err: err}
	}

	if len(env.Errors) > 0 || env.Message != "" {
		return &APIError{Messages: errorMessages(env.Errors, env.Message)}
	}
	if out == nil {
		return nil
	}

	if len(env.Data) == 0 || string(env.Data) == "null" {
		if !isSlicePtr(out) {
			// A single-entity getter received no data: surface an explicit
			// not-found style error rather than a silently-zero struct.
			return &EmptyDataError{Op: "data"}
		}
		// Empty data for a list is simply an empty list.
		return nil
	}
	if err := json.Unmarshal(env.Data, out); err != nil {
		return &UpstreamError{Op: "decode", Err: err}
	}
	return nil
}

// mapHTTPError maps an HTTP status to the port sentinel / typed error family
// (SPEC.md §7.3).
func (g *Gateway) mapHTTPError(status int, body []byte) error {
	switch status {
	case http.StatusUnauthorized:
		return &port.HTTPStatusError{Status: status, Err: port.ErrUnauthorized}
	case http.StatusForbidden:
		return &port.HTTPStatusError{Status: status, Err: port.ErrForbidden}
	case http.StatusNotFound:
		return &port.HTTPStatusError{Status: status, Err: port.ErrNotFound}
	}
	if status >= 500 {
		return &UpstreamError{Op: "http", Status: status, Err: fmt.Errorf("pve: HTTP status %d", status)}
	}
	// Other 4xx: surface the Proxmox error envelope as an APIError.
	msgs := extractErrorMessages(body)
	return &APIError{Messages: msgs}
}

// extractErrorMessages pulls {"errors": {...}} and {"message": ...} strings from
// an error response body.
func extractErrorMessages(body []byte) []string {
	var env struct {
		Errors  map[string]string `json:"errors"`
		Message string            `json:"message"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil
	}
	return errorMessages(env.Errors, env.Message)
}

// errorMessages flattens the API error envelope into a stable message list.
func errorMessages(errorsMap map[string]string, message string) []string {
	msgs := make([]string, 0, len(errorsMap)+1)
	for k, v := range errorsMap {
		msgs = append(msgs, k+": "+v)
	}
	if message != "" {
		msgs = append(msgs, message)
	}
	return msgs
}

// isSlicePtr reports whether out is a pointer to a slice, i.e. a list getter.
func isSlicePtr(v any) bool {
	if v == nil {
		return false
	}
	rv := reflect.ValueOf(v)
	return rv.Kind() == reflect.Pointer && rv.Elem().Kind() == reflect.Slice
}

// ListNodes returns the cluster nodes.
func (g *Gateway) ListNodes(ctx context.Context) ([]model.Node, error) {
	var out []model.Node
	if err := g.do(ctx, http.MethodGet, "/nodes", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetNodeStatus returns the status of a single node.
func (g *Gateway) GetNodeStatus(ctx context.Context, node string) (*model.NodeStatus, error) {
	var out model.NodeStatus
	if err := g.do(ctx, http.MethodGet, "/nodes/"+node+"/status", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetClusterStatus returns the cluster status.
func (g *Gateway) GetClusterStatus(ctx context.Context) ([]model.ClusterStatus, error) {
	var out []model.ClusterStatus
	if err := g.do(ctx, http.MethodGet, "/cluster/status", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetClusterResources returns the cluster resources.
func (g *Gateway) GetClusterResources(ctx context.Context) ([]model.ClusterResource, error) {
	var out []model.ClusterResource
	if err := g.do(ctx, http.MethodGet, "/cluster/resources", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetNextID returns the next free VMID.
func (g *Gateway) GetNextID(ctx context.Context) (string, error) {
	var out string
	if err := g.do(ctx, http.MethodGet, "/cluster/nextid", nil, &out); err != nil {
		return "", err
	}
	return out, nil
}

// ListVMs returns the QEMU VMs on a node.
func (g *Gateway) ListVMs(ctx context.Context, node string) ([]model.VM, error) {
	var out []model.VM
	if err := g.do(ctx, http.MethodGet, "/nodes/"+node+"/qemu", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetVMConfig returns the config of a VM.
func (g *Gateway) GetVMConfig(ctx context.Context, node string, vmid int) (*model.VMConfig, error) {
	var out model.VMConfig
	if err := g.do(ctx, http.MethodGet, "/nodes/"+node+"/qemu/"+strconv.Itoa(vmid)+"/config", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetVMStatus returns the status of a VM.
func (g *Gateway) GetVMStatus(ctx context.Context, node string, vmid int) (*model.VMStatus, error) {
	var out model.VMStatus
	if err := g.do(ctx, http.MethodGet, "/nodes/"+node+"/qemu/"+strconv.Itoa(vmid)+"/status/current", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListLXCs returns the LXC containers on a node.
func (g *Gateway) ListLXCs(ctx context.Context, node string) ([]model.LXC, error) {
	var out []model.LXC
	if err := g.do(ctx, http.MethodGet, "/nodes/"+node+"/lxc", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetLXCConfig returns the config of a container.
func (g *Gateway) GetLXCConfig(ctx context.Context, node string, vmid int) (*model.LXCConfig, error) {
	var out model.LXCConfig
	if err := g.do(ctx, http.MethodGet, "/nodes/"+node+"/lxc/"+strconv.Itoa(vmid)+"/config", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetLXCStatus returns the status of a container.
func (g *Gateway) GetLXCStatus(ctx context.Context, node string, vmid int) (*model.LXCStatus, error) {
	var out model.LXCStatus
	if err := g.do(ctx, http.MethodGet, "/nodes/"+node+"/lxc/"+strconv.Itoa(vmid)+"/status/current", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListStorage returns the cluster storage.
func (g *Gateway) ListStorage(ctx context.Context) ([]model.Storage, error) {
	var out []model.Storage
	if err := g.do(ctx, http.MethodGet, "/storage", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetStorageStatus returns the status of a storage on a node.
func (g *Gateway) GetStorageStatus(ctx context.Context, node, storage string) (*model.StorageStatus, error) {
	var out model.StorageStatus
	if err := g.do(ctx, http.MethodGet, "/nodes/"+node+"/storage/"+storage+"/status", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListNetwork returns the network interfaces of a node.
func (g *Gateway) ListNetwork(ctx context.Context, node string) ([]model.NetworkInterface, error) {
	var out []model.NetworkInterface
	if err := g.do(ctx, http.MethodGet, "/nodes/"+node+"/network", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListTasks returns the tasks on a node, filtered by opts.
func (g *Gateway) ListTasks(ctx context.Context, node string, opts port.TaskListOptions) ([]model.Task, error) {
	var out []model.Task
	var q url.Values
	if opts.VMD != 0 || opts.TypeFilter != "" || opts.Since != 0 || opts.Until != 0 || opts.Source != "" || opts.Limit != 0 {
		q = url.Values{}
		if opts.VMD != 0 {
			q.Set("vmid", strconv.Itoa(opts.VMD))
		}
		if opts.TypeFilter != "" {
			q.Set("typefilter", opts.TypeFilter)
		}
		if opts.Since != 0 {
			q.Set("since", strconv.FormatInt(opts.Since, 10))
		}
		if opts.Until != 0 {
			q.Set("until", strconv.FormatInt(opts.Until, 10))
		}
		if opts.Source != "" {
			q.Set("source", opts.Source)
		}
		if opts.Limit != 0 {
			q.Set("limit", strconv.Itoa(opts.Limit))
		}
	}
	if err := g.do(ctx, http.MethodGet, "/nodes/"+node+"/tasks", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetTaskStatus returns the status of a task by UPID.
func (g *Gateway) GetTaskStatus(ctx context.Context, node, upid string) (*model.TaskStatus, error) {
	var out model.TaskStatus
	if err := g.do(ctx, http.MethodGet, "/nodes/"+node+"/tasks/"+upid+"/status", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTaskLog returns the log of a task by UPID.
func (g *Gateway) GetTaskLog(ctx context.Context, node, upid string, limit int) ([]model.TaskLogEntry, error) {
	var out []model.TaskLogEntry
	var q url.Values
	if limit > 0 {
		q = url.Values{"limit": []string{strconv.Itoa(limit)}}
	}
	if err := g.do(ctx, http.MethodGet, "/nodes/"+node+"/tasks/"+upid+"/log", q, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetPVEVersion returns the PVE version info.
func (g *Gateway) GetPVEVersion(ctx context.Context) (*model.PVEVersion, error) {
	var out model.PVEVersion
	if err := g.do(ctx, http.MethodGet, "/version", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// truncate bounds an error-response body for debug logging, keeping the log
// line compact and free of unbounded payloads.
func truncate(b []byte) string {
	const max = 300
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "..."
}
