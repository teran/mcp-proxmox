package mcp

// Tests for the self-describing server (SPEC M08/M09): the enriched
// server-level Instructions surfaced over the protocol, and the `resources`
// capability exposing read-only markdown guidance resources under
// `mcp-proxmox://guidance/*`.
//
// The implementation lives in:
//   - ServerInstructions (server.go)  — the enriched top-level Instructions text.
//   - RegisterGuidanceResources (resources.go) — registers the guidance
//     resources; called from NewServer.
//
// SDK method signatures used (go-sdk v1.8.0):
//   - (*ClientSession).InitializeResult() *InitializeResult        // .Instructions string
//   - (*ClientSession).ListResources(ctx, *ListResourcesParams) (*ListResourcesResult, error)
//   - (*ClientSession).ReadResource(ctx, *ReadResourceParams) (*ReadResourceResult, error)
//   - (*Server).AddResource(r *Resource, h ResourceHandler) // ResourceHandler:
//     func(ctx, *ReadResourceRequest) (*ReadResourceResult, error)

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-proxmox/application"
)

// guidanceURI is the scheme prefix for the server's guidance resources.
const guidanceURI = "mcp-proxmox://guidance/"

// instructionsKeywords are the required substrings that the enriched
// ServerInstructions must carry (SPEC M08): the local-stdio invariant, the
// mutation gating, the secrets/data-hygiene warning, and the guidance resource
// index.
var instructionsKeywords = []string{
	"stdio",
	"ENABLE_MUTATIONS",
	"never logged",
	"mcp-proxmox://guidance",
}

// newTestApp returns an application.App with the PVE backend stubbed, matching
// the server wiring used by the other adapter tests.
func newTestApp() *application.App {
	return application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
}

// TestServerInstructions_Enriched pins the source of truth for the enriched
// server-level Instructions (M08). This is the primary regression guard: if the
// self-describing text is trimmed back to a single sentence, the required
// keywords disappear and the test fails.
func TestServerInstructions_Enriched(t *testing.T) {
	for _, kw := range instructionsKeywords {
		assert.Contains(t, ServerInstructions, kw, "ServerInstructions must mention %q", kw)
	}
	// The invariant and hygiene warning should read as complete ideas, not just
	// bare keywords.
	assert.Contains(t, ServerInstructions, "backends are optional", "ServerInstructions must state the optional-backends invariant")
	assert.Contains(t, ServerInstructions, "TLS verification is always on", "ServerInstructions must carry the TLS guarantee")
}

// newServerWithInstructions builds a server via the real NewServer path with the
// given ServerOptions (mirroring how cmd/mcp-proxmox passes
// ServerOptions.Instructions), and returns a connected client session.
func newServerWithInstructions(t *testing.T, opts *mcpSDK.ServerOptions) *mcpSDK.ClientSession {
	t.Helper()
	s := NewServer(
		&mcpSDK.Implementation{Name: "mcp-proxmox-test", Version: "v0.0.0-test"},
		Deps{App: newTestApp(), Logger: nopLogger{}, EnableMutations: false},
		opts,
	)

	return connectTestServer(t, s)
}

// TestServer_PropagatesInstructionsToClient verifies the enriched server-level
// Instructions flow end-to-end from NewServer (via ServerOptions.Instructions,
// as wired by cmd/mcp-proxmox) through the SDK to the connected client's
// InitializeResult.
func TestServer_PropagatesInstructionsToClient(t *testing.T) {
	sess := newServerWithInstructions(t, &mcpSDK.ServerOptions{Instructions: ServerInstructions})

	initResult := sess.InitializeResult()
	require.NotNil(t, initResult, "client must have an InitializeResult after connect")
	require.NotEmpty(t, initResult.Instructions, "server-level Instructions must reach the client")

	for _, kw := range instructionsKeywords {
		assert.Contains(t, initResult.Instructions, kw, "InitializeResult.Instructions must mention %q", kw)
	}
	assert.Equal(t, ServerInstructions, initResult.Instructions, "the client must receive the full ServerInstructions verbatim")
}

// guidanceUris returns the set of resource URIs advertised by a server.
func guidanceUris(t *testing.T, sess *mcpSDK.ClientSession) []*mcpSDK.Resource {
	t.Helper()
	res, err := sess.ListResources(context.Background(), nil)
	require.NoError(t, err, "ListResources must succeed")
	require.NotNil(t, res)

	return res.Resources
}

// TestServer_RegistersGuidanceResources verifies NewServer registers the
// guidance resources capability: ListResources advertises at least overview and
// tools under mcp-proxmox://guidance/, each marked text/markdown.
func TestServer_RegistersGuidanceResources(t *testing.T) {
	_, sess := newTestServer(t, newTestApp(), false)

	byURI := make(map[string]*mcpSDK.Resource)
	for _, r := range guidanceUris(t, sess) {
		byURI[r.URI] = r
	}

	require.Contains(t, byURI, guidanceURI+"overview", "server must advertise the overview guidance resource")
	require.Contains(t, byURI, guidanceURI+"tools", "server must advertise the tools guidance resource")
	require.Contains(t, byURI, guidanceURI+"security", "server must advertise the security guidance resource")

	for _, uri := range []string{guidanceURI + "overview", guidanceURI + "tools", guidanceURI + "security"} {
		r := byURI[uri]
		assert.Equal(t, "text/markdown", r.MIMEType, "guidance resource %s must be text/markdown", uri)
		assert.NotEmpty(t, r.Name, "guidance resource %s must have a name", uri)
	}
}

// TestServer_ReadGuidanceResource verifies a known guidance resource can be
// read and returns non-empty markdown with the expected URI and MIME type.
func TestServer_ReadGuidanceResource(t *testing.T) {
	_, sess := newTestServer(t, newTestApp(), false)

	res, err := sess.ReadResource(context.Background(), &mcpSDK.ReadResourceParams{URI: guidanceURI + "overview"})
	require.NoError(t, err, "reading the overview guidance resource must succeed")
	require.NotNil(t, res)
	require.NotEmpty(t, res.Contents, "read result must carry at least one content block")

	c := res.Contents[0]
	assert.Equal(t, guidanceURI+"overview", c.URI, "content URI must echo the requested resource URI")
	assert.Equal(t, "text/markdown", c.MIMEType, "guidance content must be markdown")
	assert.NotEmpty(t, c.Text, "guidance content must be non-empty markdown")
	assert.Contains(t, c.Text, "#", "guidance content should look like markdown")
}

// TestServer_ReadUnknownResource_NotFound verifies reading an unregistered URI
// under the guidance scheme returns the protocol's resource-not-found error
// (mcpSDK.ResourceNotFoundError semantics, code CodeResourceNotFound).
func TestServer_ReadUnknownResource_NotFound(t *testing.T) {
	_, sess := newTestServer(t, newTestApp(), false)

	_, err := sess.ReadResource(context.Background(), &mcpSDK.ReadResourceParams{URI: guidanceURI + "does-not-exist"})
	require.Error(t, err, "reading an unknown guidance URI must fail")

	var rpcErr *jsonrpc.Error
	require.ErrorAs(t, err, &rpcErr, "the not-found condition must surface as a JSON-RPC error")
	assert.Equal(t, int64(jsonrpc.CodeInvalidParams), rpcErr.Code, "unknown resource must map to the invalid-params code")
	assert.NotEmpty(t, rpcErr.Message)
}

// TestServer_ReadUnknownScheme_NotFound verifies a URI outside the guidance
// scheme is also treated as not-found (defensive, no leak of internal state).
func TestServer_ReadUnknownScheme_NotFound(t *testing.T) {
	_, sess := newTestServer(t, newTestApp(), false)

	_, err := sess.ReadResource(context.Background(), &mcpSDK.ReadResourceParams{URI: "mcp-proxmox://nope"})
	require.Error(t, err)

	var rpcErr *jsonrpc.Error
	require.ErrorAs(t, err, &rpcErr)
	assert.Equal(t, int64(jsonrpc.CodeInvalidParams), rpcErr.Code)
}
