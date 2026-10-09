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
	sysTool(s, "ping",
		"Liveness check",
		"Liveness check; reports which backends (pve, pbs) are enabled.",
		"Call this to verify the server is responsive. Returns status \"ok\" and the list of enabled backends (pve, pbs). Read-only; no side effects.",
		log, func(ctx context.Context, _ emptyIn) (any, error) {
			backends := app.System.Ping(ctx)
			log.Debugf(ctx, "ping: backends=%v", backends)
			return map[string]any{"status": "ok", "backends": backends}, nil
		})

	sysTool(s, "status",
		"Server status",
		"Server status: version, transport, enabled backends.",
		"Returns the server version, the active transport, and which backends are enabled. Read-only; no side effects.",
		log, func(ctx context.Context, _ emptyIn) (any, error) {
			return app.System.Status(ctx), nil
		})
}

// sysTool registers a system (always-on) tool with its M4/N11 annotations
// (read-only, non-destructive, idempotent, closed world) and per-tool
// instructions, sharing the same uniform handler shape as roTool.
func sysTool[In any, Out any](s *mcpSDK.Server, name, title, desc, instr string, log toolLogger, fn func(context.Context, In) (Out, error)) {
	outSchema := outputSchema[Out]()
	mcpSDK.AddTool(s, &mcpSDK.Tool{
		Name:         name,
		Title:        title,
		Description:  descWithInstructions(desc, instr),
		OutputSchema: outSchema,
		Annotations: &mcpSDK.ToolAnnotations{
			Title:           title,
			ReadOnlyHint:    true,
			IdempotentHint:  true,
			OpenWorldHint:   boolPtr(false),
			DestructiveHint: boolPtr(false),
		},
	}, func(ctx context.Context, _ *mcpSDK.CallToolRequest, in In) (*mcpSDK.CallToolResult, any, error) {
		out, err := fn(ctx, in)
		if err != nil {
			log.Errorf(ctx, "%s: %v", name, err)
			return nil, nil, err
		}
		wrapped, err := wrapAndValidate(ctx, name, log, outSchema, out)
		if err != nil {
			return nil, nil, err
		}
		return nil, wrapped, nil
	})
}
