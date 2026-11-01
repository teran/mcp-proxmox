package mcp

import (
	"slices"
	"strings"

	"github.com/teran/mcp-proxmox/application"
)

// ToolSurface describes one registered tool for the startup tool-surface log
// (SPEC L10/L11). AlwaysPresent is true for every tool because registration is
// static per run (no per-caller dynamic tools); the field is present so the
// auditable surface log documents it.
type ToolSurface struct {
	Name          string
	AlwaysPresent bool
}

// RegisteredToolSurface returns the exact set of tools that RegisterTools
// registers under the given config (enabled backends + mutation gate): system
// tools always, backend tools only when the backend service is non-nil,
// mutation tools only when enableMutations is true. This mirrors the
// registration logic so the startup log is auditable and matches tools/list.
//
// It reuses the very same per-tool registration functions as RegisterTools,
// passing a nil server and a zero toolLogger so the roTool/mutTool/sysTool
// helpers only compute their ToolSurface instead of registering with the SDK.
// The tool-name source is therefore identical to registration and cannot drift:
// the returned set is, by construction, the set that would be registered.
func RegisteredToolSurface(app *application.App, enableMutations bool) []ToolSurface {
	var surface []ToolSurface
	if app.PVE != nil {
		surface = append(surface, registerPVETools(nil, app, toolLogger{}, enableMutations)...)
	}
	if app.PBS != nil {
		surface = append(surface, registerPBSTools(nil, app, toolLogger{}, enableMutations)...)
	}
	surface = append(surface, registerSystemTools(nil, app, toolLogger{})...)

	// tools/list returns the registered tools sorted by name (the go-sdk
	// featureSet yields features sorted by unique ID), so sort the surface the
	// same way to keep the startup log an exact mirror of what a client sees.
	slices.SortFunc(surface, func(a, b ToolSurface) int {
		return strings.Compare(a.Name, b.Name)
	})

	return surface
}
