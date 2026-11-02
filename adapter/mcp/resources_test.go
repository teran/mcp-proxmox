package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestServer_ReadGuidanceResources covers the tools and security guidance
// bodies (guidanceBody's "tools"/"security" cases), which TestServer_ReadGuidanceResource
// (overview only) does not exercise. Every registered resource must resolve to
// non-empty markdown echoing the requested URI.
func TestServer_ReadGuidanceResources(t *testing.T) {
	_, sess := newTestServer(t, newTestApp(), false)

	for _, name := range []string{"tools", "security"} {
		t.Run(name, func(t *testing.T) {
			uri := guidanceURI + name
			res, err := sess.ReadResource(context.Background(), &mcpSDK.ReadResourceParams{URI: uri})
			require.NoError(t, err, "reading the %s guidance resource must succeed", name)
			require.NotEmpty(t, res.Contents, "read result must carry a content block")

			c := res.Contents[0]
			assert.Equal(t, uri, c.URI)
			assert.Equal(t, "text/markdown", c.MIMEType)
			assert.NotEmpty(t, c.Text, "guidance body for %q must be non-empty", name)
			assert.Contains(t, c.Text, "#", "guidance content should look like markdown")
		})
	}
}

// TestGuidanceUnknownName verifies the default branches of the guidance
// title/description/body helpers: an unknown resource name yields empty strings
// rather than a panic or a wrong-but-nonempty value.
func TestGuidanceUnknownName(t *testing.T) {
	assert.Equal(t, "", guidanceTitle("nope"))
	assert.Equal(t, "", guidanceDescription("nope"))
	assert.Equal(t, "", guidanceBody("nope"))
}

// TestGuidanceHandlerWrongURI verifies the guidance handler returns a
// ResourceNotFoundError when the requested URI does not match the resource's
// registered URI (the SDK maps this to the invalid-params JSON-RPC code).
func TestGuidanceHandlerWrongURI(t *testing.T) {
	h := guidanceHandler("overview")
	_, err := h(context.Background(), &mcpSDK.ReadResourceRequest{
		Params: &mcpSDK.ReadResourceParams{URI: "mcp-proxmox://guidance/other"},
	})
	require.Error(t, err, "a mismatched URI must resolve to not-found")

	var rpcErr *jsonrpc.Error
	require.ErrorAs(t, err, &rpcErr, "not-found must surface as a JSON-RPC error")
	assert.Equal(t, int64(jsonrpc.CodeInvalidParams), rpcErr.Code)
}
