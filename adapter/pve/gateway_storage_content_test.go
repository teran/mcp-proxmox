package pve

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/model"
	"github.com/teran/mcp-proxmox/domain/port"
)

// --- ListStorageContent (read-only, GET /nodes/{node}/storage/{storage}/content) ---

// TestGateway_ListStorageContent_Success verifies the GET request path, query
// filters, Authorization header and the decoded list.
func TestGateway_ListStorageContent_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[`+
		`{"volid":"local:iso/debian-12.iso","content":"iso","format":"iso","size":513802240,"ctime":1700000000,"vmid":0},`+
		`{"volid":"local:203/vm-203-disk-0.raw","content":"rootdir","format":"raw","size":8589934592,"ctime":1700000001,"vmid":203}`+
		`]}`, rec)
	g, _ := newTestGateway(t, srv)

	got, err := g.ListStorageContent(context.Background(), "pve1", "local",
		port.StorageContentListOptions{Content: "iso", VMID: 200})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "local:iso/debian-12.iso", got[0].VolID)
	assert.Equal(t, "iso", got[0].Content)
	assert.Equal(t, int64(0), int64(got[0].VMID))
	assert.Equal(t, "local:203/vm-203-disk-0.raw", got[1].VolID)
	assert.Equal(t, 203, got[1].VMID)

	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/api2/json/nodes/pve1/storage/local/content", rec.path)
	assert.Equal(t, "PVEAPIToken=user@pam!tokenid=secret", rec.auth)
	q, err := url.ParseQuery(rec.query)
	require.NoError(t, err)
	assert.Equal(t, "iso", q.Get("content"))
	assert.Equal(t, "200", q.Get("vmid"))
}

// TestGateway_ListStorageContent_NoFilters verifies an empty option set sends
// no query string.
func TestGateway_ListStorageContent_NoFilters(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[]}`, rec)
	g, _ := newTestGateway(t, srv)

	got, err := g.ListStorageContent(context.Background(), "pve1", "local", port.StorageContentListOptions{})
	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Equal(t, "", rec.query)
}

// TestGateway_ListStorageContent_NullData verifies a `data:null` (or empty)
// response decodes to an empty slice rather than an error.
func TestGateway_ListStorageContent_NullData(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":null}`, rec)
	g, _ := newTestGateway(t, srv)

	got, err := g.ListStorageContent(context.Background(), "pve1", "local", port.StorageContentListOptions{})
	require.NoError(t, err)
	assert.Empty(t, got)
}

// TestGateway_ListStorageContent_Unauthorized verifies 401 maps to
// ErrUnauthorized.
func TestGateway_ListStorageContent_Unauthorized(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusUnauthorized, `{"data":null}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.ListStorageContent(context.Background(), "pve1", "local", port.StorageContentListOptions{})
	require.Error(t, err)
	assert.ErrorIs(t, err, port.ErrUnauthorized)
}

// TestGateway_ListStorageContent_5xxRetriesThenUpstreamError verifies a 5xx is
// retried (this is a read) and surfaces as an UpstreamError.
func TestGateway_ListStorageContent_5xxRetriesThenUpstreamError(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.ListStorageContent(context.Background(), "pve1", "local", port.StorageContentListOptions{})
	require.Error(t, err)
	var ue *UpstreamError
	assert.ErrorAs(t, err, &ue)
	assert.Equal(t, http.StatusInternalServerError, ue.Status)
	// 1 initial + 1 retry (Retries:1).
	assert.Equal(t, 2, rec.count)
}

// TestGateway_ListStorageContent_InvalidJSON verifies a non-JSON body surfaces
// a decode error.
func TestGateway_ListStorageContent_InvalidJSON(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `this is not json`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.ListStorageContent(context.Background(), "pve1", "local", port.StorageContentListOptions{})
	require.Error(t, err)
}

// --- DownloadURL (mutation, POST /nodes/{node}/storage/{storage}/download-url) ---

// TestGateway_DownloadURL_SuccessDataForm verifies the `{"data":"UPID:..."}`
// response shape (form A) and the outbound form fields.
func TestGateway_DownloadURL_SuccessDataForm(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":"UPID:pve:download-url:..."}`, rec)
	g, _ := newTestGateway(t, srv)

	upid, err := g.DownloadURL(context.Background(), "pve1", "local", model.StorageDownloadURLRequest{
		Content: "iso", Filename: "debian-12.iso", URL: "https://example.com/debian-12.iso",
		VerifyCertificates: true,
	})
	require.NoError(t, err)
	assert.Equal(t, "UPID:pve:download-url:...", upid)

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api2/json/nodes/pve1/storage/local/download-url", rec.path)
	assert.Equal(t, "PVEAPIToken=user@pam!tokenid=secret", rec.auth)
	assert.Equal(t, "iso", rec.form.Get("content"))
	assert.Equal(t, "debian-12.iso", rec.form.Get("filename"))
	assert.Equal(t, "https://example.com/debian-12.iso", rec.form.Get("url"))
	assert.Equal(t, "1", rec.form.Get("verify-certificates"))
}

// TestGateway_DownloadURL_SuccessUpidForm verifies the `{"upid":"UPID:..."}`
// response shape (form B — the tolerant-decode gotcha). Both PVE response
// variants must yield the UPID.
func TestGateway_DownloadURL_SuccessUpidForm(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"upid":"UPID:pve:download-url:...2"}`, rec)
	g, _ := newTestGateway(t, srv)

	upid, err := g.DownloadURL(context.Background(), "pve1", "local", model.StorageDownloadURLRequest{
		Content: "iso", Filename: "a.iso", URL: "https://example.com/a.iso",
	})
	require.NoError(t, err)
	assert.Equal(t, "UPID:pve:download-url:...2", upid)
}

// TestGateway_DownloadURL_VerifyCertificatesOmitted verifies verify-certificates
// is omitted from the form when false.
func TestGateway_DownloadURL_VerifyCertificatesOmitted(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":"UPID:..."}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.DownloadURL(context.Background(), "pve1", "local", model.StorageDownloadURLRequest{
		Content: "iso", Filename: "a.iso", URL: "https://example.com/a.iso",
		VerifyCertificates: false,
	})
	require.NoError(t, err)
	assert.Empty(t, rec.form.Get("verify-certificates"))
}

// TestGateway_DownloadURL_Unauthorized verifies 401 maps to ErrUnauthorized
// with a single request.
func TestGateway_DownloadURL_Unauthorized(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusUnauthorized, `{"data":null}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.DownloadURL(context.Background(), "pve1", "local", model.StorageDownloadURLRequest{
		Content: "iso", Filename: "a.iso", URL: "https://x/a.iso",
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, port.ErrUnauthorized)
	assert.Equal(t, 1, rec.count)
}

// TestGateway_DownloadURL_5xxNoRetry verifies a 5xx surfaces as an UpstreamError
// with a single request (mutations are never retried).
func TestGateway_DownloadURL_5xxNoRetry(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.DownloadURL(context.Background(), "pve1", "local", model.StorageDownloadURLRequest{
		Content: "iso", Filename: "a.iso", URL: "https://x/a.iso",
	})
	require.Error(t, err)
	var ue *UpstreamError
	assert.ErrorAs(t, err, &ue)
	assert.Equal(t, 1, rec.count, "mutations must not be retried")
}

// TestGateway_DownloadURL_InvalidJSON verifies a non-JSON body surfaces a decode
// error.
func TestGateway_DownloadURL_InvalidJSON(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `this is not json`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.DownloadURL(context.Background(), "pve1", "local", model.StorageDownloadURLRequest{
		Content: "iso", Filename: "a.iso", URL: "https://x/a.iso",
	})
	require.Error(t, err)
}

// --- CreateLXC (mutation, POST /nodes/{node}/lxc) ---

// TestGateway_CreateLXC_SyncResponse verifies the synchronous
// `{"data":203}` response decodes to vmid 203 and the form carries all fields.
func TestGateway_CreateLXC_SyncResponse(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":203}`, rec)
	g, _ := newTestGateway(t, srv)

	vmid, err := g.CreateLXC(context.Background(), "pve1", model.CreateLXCRequest{
		VMID: 203, OSTemplate: "local:vztmpl/debian-12.tar.zst",
		Hostname: "ct1", Storage: "local", RootFS: "local:8", Cores: 2, Memory: 1024, Swap: 512,
		Net0: "name=eth0,bridge=vmbr0", Description: "prod ct", Tags: "ct;prod",
		Unprivileged: true, Password: "s3cret", SSHPublicKeys: "ssh-ed25519 AAAA...",
	})
	require.NoError(t, err)
	assert.Equal(t, 203, vmid)

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api2/json/nodes/pve1/lxc", rec.path)
	assert.Equal(t, "PVEAPIToken=user@pam!tokenid=secret", rec.auth)
	assert.Equal(t, "203", rec.form.Get("vmid"))
	assert.Equal(t, "local:vztmpl/debian-12.tar.zst", rec.form.Get("ostemplate"))
	assert.Equal(t, "ct1", rec.form.Get("hostname"))
	assert.Equal(t, "local", rec.form.Get("storage"))
	assert.Equal(t, "local:8", rec.form.Get("rootfs"))
	assert.Equal(t, "2", rec.form.Get("cores"))
	assert.Equal(t, "1024", rec.form.Get("memory"))
	assert.Equal(t, "512", rec.form.Get("swap"))
	assert.Equal(t, "name=eth0,bridge=vmbr0", rec.form.Get("net0"))
	assert.Equal(t, "prod ct", rec.form.Get("description"))
	assert.Equal(t, "ct;prod", rec.form.Get("tags"))
	assert.Equal(t, "1", rec.form.Get("unprivileged"))
	assert.Equal(t, "s3cret", rec.form.Get("password"))
	assert.Equal(t, "ssh-ed25519 AAAA...", rec.form.Get("ssh-public-keys"))
}

// TestGateway_CreateLXC_AsyncUPID verifies the async `{"data":"UPID:...:vzcreate:203:..."}`
// response reuses vmIDFromUPID and decodes to 203.
func TestGateway_CreateLXC_AsyncUPID(t *testing.T) {
	rec := &reqCapture{}
	// UPID layout: UPID:<node>:<pid>:<pstart>:<starttime>:vzcreate:<id>:<user>:<uuid>
	srv := mockServer(t, 200, `{"data":"UPID:pve1:00000123:0ABCDEF1:6ABCDEF1:vzcreate:203:root@pam:uuid"}`, rec)
	g, _ := newTestGateway(t, srv)

	vmid, err := g.CreateLXC(context.Background(), "pve1", model.CreateLXCRequest{
		VMID: 203, OSTemplate: "local:vztmpl/debian-12.tar.zst",
	})
	require.NoError(t, err)
	assert.Equal(t, 203, vmid)
}

// TestGateway_CreateLXC_UnprivilegedOmitted verifies unprivileged is omitted
// from the form when false.
func TestGateway_CreateLXC_UnprivilegedOmitted(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":203}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.CreateLXC(context.Background(), "pve1", model.CreateLXCRequest{
		VMID: 203, OSTemplate: "local:vztmpl/tpl", Unprivileged: false,
	})
	require.NoError(t, err)
	assert.Empty(t, rec.form.Get("unprivileged"))
}

// TestGateway_CreateLXC_APIError verifies 400 maps to APIError and is not
// retried.
func TestGateway_CreateLXC_APIError(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusBadRequest, `{"data":null,"errors":{"ostemplate":"no such template"}}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.CreateLXC(context.Background(), "pve1", model.CreateLXCRequest{OSTemplate: "bad"})
	require.Error(t, err)
	var ae *APIError
	assert.ErrorAs(t, err, &ae)
	assert.NotEmpty(t, ae.Messages)
	assert.Equal(t, 1, rec.count, "APIError must not be retried")
}

// TestGateway_CreateLXC_Unauthorized verifies 401 maps to ErrUnauthorized.
func TestGateway_CreateLXC_Unauthorized(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusUnauthorized, `{"data":null}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.CreateLXC(context.Background(), "pve1", model.CreateLXCRequest{OSTemplate: "tpl"})
	require.Error(t, err)
	assert.ErrorIs(t, err, port.ErrUnauthorized)
}
