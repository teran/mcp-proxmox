package mcp

import (
	"context"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"
)

// guidanceNamespace is the scheme used by the self-describing guidance
// resources. Every registered resource lives under this namespace.
const guidanceNamespace = "mcp-proxmox://guidance/"

// resourceURIs lists the guidance resources exposed by this server, keyed by
// their short name (the part after the namespace). The map is used both to
// register each resource and to resolve read requests to their markdown body.
var resourceURIs = map[string]string{
	"overview": guidanceNamespace + "overview",
	"tools":    guidanceNamespace + "tools",
	"security": guidanceNamespace + "security",
}

// RegisterGuidanceResources registers read-only markdown guidance resources
// under the mcp-proxmox://guidance/* namespace (M08/M09), making the server
// self-describing over the protocol. Unknown URIs resolve to a not-found error.
// Registering the resources causes the SDK to advertise the `resources`
// capability automatically. See the package doc / SPEC.md §2.5, §4.8, §6.
func RegisterGuidanceResources(s *mcpSDK.Server) {
	for name, uri := range resourceURIs {
		s.AddResource(
			&mcpSDK.Resource{
				URI:         uri,
				Name:        name,
				Title:       guidanceTitle(name),
				Description: guidanceDescription(name),
				MIMEType:    "text/markdown",
			},
			guidanceHandler(name),
		)
	}
}

// guidanceHandler returns the read handler for the named guidance resource. It
// returns the resource's markdown body for a known name and a
// ResourceNotFoundError for any other URI.
func guidanceHandler(name string) mcpSDK.ResourceHandler {
	return func(_ context.Context, req *mcpSDK.ReadResourceRequest) (*mcpSDK.ReadResourceResult, error) {
		uri := ""
		if p, ok := req.GetParams().(*mcpSDK.ReadResourceParams); ok {
			uri = p.URI
		}
		if uri != resourceURIs[name] {
			return nil, mcpSDK.ResourceNotFoundError(uri)
		}

		return &mcpSDK.ReadResourceResult{
			Contents: []*mcpSDK.ResourceContents{
				{URI: uri, MIMEType: "text/markdown", Text: guidanceBody(name)},
			},
		}, nil
	}
}

// guidanceTitle returns a short human-readable title for a guidance resource.
func guidanceTitle(name string) string {
	switch name {
	case "overview":
		return "Server overview"
	case "tools":
		return "Tool surface"
	case "security":
		return "Secrets & data hygiene"
	default:
		return ""
	}
}

// guidanceDescription returns a one-line description for a guidance resource.
func guidanceDescription(name string) string {
	switch name {
	case "overview":
		return "What mcp-proxmox is and how its optional backends are enabled."
	case "tools":
		return "How the tool surface is grouped (read → write → delete) by domain prefix."
	case "security":
		return "How API tokens and TLS are handled; no per-call credentials."
	default:
		return ""
	}
}

// guidanceBody returns the markdown body for a guidance resource.
func guidanceBody(name string) string {
	switch name {
	case "overview":
		return guidanceOverview
	case "tools":
		return guidanceTools
	case "security":
		return guidanceSecurity
	default:
		return ""
	}
}

// guidanceOverview describes what the server is and how its optional backends
// are enabled.
const guidanceOverview = "# mcp-proxmox\n" +
	"\n" +
	"A local stdio MCP server exposing the **Proxmox VE** and **Proxmox Backup " +
	"Server (PBS)** REST APIs as callable tools for LLM agents, IDEs and " +
	"assistants.\n" +
	"\n" +
	"## Optional backends\n" +
	"\n" +
	"Both backends are **optional** and enabled independently. A backend is " +
	"active only when **both** its `endpoint` and its `token` are configured " +
	"(via environment variables `PVE_ENDPOINT`/`PVE_TOKEN` and " +
	"`PBS_ENDPOINT`/`PBS_TOKEN`). When a backend is not configured it is simply " +
	"absent — its tools are not registered and nothing fails. `ping`/`status` " +
	"are always available and report which backends are active.\n" +
	"\n" +
	"Read the **tools** and **security** resources in this namespace for the " +
	"tool surface and data-hygiene guarantees.\n"

// guidanceTools describes how the tool surface is grouped.
const guidanceTools = "# Tool surface\n" +
	"\n" +
	"Tools are grouped by a domain prefix and by operation kind:\n" +
	"\n" +
	"- `pve_` — Proxmox VE (nodes, VMs, LXC containers, storage, cluster).\n" +
	"- `pbs_` — Proxmox Backup Server (datastores, snapshots, verify, prune).\n" +
	"- `ping` / `status` — system tools, always available.\n" +
	"\n" +
	"Within each domain, tools are grouped read → write → delete:\n" +
	"\n" +
	"- **Read-only / query tools** (e.g. `pve_vm_list`) are always registered " +
	"for an enabled backend.\n" +
	"- **Mutation tools** (write/delete) are registered **only when " +
	"`ENABLE_MUTATIONS=true`** (default off). Mutation tool descriptions " +
	"carry per-tool `Instructions:` guidance and destructive tools are gated " +
	"accordingly.\n"

// guidanceSecurity describes token handling and TLS guarantees.
const guidanceSecurity = "# Secrets & data hygiene\n" +
	"\n" +
	"- **API tokens** are configured once per backend via environment variables " +
	"(`PVE_TOKEN` / `PBS_TOKEN`) and are **never** per-call tool parameters and " +
	"**never** logged or exposed in tool output.\n" +
	"- **TLS verification is always on.** Self-signed/custom CAs are supported " +
	"only via a per-backend `ca_cert_path` used to build the client TLS " +
	"`RootCAs` — there is no insecure-skip-verify switch.\n" +
	"- This is a **Local stdio** server: the only file I/O is reading the " +
	"configured `ca_cert_path` and (when `LOG_LEVEL` is set) writing to " +
	"`LOG_FILENAME`. No tool argument names a local path the server accesses.\n" +
	"- Logging is disabled by default; when enabled, secrets and request/" +
	"response bodies are never written to the journal.\n"
