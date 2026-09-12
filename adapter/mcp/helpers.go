package mcp

import (
	"context"
	"reflect"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-proxmox/domain/port"
)

// roTool registers a read-only tool. It wraps mcpSDK.AddTool so every read-only
// handler shares a uniform shape: fn(ctx, in) returns (output, error); on error
// the line is logged via toolLogger (with the tool name) and the error is
// returned so the MCP result is flagged IsError. The output type Out is the
// typed model value returned to the client. The handler's output is passed
// through wrapOutput so the MCP structuredContent is always a JSON object (an
// MCP requirement): top-level slices and scalars are wrapped in an object.
//
// Each tool is described with its M4/N11 metadata: a human-readable title, the
// ToolAnnotations hints (read-only, non-destructive, idempotent, closed world)
// and per-tool natural-language instructions (encoded into the SDK Description,
// which the go-sdk treats as the model hint).
func roTool[In, Out any](s *mcpSDK.Server, name, title, desc, instr string, log toolLogger, fn func(context.Context, In) (Out, error)) {
	mcpSDK.AddTool(s, &mcpSDK.Tool{
		Name:        name,
		Title:       title,
		Description: descWithInstructions(desc, instr),
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
			if status, ok := port.HTTPStatus(err); ok {
				log.ErrorfStatus(ctx, status, "%s: %v", name, err)
			} else {
				log.Errorf(ctx, "%s: %v", name, err)
			}
			return nil, nil, err
		}
		return nil, wrapOutput(out), nil
	})
}

// descWithInstructions appends natural-language per-tool instructions to the
// tool description. The go-sdk's Tool.Description is the field treated as the
// model hint, so it is the right place to carry the per-tool instructions
// (SPEC.md M4/N11).
func descWithInstructions(desc, instr string) string {
	if instr == "" {
		return desc
	}
	return desc + "\n\nInstructions: " + instr
}

// boolPtr returns a pointer to b (for the SDK's *bool annotation hints).
func boolPtr(b bool) *bool { return &b }

// wrapOutput ensures the value returned to the MCP client is a JSON object, as
// required for structuredContent. A top-level slice (a list tool) or scalar
// (e.g. pve_nextid) is wrapped in a small object; structs and maps pass through.
func wrapOutput(out any) any {
	if out == nil {
		return out
	}
	v := reflect.ValueOf(out)
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return out
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		return map[string]any{"items": out}
	case reflect.Struct, reflect.Map:
		return out
	default:
		// scalar (int, string, bool, float, ...)
		return map[string]any{"value": out}
	}
}
