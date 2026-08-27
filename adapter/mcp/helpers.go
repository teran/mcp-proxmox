package mcp

import (
	"context"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"
)

// roTool registers a read-only tool. It wraps mcpSDK.AddTool so every read-only
// handler shares a uniform shape: fn(ctx, in) returns (output, error); on error
// the line is logged via toolLogger (with the tool name) and the error is
// returned so the MCP result is flagged IsError. The output type Out is the
// typed model value returned to the client.
func roTool[In any, Out any](s *mcpSDK.Server, name, desc string, log toolLogger, fn func(context.Context, In) (Out, error)) {
	mcpSDK.AddTool(s, &mcpSDK.Tool{Name: name, Description: desc}, func(ctx context.Context, _ *mcpSDK.CallToolRequest, in In) (*mcpSDK.CallToolResult, Out, error) {
		out, err := fn(ctx, in)
		if err != nil {
			log.Errorf(ctx, "%s: %v", name, err)
			var zero Out
			return nil, zero, err
		}
		return nil, out, nil
	})
}
