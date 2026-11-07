package mcp

import (
	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-proxmox/application"
)

// RegisterTools registers all tools on the server.
//
// System tools (ping/status) are always registered. A backend's tools are
// registered only when its service is non-nil (i.e. the backend is enabled).
// Mutation tools are registered only when enableMutations is true (from the
// ENABLE_MUTATIONS env var, SPEC.md §2.5); read-only tools are always
// registered for an enabled backend.
//
// Each register* function returns the ToolSurfaces of the tools it registered;
// RegisterTools discards them because registration itself is the effect. The
// same functions, called with a nil server, back RegisteredToolSurface so the
// auditable startup log matches tools/list (SPEC L10/L11).
func RegisterTools(s *mcpSDK.Server, app *application.App, log toolLogger, enableMutations bool) {
	// --- Proxmox VE (only when the PVE backend is enabled) ---
	if app.PVE != nil {
		_ = registerPVETools(s, app, log, enableMutations)
	}

	// --- PVE-managed Ceph (rides the PVE backend; only when it is enabled) ---
	if app.Ceph != nil {
		_ = registerCephTools(s, app, log, enableMutations)
	}

	// --- Proxmox Backup Server (only when the PBS backend is enabled) ---
	if app.PBS != nil {
		_ = registerPBSTools(s, app, log, enableMutations)
	}

	// --- system (CORE, always) ---
	_ = registerSystemTools(s, app, log)
}
