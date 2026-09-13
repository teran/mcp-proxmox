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
func RegisterTools(s *mcpSDK.Server, app *application.App, log toolLogger, enableMutations bool) {
	// --- Proxmox VE (only when the PVE backend is enabled) ---
	if app.PVE != nil {
		registerPVETools(s, app, log, enableMutations)
	}

	// --- Proxmox Backup Server (only when the PBS backend is enabled) ---
	if app.PBS != nil {
		registerPBSTools(s, app, log, enableMutations)
	}

	// --- system (CORE, always) ---
	registerSystemTools(s, app, log)
}
