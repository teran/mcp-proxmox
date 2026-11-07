package ceph

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
// cfg.Endpoint (the bare PVE host) to build the full request URL, exactly like
// adapter/pve — Ceph rides the same PVE endpoint (SPEC.md §3.4 / §4.5).
const apiPrefix = "/api2/json"

// defaultTimeout is applied when cfg.HTTPClient is nil.
const defaultTimeout = 30 * time.Second

// defaultBackoff is the linear retry delay used when cfg.RetryBackoff is zero.
const defaultBackoff = 200 * time.Millisecond

// statusServerError is the first HTTP status that indicates an upstream server
// error (5xx), which the gateway retries.
const statusServerError = 500

// Config is the configuration of the Ceph HTTP client. It is fed from the PVE
// backend configuration (endpoint + token + CA) — Ceph rides the PVE API token
// and is enabled together with the PVE backend (SPEC.md §3.4).
type Config struct {
	Endpoint     string           // e.g. https://pve.example.com:8006
	TokenSource  port.TokenSource // returns the PVE Authorization header
	HTTPClient   *http.Client     // timeout 2m; transport replaceable for tests
	CACertPath   string           // optional custom CA (PEM); empty = system roots
	Logger       port.AppLogger
	Retries      int           // 2 (read-only idempotent requests)
	RetryBackoff time.Duration // 200ms
	// UserAgent is the value sent as the User-Agent header on outbound requests.
	UserAgent string
}

// Gateway is an implementation of port.CephGateway on top of the PVE-managed
// Ceph subset of the PVE REST API.
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

// buildClient returns the effective resty client to use. It mirrors
// adapter/pve: Resty is the only outbound HTTP mechanism (SPEC.md G9/N23), TLS
// verification stays on, and an explicit Accept-Encoding: identity is sent so
// resty v3 does not drop the response body.
func buildClient(cfg Config) (*resty.Client, error) {
	client := resty.New()
	if cfg.UserAgent != "" {
		client.SetHeader("User-Agent", cfg.UserAgent)
	}
	timeout := defaultTimeout
	transport := &http.Transport{}
	if t, ok := http.DefaultTransport.(*http.Transport); ok {
		transport = t.Clone()
	}
	if cfg.HTTPClient != nil {
		if cfg.HTTPClient.Timeout != 0 {
			timeout = cfg.HTTPClient.Timeout
		}
		if t, ok := cfg.HTTPClient.Transport.(*http.Transport); ok && t != nil {
			transport = t.Clone()
		}
	}
	client.SetHeader("Accept-Encoding", "identity")
	client.SetTimeout(timeout)
	client.SetTransport(transport)

	if cfg.CACertPath != "" {
		pool, err := systemOrNewCertPool()
		if err != nil {
			return nil, err
		}
		// #nosec G304 -- cfg.CACertPath is a configuration-supplied CA path, not
		// attacker-controlled input. Its content is never logged.
		pem, err := os.ReadFile(cfg.CACertPath)
		if err != nil {
			return nil, fmt.Errorf("ceph: read CA cert %q: %w", cfg.CACertPath, err)
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("ceph: no valid CA certificates found in %q", cfg.CACertPath)
		}

		tlsConf := transport.TLSClientConfig
		if tlsConf == nil {
			tlsConf = &tls.Config{MinVersion: tls.VersionTLS12}
		}
		tlsConf = tlsConf.Clone()
		tlsConf.RootCAs = pool
		tlsConf.MinVersion = tls.VersionTLS12
		transport.TLSClientConfig = tlsConf
		client.SetTransport(transport)
	}

	return client, nil
}

// systemOrNewCertPool returns the system cert pool, or a fresh empty pool when
// the system pool is unavailable.
func systemOrNewCertPool() (*x509.CertPool, error) {
	pool, err := x509.SystemCertPool()
	if err == nil && pool != nil {
		return pool, nil
	}

	return x509.NewCertPool(), nil
}

// do executes a request, decoding the JSON envelope into out. Reads are
// retried (idempotent) per SPEC.md §7.5: Retries additional attempts after the
// first with linear backoff, retrying only *UpstreamError; sentinels, *APIError
// and *EmptyDataError abort immediately. The loop honors ctx cancellation.
func (g *Gateway) do(ctx context.Context, method, path string, query url.Values, out any) error {
	attempts := g.cfg.Retries + 1
	if g.cfg.Retries < 0 {
		attempts = 1
	}
	backoff := g.cfg.RetryBackoff
	if backoff <= 0 {
		backoff = defaultBackoff
	}

	var lastErr error
	for i := range attempts {
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
		if _, isUpstream := errors.AsType[*UpstreamError](lastErr); !isUpstream {
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
	return g.doOnceReq(ctx, method, path, query, nil, out)
}

// doOnceWithForm is like doOnce but sends an application/x-www-form-urlencoded
// body. It is used by the mutation surface (create/update/set), which takes its
// parameters in the form body.
func (g *Gateway) doOnceWithForm(ctx context.Context, method, path string, form url.Values, out any) error {
	return g.doOnceReq(ctx, method, path, nil, form, out)
}

// doOnceReq performs a single HTTP request (no retry). query and form are
// mutually exclusive: read requests pass query parameters, mutation requests
// pass a form-encoded body.
func (g *Gateway) doOnceReq(ctx context.Context, method, path string, query, form url.Values, out any) error {
	body, err := g.doOnceBody(ctx, method, path, query, form)
	if err != nil {
		return err
	}

	return g.decode(body, out)
}

// doOnceRaw performs a single HTTP request (no retry) and returns the raw
// response body instead of decoding the standard {"data": ...} envelope. It is
// used by creates (lenient UPID extraction) and deletes (2xx is a success).
func (g *Gateway) doOnceRaw(ctx context.Context, method, path string, form url.Values) ([]byte, error) {
	return g.doOnceBody(ctx, method, path, nil, form)
}

// doOnceBody performs a single HTTP request (no retry) and returns the raw
// response body. It shares the request/error-mapping logic with doOnceReq but
// skips the {"data": ...} decode. The token is never logged (SPEC.md §5.4).
func (g *Gateway) doOnceBody(ctx context.Context, method, path string, query, form url.Values) ([]byte, error) {
	if g.cfg.Endpoint == "" {
		return nil, &UpstreamError{Op: "config", Err: errors.New("ceph: empty endpoint")}
	}
	if g.tlsErr != nil {
		return nil, &UpstreamError{Op: "config", Err: g.tlsErr}
	}

	header, err := g.cfg.TokenSource.AuthorizationHeader(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: ceph: token source: %w", port.ErrUnauthorized, err)
	}

	requestURL := strings.TrimRight(g.cfg.Endpoint, "/") + apiPrefix + path
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}

	req := g.client.R().SetContext(ctx).
		SetHeader("Authorization", header).
		SetHeader("Accept", "application/json")
	if rid, ok := port.RequestIDFromContext(ctx); ok && rid != "" {
		req.SetHeader("X-Request-ID", rid)
	}
	if len(form) > 0 {
		req.SetFormDataFromValues(form)
	}

	resp, err := req.Execute(method, requestURL)
	if err != nil {
		return nil, &UpstreamError{Op: "transport", Err: err}
	}
	body := resp.Bytes()

	if g.cfg.Logger != nil {
		g.logger.Tracef(ctx, "ceph request: %s %s%s upstream_http_status_code=%d duration_ms=%d in_bytes=%d out_bytes=%d",
			method, apiPrefix, path, resp.StatusCode(), resp.Duration().Milliseconds(), 0, len(body))
	}

	if resp.StatusCode() >= http.StatusBadRequest {
		g.logger.Tracef(ctx, "ceph error response %d", resp.StatusCode())

		return nil, g.mapHTTPError(resp.StatusCode(), body)
	}

	return body, nil
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
	if status >= statusServerError {
		return &UpstreamError{Op: "http", Status: status, Err: fmt.Errorf("ceph: HTTP status %d", status)}
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

// doMutation executes a single mutation request (never retried — mutations are
// not idempotent and must not be replayed on a transient 5xx; SPEC.md §7.5).
func (g *Gateway) doMutation(ctx context.Context, method, path string, form url.Values, out any) error {
	return g.doOnceWithForm(ctx, method, path, form, out)
}

// nodeCephPath builds the per-node Ceph path prefix, escaping the node name so
// a value containing reserved path characters cannot inject extra segments.
func nodeCephPath(node string) string {
	return "/nodes/" + url.PathEscape(node) + "/ceph"
}

// upidFromCreateBody extracts the task UPID from a create/init response body.
// PVE returns it either as the standard envelope {"data":"UPID:..."} or as the
// flat {"upid":"UPID:..."}. An empty/null/absent data is NOT an error — the
// create succeeded but echoed no UPID, so "" is returned (SPEC.md §7.5).
func upidFromCreateBody(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var env struct {
		Data json.RawMessage `json:"data"`
		UPID string          `json:"upid"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return ""
	}
	if env.UPID != "" {
		return env.UPID
	}
	if len(env.Data) == 0 || string(env.Data) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(env.Data, &s); err == nil {
		return s
	}

	return ""
}

// --- Cluster-level reads ---

// GetClusterStatus returns the Ceph cluster status.
func (g *Gateway) GetClusterStatus(ctx context.Context) (*model.CephClusterStatus, error) {
	var out model.CephClusterStatus
	if err := g.do(ctx, http.MethodGet, "/cluster/ceph/status", nil, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// GetClusterConfig returns the Ceph cluster config.
func (g *Gateway) GetClusterConfig(ctx context.Context) ([]model.CephConfigEntry, error) {
	var out []model.CephConfigEntry
	if err := g.do(ctx, http.MethodGet, "/cluster/ceph/config", nil, &out); err != nil {
		return nil, err
	}

	return out, nil
}

// GetClusterLog returns the Ceph cluster log, capped at limit lines.
func (g *Gateway) GetClusterLog(ctx context.Context, limit int) ([]model.CephLogEntry, error) {
	var out []model.CephLogEntry
	query := url.Values{}
	if limit > 0 {
		query.Set("limit", strconv.Itoa(limit))
	}
	if err := g.do(ctx, http.MethodGet, "/cluster/ceph/log", query, &out); err != nil {
		return nil, err
	}

	return out, nil
}

// GetClusterMetadata returns the Ceph cluster metadata.
func (g *Gateway) GetClusterMetadata(ctx context.Context) (*model.CephClusterMetadata, error) {
	var out model.CephClusterMetadata
	if err := g.do(ctx, http.MethodGet, "/cluster/ceph/metadata", nil, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// --- Node-level reads ---

// GetNodeStatus returns the Ceph status of a node.
func (g *Gateway) GetNodeStatus(ctx context.Context, node string) (*model.CephNodeStatus, error) {
	var out model.CephNodeStatus
	if err := g.do(ctx, http.MethodGet, nodeCephPath(node)+"/status", nil, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// GetNodeConfig returns the Ceph config of a node.
func (g *Gateway) GetNodeConfig(ctx context.Context, node string) ([]model.CephConfigEntry, error) {
	var out []model.CephConfigEntry
	if err := g.do(ctx, http.MethodGet, nodeCephPath(node)+"/config", nil, &out); err != nil {
		return nil, err
	}

	return out, nil
}

// GetNodeConfigDB returns the Ceph config-db entries of a node.
func (g *Gateway) GetNodeConfigDB(ctx context.Context, node string) ([]model.CephConfigEntry, error) {
	var out []model.CephConfigEntry
	if err := g.do(ctx, http.MethodGet, nodeCephPath(node)+"/configdb", nil, &out); err != nil {
		return nil, err
	}

	return out, nil
}

// GetNodeFlags returns the Ceph flags of a node.
func (g *Gateway) GetNodeFlags(ctx context.Context, node string) ([]model.CephFlag, error) {
	var out []model.CephFlag
	if err := g.do(ctx, http.MethodGet, nodeCephPath(node)+"/flags", nil, &out); err != nil {
		return nil, err
	}

	return out, nil
}

// GetNodeMetadata returns the Ceph metadata of a node.
func (g *Gateway) GetNodeMetadata(ctx context.Context, node string) (*model.CephNodeMetadata, error) {
	var out model.CephNodeMetadata
	if err := g.do(ctx, http.MethodGet, nodeCephPath(node)+"/metadata", nil, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// GetNodeCrush returns the Ceph CRUSH map of a node.
func (g *Gateway) GetNodeCrush(ctx context.Context, node string) (*model.CephCrushMap, error) {
	var out model.CephCrushMap
	if err := g.do(ctx, http.MethodGet, nodeCephPath(node)+"/crush", nil, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// GetNodeRules returns the Ceph CRUSH rules of a node.
func (g *Gateway) GetNodeRules(ctx context.Context, node string) ([]model.CephCrushRule, error) {
	var out []model.CephCrushRule
	if err := g.do(ctx, http.MethodGet, nodeCephPath(node)+"/rules", nil, &out); err != nil {
		return nil, err
	}

	return out, nil
}

// ListPools returns the Ceph pools on a node.
func (g *Gateway) ListPools(ctx context.Context, node string) ([]model.CephPool, error) {
	var out []model.CephPool
	if err := g.do(ctx, http.MethodGet, nodeCephPath(node)+"/pool", nil, &out); err != nil {
		return nil, err
	}

	return out, nil
}

// ListOSDs returns the Ceph OSDs on a node.
func (g *Gateway) ListOSDs(ctx context.Context, node string) ([]model.CephOSD, error) {
	var out []model.CephOSD
	if err := g.do(ctx, http.MethodGet, nodeCephPath(node)+"/osd", nil, &out); err != nil {
		return nil, err
	}

	return out, nil
}

// ListMons returns the Ceph monitors on a node.
func (g *Gateway) ListMons(ctx context.Context, node string) ([]model.CephMon, error) {
	var out []model.CephMon
	if err := g.do(ctx, http.MethodGet, nodeCephPath(node)+"/mon", nil, &out); err != nil {
		return nil, err
	}

	return out, nil
}

// ListMgrs returns the Ceph managers on a node.
func (g *Gateway) ListMgrs(ctx context.Context, node string) ([]model.CephMgr, error) {
	var out []model.CephMgr
	if err := g.do(ctx, http.MethodGet, nodeCephPath(node)+"/mgr", nil, &out); err != nil {
		return nil, err
	}

	return out, nil
}

// --- Mutations (gated by EnableMutations, never retried) ---

// CreateCrushRule creates a Ceph CRUSH rule via POST /nodes/{node}/ceph/rules
// and returns the task UPID. Empty/null echoed data is not an error.
func (g *Gateway) CreateCrushRule(ctx context.Context, node string, req model.CephCrushRuleCreateRequest) (string, error) {
	body, err := g.doOnceRaw(ctx, http.MethodPost, nodeCephPath(node)+"/rules", crushRuleForm(req))
	if err != nil {
		return "", err
	}

	return upidFromCreateBody(body), nil
}

// crushRuleForm maps a CephCrushRuleCreateRequest into the form body.
func crushRuleForm(req model.CephCrushRuleCreateRequest) url.Values {
	form := url.Values{}
	if req.Name != "" {
		form.Set("name", req.Name)
	}
	if req.Type != "" {
		form.Set("type", req.Type)
	}
	if req.MinSize != 0 {
		form.Set("min_size", strconv.Itoa(req.MinSize))
	}
	if req.MaxSize != 0 {
		form.Set("max_size", strconv.Itoa(req.MaxSize))
	}
	if req.Root != "" {
		form.Set("root", req.Root)
	}
	if req.OSD != "" {
		form.Set("osd", req.OSD)
	}

	return form
}

// DeleteCrushRule deletes a Ceph CRUSH rule via DELETE /nodes/{node}/ceph/rules/{name}.
// name is URL-escaped as a path segment. It is a destructive mutation, never
// retried; PVE may reply with an empty 2xx body, so doOnceRaw is used.
func (g *Gateway) DeleteCrushRule(ctx context.Context, node, name string) error {
	_, err := g.doOnceRaw(ctx, http.MethodDelete, nodeCephPath(node)+"/rules/"+url.PathEscape(name), nil)

	return err
}

// CreatePool creates a Ceph pool via POST /nodes/{node}/ceph/pool and returns
// the task UPID. Empty/null echoed data is not an error.
func (g *Gateway) CreatePool(ctx context.Context, node string, req model.CephPoolCreateRequest) (string, error) {
	body, err := g.doOnceRaw(ctx, http.MethodPost, nodeCephPath(node)+"/pool", poolForm(req))
	if err != nil {
		return "", err
	}

	return upidFromCreateBody(body), nil
}

// poolForm maps a CephPoolCreateRequest into the form body.
func poolForm(req model.CephPoolCreateRequest) url.Values {
	form := url.Values{}
	if req.Name != "" {
		form.Set("name", req.Name)
	}
	if req.Size != 0 {
		form.Set("size", strconv.Itoa(req.Size))
	}
	if req.MinSize != 0 {
		form.Set("min_size", strconv.Itoa(req.MinSize))
	}
	if req.PGNum != 0 {
		form.Set("pg_num", strconv.Itoa(req.PGNum))
	}
	if req.PGAutoscaleMode != "" {
		form.Set("pg_autoscale_mode", req.PGAutoscaleMode)
	}

	return form
}

// UpdatePool updates a Ceph pool via PUT /nodes/{node}/ceph/pool/{name}.
// name is URL-escaped as a path segment.
func (g *Gateway) UpdatePool(ctx context.Context, node, name string, req model.CephPoolUpdateRequest) error {
	form := url.Values{}
	if req.Size != 0 {
		form.Set("size", strconv.Itoa(req.Size))
	}
	if req.MinSize != 0 {
		form.Set("min_size", strconv.Itoa(req.MinSize))
	}
	if req.PGNum != 0 {
		form.Set("pg_num", strconv.Itoa(req.PGNum))
	}
	if req.PGAutoscaleMode != "" {
		form.Set("pg_autoscale_mode", req.PGAutoscaleMode)
	}

	return g.doMutation(ctx, http.MethodPut, nodeCephPath(node)+"/pool/"+url.PathEscape(name), form, nil)
}

// DeletePool deletes a Ceph pool via DELETE /nodes/{node}/ceph/pool/{name}.
// name is URL-escaped as a path segment. It is a destructive mutation, never
// retried; PVE may reply with an empty 2xx body, so doOnceRaw is used.
func (g *Gateway) DeletePool(ctx context.Context, node, name string) error {
	_, err := g.doOnceRaw(ctx, http.MethodDelete, nodeCephPath(node)+"/pool/"+url.PathEscape(name), nil)

	return err
}

// CreateOSD creates a Ceph OSD via POST /nodes/{node}/ceph/osd and returns the
// task UPID. Empty/null echoed data is not an error.
func (g *Gateway) CreateOSD(ctx context.Context, node string, req model.CephOSDCreateRequest) (string, error) {
	body, err := g.doOnceRaw(ctx, http.MethodPost, nodeCephPath(node)+"/osd", osdForm(req))
	if err != nil {
		return "", err
	}

	return upidFromCreateBody(body), nil
}

// osdForm maps a CephOSDCreateRequest into the form body.
func osdForm(req model.CephOSDCreateRequest) url.Values {
	form := url.Values{}
	if req.OSD != "" {
		form.Set("osd", req.OSD)
	}

	return form
}

// DeleteOSD deletes a Ceph OSD via DELETE /nodes/{node}/ceph/osd/{osdid}.
// osdid is a numeric path segment. It is a destructive mutation, never retried;
// PVE may reply with an empty 2xx body, so doOnceRaw is used.
func (g *Gateway) DeleteOSD(ctx context.Context, node string, osdid int) error {
	_, err := g.doOnceRaw(ctx, http.MethodDelete, nodeCephPath(node)+"/osd/"+strconv.Itoa(osdid), nil)

	return err
}

// CreateMon creates a Ceph monitor via POST /nodes/{node}/ceph/mon and returns
// the task UPID. Empty/null echoed data is not an error.
func (g *Gateway) CreateMon(ctx context.Context, node string, req model.CephMonCreateRequest) (string, error) {
	body, err := g.doOnceRaw(ctx, http.MethodPost, nodeCephPath(node)+"/mon", monForm(req))
	if err != nil {
		return "", err
	}

	return upidFromCreateBody(body), nil
}

// monForm maps a CephMonCreateRequest into the form body.
func monForm(req model.CephMonCreateRequest) url.Values {
	form := url.Values{}
	if req.MonAddr != "" {
		form.Set("mon_addr", req.MonAddr)
	}

	return form
}

// DeleteMon deletes a Ceph monitor via DELETE /nodes/{node}/ceph/mon/{monid}.
// monid is URL-escaped as a path segment. It is a destructive mutation, never
// retried; PVE may reply with an empty 2xx body, so doOnceRaw is used.
func (g *Gateway) DeleteMon(ctx context.Context, node, monid string) error {
	_, err := g.doOnceRaw(ctx, http.MethodDelete, nodeCephPath(node)+"/mon/"+url.PathEscape(monid), nil)

	return err
}

// CreateMgr creates a Ceph manager via POST /nodes/{node}/ceph/mgr and returns
// the task UPID. Empty/null echoed data is not an error.
func (g *Gateway) CreateMgr(ctx context.Context, node string, req model.CephMgrCreateRequest) (string, error) {
	body, err := g.doOnceRaw(ctx, http.MethodPost, nodeCephPath(node)+"/mgr", mgrForm(req))
	if err != nil {
		return "", err
	}

	return upidFromCreateBody(body), nil
}

// mgrForm maps a CephMgrCreateRequest into the form body.
func mgrForm(req model.CephMgrCreateRequest) url.Values {
	form := url.Values{}
	if req.ID != "" {
		form.Set("id", req.ID)
	}

	return form
}

// SetFlags sets Ceph flags via PUT /nodes/{node}/ceph/flags. Each flag in
// Values is sent as its own form field.
func (g *Gateway) SetFlags(ctx context.Context, node string, req model.CephFlagsSetRequest) error {
	form := url.Values{}
	for k, v := range req.Values {
		form.Set(k, v)
	}

	return g.doMutation(ctx, http.MethodPut, nodeCephPath(node)+"/flags", form, nil)
}

// SetConfig sets a Ceph config option via PUT /nodes/{node}/ceph/config.
func (g *Gateway) SetConfig(ctx context.Context, node string, req model.CephConfigSetRequest) error {
	form := url.Values{}
	if req.Name != "" {
		form.Set("name", req.Name)
	}
	if req.Value != "" {
		form.Set("value", req.Value)
	}

	return g.doMutation(ctx, http.MethodPut, nodeCephPath(node)+"/config", form, nil)
}

// InitCeph initializes Ceph on a node via POST /nodes/{node}/ceph/init and
// returns the task UPID. Empty/null echoed data is not an error.
func (g *Gateway) InitCeph(ctx context.Context, node string, req model.CephInitRequest) (string, error) {
	form := url.Values{}
	if req.Network != "" {
		form.Set("network", req.Network)
	}
	if req.ClusterNetwork != "" {
		form.Set("cluster_network", req.ClusterNetwork)
	}
	body, err := g.doOnceRaw(ctx, http.MethodPost, nodeCephPath(node)+"/init", form)
	if err != nil {
		return "", err
	}

	return upidFromCreateBody(body), nil
}

// DestroyCeph destroys Ceph on a node via DELETE /nodes/{node}/ceph/destroy.
// It is a destructive mutation, never retried; PVE may reply with an empty 2xx
// body, so doOnceRaw is used. The optional force flag is carried in the query
// string — DELETE has no meaningful form body and resty drops one, so the query
// string is the only way the parameter reaches the server.
func (g *Gateway) DestroyCeph(ctx context.Context, node string, req model.CephDestroyRequest) error {
	path := nodeCephPath(node) + "/destroy"
	if req.Force {
		path += "?force=1"
	}
	_, err := g.doOnceRaw(ctx, http.MethodDelete, path, nil)

	return err
}
