package mcp

import (
	"context"
	"reflect"
	"strings"

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

// mutTool registers a mutation tool (gated by EnableMutations). It mirrors
// roTool's uniform handler shape but with write annotations: ReadOnlyHint=false
// and, depending on the operation, IdempotentHint. `idempotent` is true only for
// genuinely safe-to-repeat operations (resize/migrate/ha-add) and false for
// everything that starts a fresh side-effect, including create/clone/restore.
// `destructive` is variadic and defaults to false; pass true only for truly
// destructive delete operations (S12) so their DestructiveHint is set, while
// every other mutation stays non-destructive.
func mutTool[In, Out any](s *mcpSDK.Server, name, title, desc, instr string, log toolLogger, idempotent bool, fn func(context.Context, In) (Out, error), destructive ...bool) {
	destructiveHint := false
	if len(destructive) > 0 {
		destructiveHint = destructive[0]
	}
	mcpSDK.AddTool(s, &mcpSDK.Tool{
		Name:        name,
		Title:       title,
		Description: descWithInstructions(desc, instr),
		Annotations: &mcpSDK.ToolAnnotations{
			Title:           title,
			ReadOnlyHint:    false,
			IdempotentHint:  idempotent,
			OpenWorldHint:   boolPtr(false),
			DestructiveHint: boolPtr(destructiveHint),
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

// boolPtr returns a pointer to b (for the SDK's *bool annotation hints).
func boolPtr(b bool) *bool { return &b }

// wrapOutput ensures the value returned to the MCP client is a JSON object, as
// required for structuredContent. A top-level slice (a list tool) or scalar
// (e.g. pve_nextid) is wrapped in a small object; structs and maps pass through.
// The result is then passed through sanitizeOutput (S2/S9): fields tagged
// `secret:"true"` are redacted and ANSI/control escape sequences are stripped
// from string values, so no secret and no raw control bytes can reach output.
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
	var wrapped any
	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		wrapped = map[string]any{"items": out}
	case reflect.Struct, reflect.Map:
		wrapped = out
	default:
		// scalar (int, string, bool, float, ...)
		wrapped = map[string]any{"value": out}
	}
	return sanitizeOutput(wrapped)
}

// sanitizeOutput recursively walks the value that will be returned to the MCP
// client (the wrapper shapes produced by wrapOutput — maps, slices and scalars)
// and:
//   - redacts any map key whose name looks sensitive (S2), so a secret can never
//     reach tool output;
//   - strips ANSI/control escape sequences from string values (S9).
//
// Typed structs returned by wrapOutput pass through unchanged — they are the
// domain models with `json`/`omitempty` tags that the SDK validates against the
// derived outputSchema; none of them carries a secret (API tokens never enter
// the model or the output path, §5.4). Map-based output is sanitized defensively.
func sanitizeOutput(v any) any {
	switch t := v.(type) {
	case string:
		return stripControl(t)
	case []any:
		for i := range t {
			t[i] = sanitizeOutput(t[i])
		}
		return t
	case map[string]any:
		for k, val := range t {
			if isSensitiveKey(k) {
				t[k] = "[redacted]"
				continue
			}
			t[k] = sanitizeOutput(val)
		}
		return t
	default:
		return v
	}
}

// stripControl removes ANSI escape sequences and C0 control characters (except
// whitespace) from a string, so tool output cannot carry raw escape bytes (S9).
// Handles CSI sequences ("\x1b[31m"), two-char sequences ("\x1bM") and bare
// control bytes; the entire escape sequence is dropped.
func stripControl(s string) string {
	if !strings.ContainsAny(s, "\x1b\x00\x01\x02\x03\x04\x05\x06\x07\x08\x0b\x0c\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1c\x1d\x1e\x1f") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == 0x1b {
			// ESC [ ... final (CSI): skip '[' plus parameter/intermediate bytes
			// (0x20..0x3F) until a final byte in 0x40..0x7E.
			if i+1 < len(s) && s[i+1] == '[' {
				i += 2
				for i < len(s) {
					nc := s[i]
					i++
					if nc >= 0x40 && nc <= 0x7e {
						break
					}
				}
				i-- // the final byte was consumed; the loop increments
				continue
			}
			// Two-char sequence: ESC + one byte.
			i++
			continue
		}
		if c < 0x20 && c != '\t' && c != '\n' && c != '\r' {
			// drop other C0 control characters
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}
