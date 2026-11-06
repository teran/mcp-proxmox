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

// defaultBackoff is the linear retry delay used when cfg.RetryBackoff is zero.
const defaultBackoff = 200 * time.Millisecond

// statusServerError is the first HTTP status that indicates an upstream server
// error (5xx), which the gateway retries.
const statusServerError = 500

// Config is the configuration of the Proxmox VE HTTP client. See SPEC.md §4.5.
type Config struct {
	Endpoint     string           // e.g. https://pve.example.com:8006
	TokenSource  port.TokenSource // returns the Authorization header
	HTTPClient   *http.Client     // timeout 30s; transport replaceable for tests (Resty derives its transport from this)
	CACertPath   string           // optional custom CA (PEM); empty = system roots
	Logger       port.AppLogger
	Retries      int           // 2 (read-only idempotent requests)
	RetryBackoff time.Duration // 200ms
	// UserAgent is the value sent as the User-Agent header on outbound requests.
	UserAgent string
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
	// Send an explicit Accept-Encoding so resty v3 (resty.dev/v3, currently
	// only available as pre-release v3.0.0-rc.4) does not manage the header via
	// its own auto-decompress middleware. In auto mode resty v3 rc.4 drops the
	// response body of the Proxmox APIs (a 200 with a plain Content-Length but
	// no Content-Encoding), returning 0 bytes. Setting Accept-Encoding
	// explicitly (identity = no compression) makes resty pass the body through
	// unchanged. TLS verification stays on; no protocol downgrade.
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
			return nil, fmt.Errorf("pve: read CA cert %q: %w", cfg.CACertPath, err)
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("pve: no valid CA certificates found in %q", cfg.CACertPath)
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
// body. It is used by the mutation surface (create/resize/migrate/ha/backup),
// which takes its parameters in the form body rather than the query string.
func (g *Gateway) doOnceWithForm(ctx context.Context, method, path string, form url.Values, out any) error {
	return g.doOnceReq(ctx, method, path, nil, form, out)
}

// doOnceReq performs a single HTTP request (no retry). query and form are
// mutually exclusive: read requests pass query parameters, mutation requests
// pass a form-encoded body. See doOnce / doOnceWithForm.
func (g *Gateway) doOnceReq(ctx context.Context, method, path string, query, form url.Values, out any) error {
	body, err := g.doOnceBody(ctx, method, path, query, form)
	if err != nil {
		return err
	}

	return g.decode(body, out)
}

// doOnceRaw performs a single HTTP request (no retry) and returns the raw
// response body instead of decoding the standard {"data": ...} envelope. It is
// used by response shapes that do not fit the envelope decode (e.g. the
// download-url endpoint, which may return {"upid": ...}).
func (g *Gateway) doOnceRaw(ctx context.Context, method, path string, form url.Values) ([]byte, error) {
	return g.doOnceBody(ctx, method, path, nil, form)
}

// doOnceBody performs a single HTTP request (no retry) and returns the raw
// response body. It shares the request/error-mapping logic with doOnceReq but
// skips the {"data": ...} decode. The token is never logged (SPEC.md §5.4).
func (g *Gateway) doOnceBody(ctx context.Context, method, path string, query, form url.Values) ([]byte, error) {
	if g.cfg.Endpoint == "" {
		return nil, &UpstreamError{Op: "config", Err: errors.New("pve: empty endpoint")}
	}
	if g.tlsErr != nil {
		return nil, &UpstreamError{Op: "config", Err: g.tlsErr}
	}

	// Token source failure is a misconfiguration surfaced before any request is
	// sent (SPEC.md §7.4) and is reported in the ErrUnauthorized family.
	header, err := g.cfg.TokenSource.AuthorizationHeader(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: pve: token source: %w", port.ErrUnauthorized, err)
	}

	requestURL := strings.TrimRight(g.cfg.Endpoint, "/") + apiPrefix + path
	if len(query) > 0 {
		requestURL += "?" + query.Encode()
	}

	req := g.client.R().SetContext(ctx).
		SetHeader("Authorization", header).
		SetHeader("Accept", "application/json")
	// Forward the per-request correlation ID to the upstream (SPEC.md §5.5);
	// never the token.
	if rid, ok := port.RequestIDFromContext(ctx); ok && rid != "" {
		req.SetHeader("X-Request-ID", rid)
	}
	// Mutation form body (never sent for reads).
	if len(form) > 0 {
		req.SetFormDataFromValues(form)
	}

	resp, err := req.Execute(method, requestURL)
	if err != nil {
		return nil, &UpstreamError{Op: "transport", Err: err}
	}
	body := resp.Bytes()

	if g.cfg.Logger != nil {
		// Request logging carries the upstream HTTP status code plus request
		// metrics so both the success (2xx) and error paths show them. Method +
		// URL path (including the API prefix) only; the Authorization header
		// value is never logged (SPEC.md §5.4). Logged at trace level via the
		// ctx-aware logger so lines carry the same session_id/request_id as the
		// tool and error lines, and stay hidden unless LOG_LEVEL=trace.
		g.logger.Tracef(ctx, "pve request: %s %s%s upstream_http_status_code=%d duration_ms=%d in_bytes=%d out_bytes=%d",
			method, apiPrefix, path, resp.StatusCode(), resp.Duration().Milliseconds(), 0, len(body))
	}

	if resp.StatusCode() >= http.StatusBadRequest {
		// Only the status is surfaced (at trace level); the error response body
		// is never written to the journal — it could carry secret-adjacent data,
		// and the requirement is to log no response bodies (SPEC.md §5.4).
		g.logger.Tracef(ctx, "pve error response %d", resp.StatusCode())

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
	if err := g.do(ctx, http.MethodGet, "/nodes/"+url.PathEscape(node)+"/status", nil, &out); err != nil {
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

// ListBackupJobs returns the vzdump backup jobs (schedules) configured on the
// cluster via GET /cluster/backup.
func (g *Gateway) ListBackupJobs(ctx context.Context) ([]model.BackupJob, error) {
	var out []model.BackupJob
	if err := g.do(ctx, http.MethodGet, "/cluster/backup", nil, &out); err != nil {
		return nil, err
	}

	return out, nil
}

// CreateBackupJob creates a vzdump backup job via POST /cluster/backup. The
// response `data` is the new job's ID, but PVE may also return empty or `{}`
// data on success (the job is still created). The raw `data` is captured and
// the job ID extracted leniently, so a success with no ID does not error.
func (g *Gateway) CreateBackupJob(ctx context.Context, req model.CreateBackupJobRequest) (string, error) {
	var raw json.RawMessage
	if err := g.doMutation(ctx, http.MethodPost, "/cluster/backup", backupJobForm(req), &raw); err != nil {
		return "", err
	}

	return backupJobIDFromData(raw), nil
}

// backupJobIDFromData extracts the backup-job ID from a create/update response
// `data`. PVE returns the job ID in several shapes — a bare JSON string
// (`"backup-100"`), an object with an `id` key (`{"id":"backup-100"}`), an
// object wrapping a nested `data.id` (`{"data":{"id":"x"}}`), or nothing at all
// (`null`, `{}`, empty). It resolves the ID from each form, returning "" when
// none is present (a success with no echoed ID is not an error).
func backupJobIDFromData(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}

	// Form 1: bare JSON string.
	var id string
	if err := json.Unmarshal(raw, &id); err == nil && id != "" {
		return id
	}

	// Form 2: object with an `id` key.
	var obj struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil && obj.ID != "" {
		return obj.ID
	}

	// Form 3: nested envelope {"data":{"id":"x"}}.
	var nested struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &nested); err == nil {
		return nested.Data.ID
	}

	return ""
}

// UpdateBackupJob updates a vzdump backup job via PUT /cluster/backup/{id}. The
// response `data` is the job's ID, but PVE may also return empty or `{}` data
// on success (the job is still updated). The raw `data` is captured and the job
// ID extracted leniently, so a success with no ID does not error — mirroring
// CreateBackupJob. id is URL-escaped so it is safe inside the path.
func (g *Gateway) UpdateBackupJob(ctx context.Context, id string, req model.CreateBackupJobRequest) (string, error) {
	var raw json.RawMessage
	if err := g.doMutation(ctx, http.MethodPut, "/cluster/backup/"+url.PathEscape(id), backupJobForm(req), &raw); err != nil {
		return "", err
	}

	return backupJobIDFromData(raw), nil
}

// DeleteBackupJob deletes a vzdump backup job via DELETE /cluster/backup/{id}.
// id is URL-escaped so it is safe inside the path. Mutations are never retried.
// PVE may reply to a successful delete with an empty 2xx body (no JSON
// envelope), so the raw body path is used: any 2xx is a success and the body is
// not decoded.
func (g *Gateway) DeleteBackupJob(ctx context.Context, id string) error {
	_, err := g.doOnceRaw(ctx, http.MethodDelete, "/cluster/backup/"+url.PathEscape(id), nil)

	return err
}

// backupJobForm builds the form-encoded mutation body for create/update of a
// vzdump backup job. Empty/zero fields are omitted (X02); Enabled is a pointer
// so an explicit false can be sent to disable a job.
func backupJobForm(req model.CreateBackupJobRequest) url.Values {
	form := url.Values{}
	if req.ID != "" {
		form.Set("id", req.ID)
	}
	if req.All {
		form.Set("all", "1")
	}
	if req.VMID != "" {
		form.Set("vmid", req.VMID)
	}
	form.Set("storage", req.Storage)
	form.Set("schedule", req.Schedule)
	if req.Mode != "" {
		form.Set("mode", req.Mode)
	}
	if req.Compress != "" {
		form.Set("compress", req.Compress)
	}
	if req.Enabled != nil {
		if *req.Enabled {
			form.Set("enabled", "1")
		} else {
			form.Set("enabled", "0")
		}
	}
	if req.NotesTemplate != "" {
		form.Set("notes-template", req.NotesTemplate)
	}
	if req.Comment != "" {
		form.Set("comment", req.Comment)
	}
	if req.Node != "" {
		form.Set("node", req.Node)
	}

	return form
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
	if err := g.do(ctx, http.MethodGet, nodePath(node)+"/qemu", nil, &out); err != nil {
		return nil, err
	}

	return out, nil
}

// GetVMConfig returns the config of a VM.
func (g *Gateway) GetVMConfig(ctx context.Context, node string, vmid int) (*model.VMConfig, error) {
	var out model.VMConfig
	if err := g.do(ctx, http.MethodGet, vmPath(node, vmid)+"/config", nil, &out); err != nil {
		return nil, err
	}

	// PVE does not echo the VMID in the config response body (it is carried only
	// in the request path), so populate it from the argument to make the result
	// self-describing.
	out.VMID = vmid

	return &out, nil
}

// GetVMStatus returns the status of a VM.
func (g *Gateway) GetVMStatus(ctx context.Context, node string, vmid int) (*model.VMStatus, error) {
	var out model.VMStatus
	if err := g.do(ctx, http.MethodGet, vmPath(node, vmid)+"/status/current", nil, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// ListLXCs returns the LXC containers on a node.
func (g *Gateway) ListLXCs(ctx context.Context, node string) ([]model.LXC, error) {
	var out []model.LXC
	if err := g.do(ctx, http.MethodGet, nodePath(node)+"/lxc", nil, &out); err != nil {
		return nil, err
	}

	return out, nil
}

// GetLXCConfig returns the config of a container.
func (g *Gateway) GetLXCConfig(ctx context.Context, node string, vmid int) (*model.LXCConfig, error) {
	var out model.LXCConfig
	if err := g.do(ctx, http.MethodGet, lxcPath(node, vmid)+"/config", nil, &out); err != nil {
		return nil, err
	}

	// PVE does not echo the VMID in the container config response body (it is
	// carried only in the request path), so populate it from the argument to
	// make the result self-describing — mirroring GetVMConfig.
	out.VMID = vmid

	return &out, nil
}

// GetLXCStatus returns the status of a container.
func (g *Gateway) GetLXCStatus(ctx context.Context, node string, vmid int) (*model.LXCStatus, error) {
	var out model.LXCStatus
	if err := g.do(ctx, http.MethodGet, lxcPath(node, vmid)+"/status/current", nil, &out); err != nil {
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
	path := nodePath(node) + "/storage/" + url.PathEscape(storage) + "/status"
	if err := g.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}

	// PVE does not echo the storage name in the status response body (it is
	// carried only in the request path), so populate it from the argument to
	// make the result self-describing.
	out.Storage = storage

	return &out, nil
}

// ListStorageContent returns the content of a storage on a node
// (GET /nodes/{node}/storage/{storage}/content) filtered by opts. It is
// read-only (idempotent) and retried via the standard do path.
func (g *Gateway) ListStorageContent(
	ctx context.Context, node, storage string, opts port.StorageContentListOptions,
) ([]model.StorageContent, error) {
	var out []model.StorageContent
	path := nodePath(node) + "/storage/" + url.PathEscape(storage) + "/content"
	if err := g.do(ctx, http.MethodGet, path, buildStorageContentQuery(opts), &out); err != nil {
		return nil, err
	}

	return out, nil
}

// buildStorageContentQuery maps the storage-content filters into a URL query
// string. It returns an empty (non-nil) url.Values when no filter is set, which
// the gateway renders as no query string at all.
func buildStorageContentQuery(opts port.StorageContentListOptions) url.Values {
	query := url.Values{}
	if opts.Content != "" {
		query.Set("content", opts.Content)
	}
	if opts.VMID != 0 {
		query.Set("vmid", strconv.Itoa(opts.VMID))
	}

	return query
}

// DownloadURL downloads a file from a URL into a storage on a node via
// POST /nodes/{node}/storage/{storage}/download-url. The response is a task
// UPID. PVE returns it in two shapes — {"data":"UPID:..."} or {"upid":"UPID:..."}
// — which upidFromDownloadResponse handles leniently. It is a mutation, so it is
// never retried (doOnceRaw).
func (g *Gateway) DownloadURL(
	ctx context.Context, node, storage string, req model.StorageDownloadURLRequest,
) (string, error) {
	path := nodePath(node) + "/storage/" + url.PathEscape(storage) + "/download-url"
	body, err := g.doOnceRaw(ctx, http.MethodPost, path, buildDownloadURLForm(req))
	if err != nil {
		return "", err
	}

	return upidFromDownloadResponse(body)
}

// buildDownloadURLForm maps a StorageDownloadURLRequest into the form-encoded
// mutation body. content/filename/url are always sent; verify-certificates is
// ALWAYS sent too — "1" when true and "0" when false — so the caller can
// explicitly override PVE's default rather than silently omitting it.
func buildDownloadURLForm(req model.StorageDownloadURLRequest) url.Values {
	form := url.Values{}
	form.Set("content", req.Content)
	form.Set("filename", req.Filename)
	form.Set("url", req.URL)
	if req.VerifyCertificates {
		form.Set("verify-certificates", "1")
	} else {
		form.Set("verify-certificates", "0")
	}

	return form
}

// upidFromDownloadResponse resolves the task UPID from a download-url response
// body. PVE returns it either as the standard envelope {"data":"UPID:..."} or as
// the flat {"upid":"UPID:..."} (which the standard envelope decode would treat
// as empty data). It returns an EmptyDataError when neither shape is present.
func upidFromDownloadResponse(body []byte) (string, error) {
	var env struct {
		Data json.RawMessage `json:"data"`
		UPID string          `json:"upid"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return "", &UpstreamError{Op: "decode", Err: err}
	}
	if env.UPID != "" {
		return env.UPID, nil
	}
	if len(env.Data) > 0 && string(env.Data) != "null" {
		var upid string
		if err := json.Unmarshal(env.Data, &upid); err == nil && upid != "" {
			return upid, nil
		}
	}

	return "", &EmptyDataError{Op: "data"}
}

// ListNetwork returns the network interfaces of a node.
func (g *Gateway) ListNetwork(ctx context.Context, node string) ([]model.NetworkInterface, error) {
	var out []model.NetworkInterface
	if err := g.do(ctx, http.MethodGet, "/nodes/"+url.PathEscape(node)+"/network", nil, &out); err != nil {
		return nil, err
	}

	return out, nil
}

// ListTasks returns the tasks on a node, filtered by opts.
func (g *Gateway) ListTasks(ctx context.Context, node string, opts port.TaskListOptions) ([]model.Task, error) {
	var out []model.Task
	if err := g.do(ctx, http.MethodGet, nodePath(node)+"/tasks", buildTaskListQuery(opts), &out); err != nil {
		return nil, err
	}

	return out, nil
}

// buildTaskListQuery maps the task-list filters into a URL query string. It
// returns an empty (non-nil) url.Values when no filter is set, which the gateway
// renders as no query string at all.
func buildTaskListQuery(opts port.TaskListOptions) url.Values {
	query := url.Values{}
	if opts.VMD != 0 {
		query.Set("vmid", strconv.Itoa(opts.VMD))
	}
	if opts.TypeFilter != "" {
		query.Set("typefilter", opts.TypeFilter)
	}
	if opts.Since != 0 {
		query.Set("since", strconv.FormatInt(opts.Since, 10))
	}
	if opts.Until != 0 {
		query.Set("until", strconv.FormatInt(opts.Until, 10))
	}
	if opts.Source != "" {
		query.Set("source", opts.Source)
	}
	if opts.Limit != 0 {
		query.Set("limit", strconv.Itoa(opts.Limit))
	}

	return query
}

// GetTaskStatus returns the status of a task by UPID.
func (g *Gateway) GetTaskStatus(ctx context.Context, node, upid string) (*model.TaskStatus, error) {
	var out model.TaskStatus
	path := nodePath(node) + "/tasks/" + url.PathEscape(upid) + "/status"
	if err := g.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return nil, err
	}

	return &out, nil
}

// GetTaskLog returns the log of a task by UPID.
func (g *Gateway) GetTaskLog(ctx context.Context, node, upid string, limit int) ([]model.TaskLogEntry, error) {
	var out []model.TaskLogEntry
	path := nodePath(node) + "/tasks/" + url.PathEscape(upid) + "/log"
	if limit > 0 {
		path += "?limit=" + strconv.Itoa(limit)
	}
	if err := g.do(ctx, http.MethodGet, path, nil, &out); err != nil {
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

// doMutation executes a single mutation request (never retried — mutations are
// not idempotent and must not be replayed on a transient 5xx; SPEC.md §7.5)
// with an optional form-encoded body.
func (g *Gateway) doMutation(ctx context.Context, method, path string, form url.Values, out any) error {
	return g.doOnceWithForm(ctx, method, path, form, out)
}

// CreateVM creates a QEMU VM via POST /nodes/{node}/qemu. The response `data`
// is the new VMID, but real PVE returns it in two different shapes depending on
// whether an explicit vmid was requested:
//
//   - synchronous create (auto-assigned vmid): data is the VMID as a JSON
//     string or number, e.g. {"data":"100"} or {"data":100};
//   - async/task-based create (explicit vmid): data is a task UPID, e.g.
//     {"data":"UPID:...:qmcreate:141:..."}.
//
// vmIDFromCreateResponse resolves the numeric VMID from either shape, falling
// back to the requested req.VMID when the VMID cannot be extracted (e.g. a
// UPID without a numeric id segment). An empty/absent data is treated as
// success returning the requested vmid (or 0 when none was requested).
func (g *Gateway) CreateVM(ctx context.Context, node string, req model.CreateVMRequest) (int, error) {
	form := buildCreateVMForm(req)
	var raw json.RawMessage
	if err := g.doMutation(ctx, http.MethodPost, nodePath(node)+"/qemu", form, &raw); err != nil {
		return 0, err
	}

	return vmIDFromCreateResponse(raw, req.VMID), nil
}

// CreateLXC creates an LXC container via POST /nodes/{node}/lxc. The response
// `data` is the new VMID, in the same two shapes CreateVM handles (a plain
// number/string, or a task UPID for the async vzcreate path), so the shared
// vmIDFromCreateResponse is reused. It is a mutation, so it is never retried
// (doMutation).
func (g *Gateway) CreateLXC(ctx context.Context, node string, req model.CreateLXCRequest) (int, error) {
	form := buildCreateLXCForm(req)
	var raw json.RawMessage
	if err := g.doMutation(ctx, http.MethodPost, nodePath(node)+"/lxc", form, &raw); err != nil {
		return 0, err
	}

	return vmIDFromCreateResponse(raw, req.VMID), nil
}

// buildCreateLXCForm maps a CreateLXCRequest into the form-encoded mutation
// body, omitting fields that were left at their zero value so the PVE defaults
// apply. ostemplate is always sent; unprivileged is set to "1" only when true.
// password and ssh-public-keys are sent verbatim (never logged).
func buildCreateLXCForm(req model.CreateLXCRequest) url.Values {
	form := url.Values{}
	if req.VMID != 0 {
		form.Set("vmid", strconv.Itoa(req.VMID))
	}
	form.Set("ostemplate", req.OSTemplate)
	if req.Hostname != "" {
		form.Set("hostname", req.Hostname)
	}
	if req.Storage != "" {
		form.Set("storage", req.Storage)
	}
	if req.RootFS != "" {
		form.Set("rootfs", req.RootFS)
	}
	if req.Cores != 0 {
		form.Set("cores", strconv.Itoa(req.Cores))
	}
	if req.Memory != 0 {
		form.Set("memory", strconv.Itoa(req.Memory))
	}
	if req.Swap != 0 {
		form.Set("swap", strconv.Itoa(req.Swap))
	}
	if req.Net0 != "" {
		form.Set("net0", req.Net0)
	}
	if req.Description != "" {
		form.Set("description", req.Description)
	}
	if req.Tags != "" {
		form.Set("tags", req.Tags)
	}
	if req.Unprivileged {
		form.Set("unprivileged", "1")
	}
	if req.Password != "" {
		form.Set("password", req.Password)
	}
	if req.SSHPublicKeys != "" {
		form.Set("ssh-public-keys", req.SSHPublicKeys)
	}

	return form
}

// vmIDFromCreateResponse resolves the created VMID from the create response
// `data`. It accepts a plain JSON number ({"data":100}), a numeric JSON string
// ({"data":"100"}), or a task UPID ({"data":"UPID:...:qmcreate:<vmid>:..."}),
// extracting the VMID from the UPID or falling back to fallback (the requested
// vmid). A zero/empty data returns the fallback (or 0).
func vmIDFromCreateResponse(raw json.RawMessage, fallback int) int {
	if len(raw) == 0 {
		return fallback
	}
	// A plain JSON number ({"data":100} or {"data":0}).
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n
	}
	// A JSON string: either the numeric VMID ("100") or a task UPID.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		s = strings.TrimSpace(s)
		if v, err := strconv.Atoi(s); err == nil {
			return v
		}
		if v := vmIDFromUPID(s); v > 0 {
			return v
		}
	}

	return fallback
}

// UPID layout constants. A create-task UPID is
// "UPID:<node>:<pid>:<pstart>:<starttime>:<type>:<id>:<user>:<uuid>"; the VMID
// is the <id> segment immediately after the task type.
const (
	upidMinSegments = 7 // node,pid,pstart,starttime,type,id (then user,uuid)
	upidIDIndex     = 6
)

// vmIDFromUPID extracts the VMID from a create-task UPID of the form
// "UPID:<node>:<pid>:<pstart>:<starttime>:<type>:<id>:<user>:<uuid>". For a
// per-VM task the <id> segment (index 6, immediately after the task type) is
// the numeric VMID. It returns 0 when the segment is absent or not numeric.
func vmIDFromUPID(upid string) int {
	parts := strings.Split(upid, ":")
	if len(parts) < upidMinSegments {
		return 0
	}
	n, err := strconv.Atoi(parts[upidIDIndex])
	if err != nil || n <= 0 {
		return 0
	}

	return n
}

// buildCreateVMForm maps a CreateVMRequest into the form-encoded mutation body,
// omitting fields that were left at their zero value so the PVE defaults apply.
func buildCreateVMForm(req model.CreateVMRequest) url.Values {
	form := url.Values{}
	if req.VMID != 0 {
		form.Set("vmid", strconv.Itoa(req.VMID))
	}
	if req.Name != "" {
		form.Set("name", req.Name)
	}
	if req.Cores != 0 {
		form.Set("cores", strconv.Itoa(req.Cores))
	}
	if req.Memory != 0 {
		form.Set("memory", strconv.Itoa(req.Memory))
	}
	if req.Sockets != 0 {
		form.Set("sockets", strconv.Itoa(req.Sockets))
	}
	if req.Ostype != "" {
		form.Set("ostype", req.Ostype)
	}
	if req.Net0 != "" {
		form.Set("net0", req.Net0)
	}
	if req.Scsi0 != "" {
		form.Set("scsi0", req.Scsi0)
	}
	if req.Ide2 != "" {
		form.Set("ide2", req.Ide2)
	}
	if req.Boot != "" {
		form.Set("boot", req.Boot)
	}
	if req.Balloon != 0 {
		form.Set("balloon", strconv.Itoa(req.Balloon))
	}
	if req.Description != "" {
		form.Set("description", req.Description)
	}
	if req.Tags != "" {
		form.Set("tags", req.Tags)
	}
	if req.Agent != 0 {
		form.Set("agent", strconv.Itoa(req.Agent))
	}
	if req.OnBoot != 0 {
		form.Set("onboot", strconv.Itoa(req.OnBoot))
	}
	if req.CPU != "" {
		form.Set("cpu", req.CPU)
	}
	if req.Machine != "" {
		form.Set("machine", req.Machine)
	}
	if req.VGA != "" {
		form.Set("vga", req.VGA)
	}

	return form
}

// ResizeVM resizes a VM disk via PUT /nodes/{node}/qemu/{vmid}/resize. The
// `size` form value is built from SizeGB as "±NG" in GiB (grow/shrink sign).
func (g *Gateway) ResizeVM(ctx context.Context, node string, vmid int, req model.ResizeVMRequest) error {
	form := url.Values{}
	form.Set("disk", req.Disk)
	form.Set("size", fmt.Sprintf("%+dG", req.SizeGB))

	return g.doMutation(ctx, http.MethodPut, "/nodes/"+url.PathEscape(node)+"/qemu/"+strconv.Itoa(vmid)+"/resize", form, nil)
}

// MigrateVM migrates a VM to another node via
// POST /nodes/{node}/qemu/{vmid}/migrate.
func (g *Gateway) MigrateVM(ctx context.Context, node string, vmid int, req model.MigrateVMRequest) error {
	form := url.Values{}
	form.Set("target", req.Target)
	if req.Online {
		form.Set("online", "1")
	}
	if req.WithLocalDisks {
		form.Set("with-local-disks", "1")
	}

	return g.doMutation(ctx, http.MethodPost, "/nodes/"+url.PathEscape(node)+"/qemu/"+strconv.Itoa(vmid)+"/migrate", form, nil)
}

// AddHAResource registers a new HA resource. Real PVE REJECTS `nodes` in the
// POST /cluster/ha/resources create body (nodes is only accepted on a later
// update), so the create POST carries only sid/type/comment. When preferred
// nodes are provided, a second PUT /cluster/ha/resources/{sid} sets them
// comma-joined. The two-step flow is idempotent: re-issuing the same create
// then nodes update converges to the same resource state.
func (g *Gateway) AddHAResource(ctx context.Context, req model.HAResourceRequest) error {
	create := url.Values{}
	create.Set("sid", req.SID)
	create.Set("type", req.Type)
	if req.Comment != "" {
		create.Set("comment", req.Comment)
	}
	if err := g.doMutation(ctx, http.MethodPost, "/cluster/ha/resources", create, nil); err != nil {
		return err
	}

	if len(req.Nodes) > 0 {
		update := url.Values{}
		update.Set("nodes", strings.Join(req.Nodes, ","))
		if err := g.doMutation(ctx, http.MethodPut, "/cluster/ha/resources/"+url.PathEscape(req.SID), update, nil); err != nil {
			return err
		}
	}

	return nil
}

// RemoveHAResource removes (unregisters) a HA resource by its SID via
// DELETE /cluster/ha/resources/{sid}. sid is URL-escaped as a path segment so
// a '/' cannot inject extra path segments (S11). Note url.PathEscape leaves
// ':' unescaped, so an SID like vm:100 stays vm:100 on the wire (as PVE
// expects), while vm:14/1 becomes vm:14%2F1. It is a destructive mutation:
// never retried (doMutation) and the request carries no body. A 404 for an
// unknown SID surfaces as the standard not-found/upstream error via the shared
// error mapping.
func (g *Gateway) RemoveHAResource(ctx context.Context, sid string) error {
	return g.doMutation(ctx, http.MethodDelete, "/cluster/ha/resources/"+url.PathEscape(sid), nil, nil)
}

// StartVMBackup starts a vzdump backup via POST /nodes/{node}/vzdump. The
// response `data` is the task UPID; it is wrapped in a Task so the tool can
// return a familiar shape (only UPID is populated from the create response).
func (g *Gateway) StartVMBackup(ctx context.Context, node string, req model.VMBackupRequest) (*model.Task, error) {
	form := url.Values{}
	form.Set("vmid", strconv.Itoa(req.VMID))
	form.Set("storage", req.Storage)
	form.Set("mode", req.Mode)
	if req.NotesTemplate != "" {
		form.Set("notes-template", req.NotesTemplate)
	}
	if req.Compress != "" {
		form.Set("compress", req.Compress)
	}

	var upid string
	if err := g.doMutation(ctx, http.MethodPost, "/nodes/"+url.PathEscape(node)+"/vzdump", form, &upid); err != nil {
		return nil, err
	}

	return &model.Task{UPID: upid}, nil
}

// nodePath builds the PVE node path prefix, escaping the node name so a value
// containing reserved path characters cannot inject extra segments.
func nodePath(node string) string {
	return "/nodes/" + url.PathEscape(node)
}

// vmPath builds the QEMU VM path prefix for a node + vmid. node is path-escaped
// so a value containing reserved path characters cannot inject extra segments.
func vmPath(node string, vmid int) string {
	return "/nodes/" + url.PathEscape(node) + "/qemu/" + strconv.Itoa(vmid)
}

// StartVM starts a QEMU VM via POST /nodes/{node}/qemu/{vmid}/status/start.
func (g *Gateway) StartVM(ctx context.Context, node string, vmid int) (*model.Task, error) {
	var upid string
	if err := g.doMutation(ctx, http.MethodPost, vmPath(node, vmid)+"/status/start", nil, &upid); err != nil {
		return nil, err
	}

	return &model.Task{UPID: upid}, nil
}

// StopVM stops a QEMU VM (hard power off) via
// POST /nodes/{node}/qemu/{vmid}/status/stop.
func (g *Gateway) StopVM(ctx context.Context, node string, vmid int, skiplock bool) (*model.Task, error) {
	form := url.Values{}
	if skiplock {
		form.Set("skiplock", "1")
	}
	var upid string
	if err := g.doMutation(ctx, http.MethodPost, vmPath(node, vmid)+"/status/stop", form, &upid); err != nil {
		return nil, err
	}

	return &model.Task{UPID: upid}, nil
}

// ShutdownVM gracefully shuts down a QEMU VM via
// POST /nodes/{node}/qemu/{vmid}/status/shutdown. forceStop requests an
// immediate stop if the graceful shutdown times out; timeout is the shutdown
// timeout in seconds.
func (g *Gateway) ShutdownVM(ctx context.Context, node string, vmid int, forceStop bool, timeout int) (*model.Task, error) {
	form := url.Values{}
	if forceStop {
		form.Set("forceStop", "1")
	}
	if timeout > 0 {
		form.Set("timeout", strconv.Itoa(timeout))
	}
	var upid string
	if err := g.doMutation(ctx, http.MethodPost, vmPath(node, vmid)+"/status/shutdown", form, &upid); err != nil {
		return nil, err
	}

	return &model.Task{UPID: upid}, nil
}

// RebootVM reboots a QEMU VM via
// POST /nodes/{node}/qemu/{vmid}/status/reboot. timeout is the reboot timeout
// in seconds.
func (g *Gateway) RebootVM(ctx context.Context, node string, vmid int, timeout int) (*model.Task, error) {
	form := url.Values{}
	if timeout > 0 {
		form.Set("timeout", strconv.Itoa(timeout))
	}
	var upid string
	if err := g.doMutation(ctx, http.MethodPost, vmPath(node, vmid)+"/status/reboot", form, &upid); err != nil {
		return nil, err
	}

	return &model.Task{UPID: upid}, nil
}

// ResetVM resets a QEMU VM (reboot without graceful shutdown) via
// POST /nodes/{node}/qemu/{vmid}/status/reset.
func (g *Gateway) ResetVM(ctx context.Context, node string, vmid int) (*model.Task, error) {
	var upid string
	if err := g.doMutation(ctx, http.MethodPost, vmPath(node, vmid)+"/status/reset", nil, &upid); err != nil {
		return nil, err
	}

	return &model.Task{UPID: upid}, nil
}

// SuspendVM suspends a QEMU VM via
// POST /nodes/{node}/qemu/{vmid}/status/suspend. todisk writes the guest RAM to
// disk for a full suspend-to-disk.
func (g *Gateway) SuspendVM(ctx context.Context, node string, vmid int, todisk bool) (*model.Task, error) {
	form := url.Values{}
	if todisk {
		form.Set("todisk", "1")
	}
	var upid string
	if err := g.doMutation(ctx, http.MethodPost, vmPath(node, vmid)+"/status/suspend", form, &upid); err != nil {
		return nil, err
	}

	return &model.Task{UPID: upid}, nil
}

// ResumeVM resumes a suspended QEMU VM via
// POST /nodes/{node}/qemu/{vmid}/status/resume.
func (g *Gateway) ResumeVM(ctx context.Context, node string, vmid int) (*model.Task, error) {
	var upid string
	if err := g.doMutation(ctx, http.MethodPost, vmPath(node, vmid)+"/status/resume", nil, &upid); err != nil {
		return nil, err
	}

	return &model.Task{UPID: upid}, nil
}

// DeleteVM deletes a QEMU VM via DELETE /nodes/{node}/qemu/{vmid}. The mutation
// parameters are carried in the query string (DELETE has no form body). purge
// removes the VM from any backup jobs and destroy-unreferenced-disks removes
// disks not referenced by the config.
func (g *Gateway) DeleteVM(
	ctx context.Context, node string, vmid int, purge, destroyUnreferencedDisks bool,
) (*model.Task, error) {
	query := url.Values{}
	if purge {
		query.Set("purge", "1")
	}
	if destroyUnreferencedDisks {
		query.Set("destroy-unreferenced-disks", "1")
	}
	var upid string
	// Mutations are never retried: use the single-attempt doOnce path with the
	// query string (doMutation sends a form body, which DELETE does not use).
	if err := g.doOnce(ctx, http.MethodDelete, vmPath(node, vmid), query, &upid); err != nil {
		return nil, err
	}

	return &model.Task{UPID: upid}, nil
}

// ListVMSnapshots lists the snapshots of a QEMU VM via
// GET /nodes/{node}/qemu/{vmid}/snapshot. Read-only.
func (g *Gateway) ListVMSnapshots(ctx context.Context, node string, vmid int) ([]model.Snapshot, error) {
	var out []model.Snapshot
	if err := g.do(ctx, http.MethodGet, vmPath(node, vmid)+"/snapshot", nil, &out); err != nil {
		return nil, err
	}

	return out, nil
}

// CloneVM clones a QEMU VM via POST /nodes/{node}/qemu/{vmid}/clone.
func (g *Gateway) CloneVM(ctx context.Context, node string, vmid int, req model.CloneVMRequest) (*model.Task, error) {
	form := url.Values{}
	form.Set("newid", strconv.Itoa(req.NewID))
	if req.Name != "" {
		form.Set("name", req.Name)
	}
	if req.Full {
		form.Set("full", "1")
	}
	if req.Storage != "" {
		form.Set("storage", req.Storage)
	}
	if req.Pool != "" {
		form.Set("pool", req.Pool)
	}
	if req.Description != "" {
		form.Set("description", req.Description)
	}
	if req.Format != "" {
		form.Set("format", req.Format)
	}
	if req.Snapname != "" {
		form.Set("snapname", req.Snapname)
	}
	var upid string
	if err := g.doMutation(ctx, http.MethodPost, vmPath(node, vmid)+"/clone", form, &upid); err != nil {
		return nil, err
	}

	return &model.Task{UPID: upid}, nil
}

// CreateVMSnapshot creates a snapshot of a QEMU VM via
// POST /nodes/{node}/qemu/{vmid}/snapshot.
func (g *Gateway) CreateVMSnapshot(
	ctx context.Context, node string, vmid int, req model.SnapshotCreateRequest,
) (*model.Task, error) {
	form := url.Values{}
	form.Set("snapname", req.Snapname)
	if req.VMState {
		form.Set("vmstate", "1")
	}
	if req.Description != "" {
		form.Set("description", req.Description)
	}
	var upid string
	if err := g.doMutation(ctx, http.MethodPost, vmPath(node, vmid)+"/snapshot", form, &upid); err != nil {
		return nil, err
	}

	return &model.Task{UPID: upid}, nil
}

// DeleteVMSnapshot deletes a snapshot of a QEMU VM via
// DELETE /nodes/{node}/qemu/{vmid}/snapshot/{snapname}.
func (g *Gateway) DeleteVMSnapshot(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
	var upid string
	path := vmPath(node, vmid) + "/snapshot/" + url.PathEscape(snapname)
	if err := g.doMutation(ctx, http.MethodDelete, path, nil, &upid); err != nil {
		return nil, err
	}

	return &model.Task{UPID: upid}, nil
}

// RollbackVMSnapshot rolls a QEMU VM back to a snapshot via
// POST /nodes/{node}/qemu/{vmid}/snapshot/{snapname}/rollback.
func (g *Gateway) RollbackVMSnapshot(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
	var upid string
	path := vmPath(node, vmid) + "/snapshot/" + url.PathEscape(snapname) + "/rollback"
	if err := g.doMutation(ctx, http.MethodPost, path, nil, &upid); err != nil {
		return nil, err
	}

	return &model.Task{UPID: upid}, nil
}

// lxcPath builds the LXC container path prefix for a node + vmid. node is
// path-escaped so a value containing reserved path characters cannot inject
// extra segments.
func lxcPath(node string, vmid int) string {
	return "/nodes/" + url.PathEscape(node) + "/lxc/" + strconv.Itoa(vmid)
}

// StartLXC starts an LXC container via
// POST /nodes/{node}/lxc/{vmid}/status/start.
func (g *Gateway) StartLXC(ctx context.Context, node string, vmid int) (*model.Task, error) {
	var upid string
	if err := g.doMutation(ctx, http.MethodPost, lxcPath(node, vmid)+"/status/start", nil, &upid); err != nil {
		return nil, err
	}

	return &model.Task{UPID: upid}, nil
}

// StopLXC stops an LXC container (hard stop) via
// POST /nodes/{node}/lxc/{vmid}/status/stop.
func (g *Gateway) StopLXC(ctx context.Context, node string, vmid int, skiplock, forceStop bool) (*model.Task, error) {
	form := url.Values{}
	if skiplock {
		form.Set("skiplock", "1")
	}
	if forceStop {
		form.Set("forceStop", "1")
	}
	var upid string
	if err := g.doMutation(ctx, http.MethodPost, lxcPath(node, vmid)+"/status/stop", form, &upid); err != nil {
		return nil, err
	}

	return &model.Task{UPID: upid}, nil
}

// ShutdownLXC gracefully shuts down an LXC container via
// POST /nodes/{node}/lxc/{vmid}/status/shutdown. forceStop requests an
// immediate stop if the graceful shutdown times out; timeout is the shutdown
// timeout in seconds.
func (g *Gateway) ShutdownLXC(ctx context.Context, node string, vmid int, forceStop bool, timeout int) (*model.Task, error) {
	form := url.Values{}
	if forceStop {
		form.Set("forceStop", "1")
	}
	if timeout > 0 {
		form.Set("timeout", strconv.Itoa(timeout))
	}
	var upid string
	if err := g.doMutation(ctx, http.MethodPost, lxcPath(node, vmid)+"/status/shutdown", form, &upid); err != nil {
		return nil, err
	}

	return &model.Task{UPID: upid}, nil
}

// RebootLXC reboots an LXC container via
// POST /nodes/{node}/lxc/{vmid}/status/reboot. timeout is the reboot timeout in
// seconds.
func (g *Gateway) RebootLXC(ctx context.Context, node string, vmid int, timeout int) (*model.Task, error) {
	form := url.Values{}
	if timeout > 0 {
		form.Set("timeout", strconv.Itoa(timeout))
	}
	var upid string
	if err := g.doMutation(ctx, http.MethodPost, lxcPath(node, vmid)+"/status/reboot", form, &upid); err != nil {
		return nil, err
	}

	return &model.Task{UPID: upid}, nil
}

// DeleteLXC deletes an LXC container via DELETE /nodes/{node}/lxc/{vmid}. The
// mutation parameters are carried in the query string (DELETE has no form
// body). purge removes the container from any backup jobs,
// destroy-unreferenced-disks removes disks not referenced by the config, and
// force requests deletion of a running container.
func (g *Gateway) DeleteLXC(
	ctx context.Context, node string, vmid int, purge, destroyUnreferencedDisks, force bool,
) (*model.Task, error) {
	query := url.Values{}
	if purge {
		query.Set("purge", "1")
	}
	if destroyUnreferencedDisks {
		query.Set("destroy-unreferenced-disks", "1")
	}
	if force {
		query.Set("force", "1")
	}
	var upid string
	// Mutations are never retried: use the single-attempt doOnce path with the
	// query string (doMutation sends a form body, which DELETE does not use).
	if err := g.doOnce(ctx, http.MethodDelete, lxcPath(node, vmid), query, &upid); err != nil {
		return nil, err
	}

	return &model.Task{UPID: upid}, nil
}

// ListLXCSnapshots lists the snapshots of an LXC container via
// GET /nodes/{node}/lxc/{vmid}/snapshot. Read-only.
func (g *Gateway) ListLXCSnapshots(ctx context.Context, node string, vmid int) ([]model.Snapshot, error) {
	var out []model.Snapshot
	if err := g.do(ctx, http.MethodGet, lxcPath(node, vmid)+"/snapshot", nil, &out); err != nil {
		return nil, err
	}

	return out, nil
}

// CloneLXC clones an LXC container via POST /nodes/{node}/lxc/{vmid}/clone.
func (g *Gateway) CloneLXC(ctx context.Context, node string, vmid int, req model.CloneLXCRequest) (*model.Task, error) {
	form := url.Values{}
	form.Set("newid", strconv.Itoa(req.NewID))
	if req.Full {
		form.Set("full", "1")
	}
	if req.Storage != "" {
		form.Set("storage", req.Storage)
	}
	if req.Hostname != "" {
		form.Set("hostname", req.Hostname)
	}
	if req.Description != "" {
		form.Set("description", req.Description)
	}
	if req.Pool != "" {
		form.Set("pool", req.Pool)
	}
	if req.Snapname != "" {
		form.Set("snapname", req.Snapname)
	}
	var upid string
	if err := g.doMutation(ctx, http.MethodPost, lxcPath(node, vmid)+"/clone", form, &upid); err != nil {
		return nil, err
	}

	return &model.Task{UPID: upid}, nil
}

// CreateLXCSnapshot creates a snapshot of an LXC container via
// POST /nodes/{node}/lxc/{vmid}/snapshot. LXC snapshots do not capture a VM
// state, so only snapname and description are sent.
func (g *Gateway) CreateLXCSnapshot(
	ctx context.Context, node string, vmid int, req model.SnapshotCreateRequest,
) (*model.Task, error) {
	form := url.Values{}
	form.Set("snapname", req.Snapname)
	if req.Description != "" {
		form.Set("description", req.Description)
	}
	var upid string
	if err := g.doMutation(ctx, http.MethodPost, lxcPath(node, vmid)+"/snapshot", form, &upid); err != nil {
		return nil, err
	}

	return &model.Task{UPID: upid}, nil
}

// DeleteLXCSnapshot deletes a snapshot of an LXC container via
// DELETE /nodes/{node}/lxc/{vmid}/snapshot/{snapname}.
func (g *Gateway) DeleteLXCSnapshot(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
	var upid string
	path := lxcPath(node, vmid) + "/snapshot/" + url.PathEscape(snapname)
	if err := g.doMutation(ctx, http.MethodDelete, path, nil, &upid); err != nil {
		return nil, err
	}

	return &model.Task{UPID: upid}, nil
}

// RollbackLXCSnapshot rolls an LXC container back to a snapshot via
// POST /nodes/{node}/lxc/{vmid}/snapshot/{snapname}/rollback. The `start` form
// value controls whether the container is started after the rollback; here it
// is left unset so PVE's default applies.
func (g *Gateway) RollbackLXCSnapshot(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
	var upid string
	path := lxcPath(node, vmid) + "/snapshot/" + url.PathEscape(snapname) + "/rollback"
	if err := g.doMutation(ctx, http.MethodPost, path, nil, &upid); err != nil {
		return nil, err
	}

	return &model.Task{UPID: upid}, nil
}

// RestoreVM restores a QEMU VM backup (qmrestore) via
// POST /nodes/{node}/qemu. The `restore` form flag is set to 1 when no VMID is
// supplied so PVE restores the VM to its original ID.
func (g *Gateway) RestoreVM(ctx context.Context, node string, req model.PVERestoreRequest) (*model.Task, error) {
	form := url.Values{}
	form.Set("archive", req.Archive)
	if req.VMID != 0 {
		form.Set("vmid", strconv.Itoa(req.VMID))
	} else {
		form.Set("restore", "1")
	}
	if req.Storage != "" {
		form.Set("storage", req.Storage)
	}
	if req.Unique {
		form.Set("unique", "1")
	}
	if req.Force {
		form.Set("force", "1")
	}
	if req.Pool != "" {
		form.Set("pool", req.Pool)
	}
	if req.BwLimit != 0 {
		form.Set("bwlimit", strconv.Itoa(req.BwLimit))
	}
	var upid string
	if err := g.doMutation(ctx, http.MethodPost, "/nodes/"+url.PathEscape(node)+"/qemu", form, &upid); err != nil {
		return nil, err
	}

	return &model.Task{UPID: upid}, nil
}

// SetVMConfig updates a QEMU VM's config via PUT /nodes/{node}/qemu/{vmid}/config.
func (g *Gateway) SetVMConfig(ctx context.Context, node string, vmid int, req model.SetVMConfigRequest) error {
	return g.doMutation(ctx, http.MethodPut, vmPath(node, vmid)+"/config", valuesFromMap(req.Config), nil)
}

// SetLXCConfig updates an LXC container's config via PUT /nodes/{node}/lxc/{vmid}/config.
func (g *Gateway) SetLXCConfig(ctx context.Context, node string, vmid int, req model.SetLXCConfigRequest) error {
	return g.doMutation(ctx, http.MethodPut, lxcPath(node, vmid)+"/config", valuesFromMap(req.Config), nil)
}

// valuesFromMap converts a config map into form-encoded values (mutation body).
func valuesFromMap(m map[string]string) url.Values {
	form := url.Values{}
	for k, v := range m {
		form.Set(k, v)
	}

	return form
}
