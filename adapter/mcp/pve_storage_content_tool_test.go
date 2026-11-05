package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/port"
)

// TestStorageContentListToolSuccess verifies pve_storage_content_list succeeds
// and returns structured output (an array of storage-content entries).
func TestStorageContentListToolSuccess(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServer(t, app, false)

	res := callTool(t, sess, "pve_storage_content_list",
		map[string]any{"node": "pve1", "storage": "local", "content": "iso", "vmid": 0})
	assert.False(t, res.IsError, "pve_storage_content_list should succeed")
	require.NotNil(t, res.StructuredContent, "tool should return structured output")
}

// TestStorageContentListToolErrorPropagation verifies IsError on gateway
// failure for pve_storage_content_list.
func TestStorageContentListToolErrorPropagation(t *testing.T) {
	gateway := &stubPVEGateway{err: port.ErrNotFound}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(gateway))
	_, sess := newTestServer(t, app, false)

	res := callTool(t, sess, "pve_storage_content_list",
		map[string]any{"node": "pve1", "storage": "local"})
	assert.True(t, res.IsError, "pve_storage_content_list should be IsError on gateway failure")
}

// TestStorageContentListToolRegisteredRO verifies pve_storage_content_list is
// a read-only tool registered even with the mutation gate OFF.
func TestStorageContentListToolRegisteredRO(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServer(t, app, false)
	names := toolNames(t, sess)

	assert.True(t, names["pve_storage_content_list"],
		"pve_storage_content_list is read-only and must be registered with gate OFF")
}

// TestStorageDownloadURLToolSuccess verifies pve_storage_download_url succeeds
// and surfaces the returned UPID in its structured output.
func TestStorageDownloadURLToolSuccess(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServer(t, app, true)

	res := callTool(t, sess, "pve_storage_download_url",
		map[string]any{
			"node": "pve1", "storage": "local", "content": "iso",
			"filename": "debian-12.iso", "url": "https://example.com/debian-12.iso",
		})
	assert.False(t, res.IsError, "pve_storage_download_url should succeed")
	require.NotNil(t, res.StructuredContent)
	sc, ok := res.StructuredContent.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "UPID:pve:download-url:...", sc["upid"])
}

// TestStorageDownloadURLToolErrorPropagation verifies IsError on gateway
// failure for pve_storage_download_url.
func TestStorageDownloadURLToolErrorPropagation(t *testing.T) {
	gateway := &stubPVEGateway{err: port.ErrUnauthorized}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(gateway))
	_, sess := newTestServer(t, app, true)

	res := callTool(t, sess, "pve_storage_download_url",
		map[string]any{
			"node": "pve1", "storage": "local", "content": "iso",
			"filename": "a.iso", "url": "https://example.com/a.iso",
		})
	assert.True(t, res.IsError, "pve_storage_download_url should be IsError on gateway failure")
}

// TestStorageDownloadURLToolGateOff verifies pve_storage_download_url is a
// mutation tool absent when the gate is OFF.
func TestStorageDownloadURLToolGateOff(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServer(t, app, false)
	names := toolNames(t, sess)

	assert.False(t, names["pve_storage_download_url"],
		"pve_storage_download_url is a mutation and must be gated off")
}
