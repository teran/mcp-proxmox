package pbs

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

// apiPrefix is the path prefix of the Proxmox Backup Server REST API. It is
// appended to cfg.Endpoint (which is the bare host, e.g.
// https://pbs.example.com:8007) to build the full request URL. See SPEC.md
// §1 / §4.5.
const apiPrefix = "/api2/json"

// defaultTimeout is applied when cfg.HTTPClient is nil.
const defaultTimeout = 30 * time.Second

// Config is the configuration of the Proxmox Backup Server HTTP client.
// See SPEC.md §4.5.
type Config struct {
	Endpoint     string           // e.g. https://pbs.example.com:8007
	TokenSource  port.TokenSource // returns the Authorization header
	HTTPClient   *http.Client     // timeout 30s; transport replaceable for tests (Resty derives its transport from this)
	CACertPath   string           // optional custom CA (PEM); empty = system roots
	Logger       port.AppLogger
	Retries      int           // 2 (read-only idempotent requests)
	RetryBackoff time.Duration // 200ms
}

// Gateway is an implementation of port.PBSGateway on top of the PBS REST API.
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
			return nil, fmt.Errorf("pbs: read CA cert %q: %w", cfg.CACertPath, err)
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("pbs: no valid CA certificates found in %q", cfg.CACertPath)
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

// do executes a PBS request, decoding the JSON envelope into out. Reads are
// retried (idempotent) per SPEC.md §7.5: Retries additional attempts after the
// first with linear backoff, retrying only *UpstreamError; sentinels, *APIError
// and *EmptyDataError abort immediately. The loop honors ctx cancellation.
func (g *Gateway) do(ctx context.Context, method, path string, out any) error {
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
		lastErr = g.doOnce(ctx, method, path, out)
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
func (g *Gateway) doOnce(ctx context.Context, method, path string, out any) error {
	return g.doOnceReq(ctx, method, path, nil, out)
}

// doOnceWithForm is like doOnce but sends an application/x-www-form-urlencoded
// body. It is used by the mutation surface (verify/gc/prune/sync), which takes
// its parameters in the form body.
func (g *Gateway) doOnceWithForm(ctx context.Context, method, path string, form url.Values, out any) error {
	return g.doOnceReq(ctx, method, path, form, out)
}

// doOnceReq performs a single HTTP request (no retry). form carries the optional
// form-encoded mutation body (nil for reads). See doOnce / doOnceWithForm.
func (g *Gateway) doOnceReq(ctx context.Context, method, path string, form url.Values, out any) error {
	if g.cfg.Endpoint == "" {
		return &UpstreamError{Op: "config", Err: errors.New("pbs: empty endpoint")}
	}
	if g.tlsErr != nil {
		return &UpstreamError{Op: "config", Err: g.tlsErr}
	}

	// Token source failure is a misconfiguration surfaced before any request is
	// sent (SPEC.md §7.4) and is reported in the ErrUnauthorized family.
	header, err := g.cfg.TokenSource.AuthorizationHeader(ctx)
	if err != nil {
		return fmt.Errorf("%w: pbs: token source: %v", port.ErrUnauthorized, err)
	}

	u := strings.TrimRight(g.cfg.Endpoint, "/") + apiPrefix + path

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

	resp, err := req.Execute(method, u)
	if err != nil {
		return &UpstreamError{Op: "transport", Err: err}
	}
	body := resp.Bytes()

	if g.cfg.Logger != nil {
		// Request logging carries the upstream HTTP status code plus request
		// metrics so both the success (2xx) and error paths show them. Method +
		// URL path (including the API prefix) only; the Authorization header
		// value is never logged (SPEC.md §5.4). Logged at trace level via the
		// ctx-aware logger so lines carry the same session_id/request_id as the
		// tool and error lines, and stay hidden unless LOG_LEVEL=trace.
		g.logger.Tracef(ctx, "pbs request: %s %s%s upstream_http_status_code=%d duration_ms=%d in_bytes=%d out_bytes=%d",
			method, apiPrefix, path, resp.StatusCode(), resp.Duration().Milliseconds(), 0, len(body))
	}

	if resp.StatusCode() >= http.StatusBadRequest {
		// Only the status is surfaced (at trace level); the error response body
		// is never written to the journal — it could carry secret-adjacent data,
		// and the requirement is to log no response bodies (SPEC.md §5.4).
		g.logger.Tracef(ctx, "pbs error response %d", resp.StatusCode())
		return g.mapHTTPError(resp.StatusCode(), body)
	}
	return g.decode(body, out)
}

// decode parses the {"data": ...} envelope into out, mapping empty/null data for
// single-entity getters to EmptyDataError and an error envelope to APIError.
func (g *Gateway) decode(body []byte, out any) error {
	if len(body) == 0 {
		return &UpstreamError{Op: "decode", Err: errors.New("pbs: empty response body")}
	}

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
		return &UpstreamError{Op: "http", Status: status, Err: fmt.Errorf("pbs: HTTP status %d", status)}
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

// ListDatastores returns the PBS datastores.
func (g *Gateway) ListDatastores(ctx context.Context) ([]model.Datastore, error) {
	var out []model.Datastore
	if err := g.do(ctx, http.MethodGet, "/admin/datastore", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetDatastoreStatus returns the status of a single datastore.
func (g *Gateway) GetDatastoreStatus(ctx context.Context, store string) (*model.DatastoreStatus, error) {
	var out model.DatastoreStatus
	if err := g.do(ctx, http.MethodGet, "/admin/datastore/"+store+"/status", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetDatastoreConfig returns the full configuration of a single datastore via
// GET /config/datastore/{store}. The store name is path-escaped so a value
// containing reserved path characters cannot inject extra segments (S11
// path-injection fix). Read-only.
func (g *Gateway) GetDatastoreConfig(ctx context.Context, store string) (*model.DatastoreConfig, error) {
	var out model.DatastoreConfig
	if err := g.do(ctx, http.MethodGet, "/config/datastore/"+url.PathEscape(store), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListBackups returns the backup snapshots in a datastore.
func (g *Gateway) ListBackups(ctx context.Context, store string) ([]model.Backup, error) {
	var out []model.Backup
	if err := g.do(ctx, http.MethodGet, "/admin/datastore/"+store+"/snapshots", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetBackup returns all backup snapshots in the datastore whose backup-id
// matches backupID. PBS exposes no single-snapshot GET (the per-snapshot path
// does not exist); snapshots are listed via GET /admin/datastore/{store}/snapshots
// and filtered by backup-id, which is the real grouping key (e.g. a VMID).
// Returns an empty slice (no error) when no snapshot matches.
func (g *Gateway) GetBackup(ctx context.Context, store, backupID string) ([]model.Backup, error) {
	all, err := g.ListBackups(ctx, store)
	if err != nil {
		return nil, err
	}
	out := make([]model.Backup, 0, len(all))
	for _, b := range all {
		if b.BackupID == backupID {
			out = append(out, b)
		}
	}
	return out, nil
}

// GetBackupNotes returns the notes of a backup group from
// GET /admin/datastore/{store}/group-notes?backup-id=<id>&backup-type=<type>.
// The backup-id and backup-type parameters are URL-encoded via url.Values. Only
// the URL path is logged, never the token or any secrets (SPEC.md §5.4).
func (g *Gateway) GetBackupNotes(ctx context.Context, store, backupID, backupType string) (*model.BackupNotes, error) {
	q := url.Values{}
	q.Set("backup-id", backupID)
	q.Set("backup-type", backupType)
	path := "/admin/datastore/" + store + "/group-notes?" + q.Encode()

	// GET /group-notes returns the notes as a plain string, not an object.
	var notes string
	if err := g.do(ctx, http.MethodGet, path, &notes); err != nil {
		return nil, err
	}
	return &model.BackupNotes{Notes: notes}, nil
}

// GetVerifyStatus returns the status of a verify job by UPID.
func (g *Gateway) GetVerifyStatus(ctx context.Context, store, upid string) (*model.VerifyStatus, error) {
	var out model.VerifyStatus
	if err := g.do(ctx, http.MethodGet, "/admin/datastore/"+store+"/verify/"+upid, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetPruneStatus returns the status of a prune job by UPID.
func (g *Gateway) GetPruneStatus(ctx context.Context, store, upid string) (*model.PruneStatus, error) {
	var out model.PruneStatus
	if err := g.do(ctx, http.MethodGet, "/admin/datastore/"+store+"/prune/"+upid, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetPBSVersion returns the PBS version info.
func (g *Gateway) GetPBSVersion(ctx context.Context) (*model.PBSVersion, error) {
	var out model.PBSVersion
	if err := g.do(ctx, http.MethodGet, "/version", &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// backupFilesPath builds the PBS backup file-list path prefix for a store,
// backup type, backup ID and snapshot.
func backupFilesPath(store, backupType, backupID, snapshot string) string {
	return "/admin/datastore/" + store + "/backups/" + backupType + "/" + backupID + "/" + snapshot
}

// ListBackupFiles lists the files inside a backup snapshot via
// GET /admin/datastore/{store}/backups/{type}/{id}/{snapshot}/files. The
// optional path filter is passed as the `path` query parameter (default "/").
// Read-only.
func (g *Gateway) ListBackupFiles(ctx context.Context, store, backupType, backupID, snapshot, path string) ([]model.PBSFile, error) {
	q := url.Values{}
	if path != "" {
		q.Set("path", path)
	}
	urlPath := backupFilesPath(store, backupType, backupID, snapshot) + "/files"
	if len(q) > 0 {
		urlPath += "?" + q.Encode()
	}
	var out []model.PBSFile
	if err := g.do(ctx, http.MethodGet, urlPath, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// GetTaskStatus returns the status of a task by UPID via
// GET /admin/tasks/{upid}. Read-only.
func (g *Gateway) GetTaskStatus(ctx context.Context, upid string) (*model.PBSTask, error) {
	var out model.PBSTask
	if err := g.do(ctx, http.MethodGet, "/admin/tasks/"+upid, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTaskLog returns the log of a task by UPID via
// GET /admin/tasks/{upid}/log. The optional limit caps the number of returned
// lines. Read-only.
func (g *Gateway) GetTaskLog(ctx context.Context, upid string, limit int) ([]model.TaskLogEntry, error) {
	urlPath := "/admin/tasks/" + upid + "/log"
	if limit > 0 {
		urlPath += "?limit=" + strconv.Itoa(limit)
	}
	var out []model.TaskLogEntry
	if err := g.do(ctx, http.MethodGet, urlPath, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// RestoreFile restores a single file out of a backup snapshot via
// POST /admin/datastore/{store}/backups/{type}/{id}/{snapshot}/files. The
// source `path` inside the backup is written to the `target` path on the PBS
// filesystem; the endpoint returns no task UPID.
func (g *Gateway) RestoreFile(ctx context.Context, store, backupType, backupID, snapshot string, req model.PBSFileRestoreRequest) error {
	form := url.Values{}
	form.Set("path", req.Path)
	form.Set("target", req.Target)
	return g.doMutation(ctx, http.MethodPost, backupFilesPath(store, backupType, backupID, snapshot)+"/files", form, nil)
}

// RestoreVMBackup restores a VM backup snapshot into a PVE datastore via
// POST /admin/datastore/{store}/backups/{type}/{id}/{snapshot}/restore and
// returns the task UPID. Target is the PVE storage; VMID is either a numeric ID
// or the string "next". The optional password/fingerprint authenticate to the
// target PVE node and are sent in the form body — they are never logged.
func (g *Gateway) RestoreVMBackup(ctx context.Context, store, backupType, backupID, snapshot string, req model.PBSVMRestoreRequest) (string, error) {
	form := url.Values{}
	form.Set("target", req.Target)
	if req.VMID != "" {
		form.Set("vmid", req.VMID)
	}
	if req.Host != "" {
		form.Set("host", req.Host)
	}
	if req.Password != "" {
		form.Set("password", req.Password)
	}
	if req.Fingerprint != "" {
		form.Set("fingerprint", req.Fingerprint)
	}
	if req.Pool != "" {
		form.Set("pool", req.Pool)
	}
	if req.Verbose {
		form.Set("verbose", "1")
	}
	if req.Reload {
		form.Set("reload", "1")
	}
	var upid string
	if err := g.doMutation(ctx, http.MethodPost, backupFilesPath(store, backupType, backupID, snapshot)+"/restore", form, &upid); err != nil {
		return "", err
	}
	return upid, nil
}

// doMutation executes a single mutation request (never retried — mutations are
// not idempotent and must not be replayed on a transient 5xx; SPEC.md §7.5)
// with an optional form-encoded body. The mutation response `data` is the task
// UPID (a plain string).
func (g *Gateway) doMutation(ctx context.Context, method, path string, form url.Values, out any) error {
	return g.doOnceWithForm(ctx, method, path, form, out)
}

// StartVerify starts a verify job via POST /admin/datastore/{store}/verify and
// returns the task UPID.
func (g *Gateway) StartVerify(ctx context.Context, store string) (string, error) {
	var upid string
	if err := g.doMutation(ctx, http.MethodPost, "/admin/datastore/"+store+"/verify", nil, &upid); err != nil {
		return "", err
	}
	return upid, nil
}

// StartGC starts a garbage-collection job via POST /admin/datastore/{store}/gc
// and returns the task UPID.
func (g *Gateway) StartGC(ctx context.Context, store string) (string, error) {
	var upid string
	if err := g.doMutation(ctx, http.MethodPost, "/admin/datastore/"+store+"/gc", nil, &upid); err != nil {
		return "", err
	}
	return upid, nil
}

// StartPrune starts a prune job via POST /admin/datastore/{store}/prune and
// returns the task UPID.
func (g *Gateway) StartPrune(ctx context.Context, store string) (string, error) {
	var upid string
	if err := g.doMutation(ctx, http.MethodPost, "/admin/datastore/"+store+"/prune", nil, &upid); err != nil {
		return "", err
	}
	return upid, nil
}

// StartSync starts a one-off sync job via POST /admin/datastore/{store}/sync
// and returns the task UPID.
func (g *Gateway) StartSync(ctx context.Context, store string, req model.PBSSyncRequest) (string, error) {
	form := url.Values{}
	if req.Remote != "" {
		form.Set("remote", req.Remote)
	}
	if req.RemoteStore != "" {
		form.Set("remote-store", req.RemoteStore)
	}
	if req.Owner != "" {
		form.Set("owner", req.Owner)
	}
	if req.MaxFiles != 0 {
		form.Set("maxfiles", strconv.Itoa(req.MaxFiles))
	}
	if req.RemoveVanished {
		form.Set("remove-vanished", "1")
	}
	if req.RateIn != 0 {
		form.Set("ratein", strconv.Itoa(req.RateIn))
	}
	if req.RateOut != 0 {
		form.Set("rateout", strconv.Itoa(req.RateOut))
	}
	if req.SkipLost {
		form.Set("skip-lost", "1")
	}
	if req.NotifyUser != "" {
		form.Set("notify-user", req.NotifyUser)
	}

	var upid string
	if err := g.doMutation(ctx, http.MethodPost, "/admin/datastore/"+store+"/sync", form, &upid); err != nil {
		return "", err
	}
	return upid, nil
}
