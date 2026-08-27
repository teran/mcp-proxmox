package mcp

import (
	"context"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-proxmox/application"
)

// emptyIn is the input of tools that take no parameters.
type emptyIn struct{}

// registerSystemTools registers the system tools: ping and status. They are
// always registered and report which backends are enabled.
func registerSystemTools(s *mcpSDK.Server, app *application.App, log toolLogger) {
	mcpSDK.AddTool(s, &mcpSDK.Tool{
		Name:        "ping",
		Description: "Liveness check; reports which backends (pve, pbs) are enabled.",
	}, func(ctx context.Context, _ *mcpSDK.CallToolRequest, _ emptyIn) (*mcpSDK.CallToolResult, any, error) {
		backends := app.System.Ping(ctx)
		log.Debugf(ctx, "ping: backends=%v", backends)
		return nil, map[string]any{"status": "ok", "backends": backends}, nil
	})

	mcpSDK.AddTool(s, &mcpSDK.Tool{
		Name:        "status",
		Description: "Server status: version, transport, enabled backends.",
	}, func(ctx context.Context, _ *mcpSDK.CallToolRequest, _ emptyIn) (*mcpSDK.CallToolResult, any, error) {
		st := app.System.Status(ctx)
		return nil, st, nil
	})
}
