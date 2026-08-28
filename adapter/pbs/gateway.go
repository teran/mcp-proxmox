package pbs

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"time"

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
	HTTPClient   *http.Client     // timeout 30s; transport replaceable for tests
	CACertPath   string           // optional custom CA (PEM); empty = system roots
	Logger       port.AppLogger
	Retries      int           // 2 (read-only idempotent requests)
	RetryBackoff time.Duration // 200ms
}

// Gateway is an implementation of port.PBSGateway on top of the PBS REST API.
type Gateway struct {
	cfg    Config
	client *http.Client
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

// buildClient returns the effective *http.Client to use. When CACertPath is
// empty the provided HTTPClient (or a default one) is returned unchanged, so
// tests can inject an httptest client. When CACertPath is set, the transport is
// cloned and its TLS RootCAs are replaced by a pool that includes the custom CA.
func buildClient(cfg Config) (*http.Client, error) {
	c := cfg.HTTPClient
	if c == nil {
		c = &http.Client{Timeout: defaultTimeout}
	}
	if cfg.CACertPath == "" {
		return c, nil
	}

	pool, err := systemOrNewCertPool()
	if err != nil {
		return c, err
	}
	// #nosec G304 -- cfg.CACertPath is a configuration-supplied CA path, not
	// attacker-controlled input. Its content is never logged.
	pem, err := os.ReadFile(cfg.CACertPath)
	if err != nil {
		return c, fmt.Errorf("pbs: read CA cert %q: %w", cfg.CACertPath, err)
	}
	if !pool.AppendCertsFromPEM(pem) {
		return c, fmt.Errorf("pbs: no valid CA certificates found in %q", cfg.CACertPath)
	}

	clone := *c
	tr := cloneTransport(c)
	tr.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	clone.Transport = tr
	return &clone, nil
}

// cloneTransport returns a deep clone of c.Transport when it is a
// *http.Transport, otherwise a clone of the default transport.
func cloneTransport(c *http.Client) *http.Transport {
	if t, ok := c.Transport.(*http.Transport); ok && t != nil {
		return t.Clone()
	}
	return http.DefaultTransport.(*http.Transport).Clone()
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
// §7.3.
func (g *Gateway) doOnce(ctx context.Context, method, path string, out any) error {
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

	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return &UpstreamError{Op: "build", Err: err}
	}
	req.Header.Set("Authorization", header)
	req.Header.Set("Accept", "application/json")

	client := g.client
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return &UpstreamError{Op: "transport", Err: err}
	}
	defer resp.Body.Close()

	if g.cfg.Logger != nil {
		// Request logging carries the upstream HTTP status code so both the
		// success (2xx) and error paths show it. Method + URL path only; the
		// Authorization header value is never logged (SPEC.md §5.4). Logged via
		// the ctx-aware logger so lines carry the same session_id/request_id as
		// the tool and error lines.
		g.logger.Debugf(ctx, "pbs request: %s %s upstream_http_status_code=%d", method, path, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &UpstreamError{Op: "read", Err: err}
	}

	if resp.StatusCode >= http.StatusBadRequest {
		// Surface the real PBS error message at debug level (truncated) so the
		// journal reveals the exact reason. The body of an error response never
		// contains the API token.
		g.logger.Debugf(ctx, "pbs error response %d: %s", resp.StatusCode, truncate(body))
		return g.mapHTTPError(resp.StatusCode, body)
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

	var out model.BackupNotes
	if err := g.do(ctx, http.MethodGet, path, &out); err != nil {
		return nil, err
	}
	return &out, nil
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

// truncate bounds an error-response body for debug logging, keeping the log
// line compact and free of unbounded payloads.
func truncate(b []byte) string {
	const max = 300
	if len(b) <= max {
		return string(b)
	}
	return string(b[:max]) + "..."
}
