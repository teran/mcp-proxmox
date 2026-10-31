package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
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
func roTool[In, Out any](
	s *mcpSDK.Server,
	name, title, desc, instr string,
	log toolLogger,
	handler func(context.Context, In) (Out, error),
) {
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
			OpenWorldHint:   new(bool),
			DestructiveHint: new(bool),
		},
	}, func(ctx context.Context, _ *mcpSDK.CallToolRequest, in In) (*mcpSDK.CallToolResult, any, error) {
		out, err := handler(ctx, in)
		if err != nil {
			if status, ok := port.HTTPStatus(err); ok {
				log.ErrorfStatusf(ctx, status, "%s: %v", name, err)
			} else {
				log.Errorf(ctx, "%s: %v", name, err)
			}

			return nil, nil, err
		}
		wrapped, err := wrapAndValidate(ctx, name, log, outSchema, out)
		if err != nil {
			return nil, nil, err
		}

		return nil, wrapped, nil
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
func mutTool[In, Out any](
	s *mcpSDK.Server,
	name, title, desc, instr string,
	log toolLogger,
	idempotent bool,
	handler func(context.Context, In) (Out, error),
	destructive ...bool,
) {
	destructiveHint := false
	if len(destructive) > 0 {
		destructiveHint = destructive[0]
	}
	outSchema := outputSchema[Out]()
	mcpSDK.AddTool(s, &mcpSDK.Tool{
		Name:         name,
		Title:        title,
		Description:  descWithInstructions(desc, instr),
		OutputSchema: outSchema,
		Annotations: &mcpSDK.ToolAnnotations{
			Title:           title,
			ReadOnlyHint:    false,
			IdempotentHint:  idempotent,
			OpenWorldHint:   new(bool),
			DestructiveHint: &destructiveHint,
		},
	}, func(ctx context.Context, _ *mcpSDK.CallToolRequest, in In) (*mcpSDK.CallToolResult, any, error) {
		out, err := handler(ctx, in)
		if err != nil {
			if status, ok := port.HTTPStatus(err); ok {
				log.ErrorfStatusf(ctx, status, "%s: %v", name, err)
			} else {
				log.Errorf(ctx, "%s: %v", name, err)
			}

			return nil, nil, err
		}
		wrapped, err := wrapAndValidate(ctx, name, log, outSchema, out)
		if err != nil {
			return nil, nil, err
		}

		return nil, wrapped, nil
	})
}

// outputSchema derives the JSON Schema describing a tool's structured output
// (S09/N22). The schema is generated from the typed output type Out using the
// same reflection/jsonschema path the go-sdk uses to derive input schemas
// (jsonschema.ForType), and mirrors the wrapping shape that wrapOutput produces:
//
//   - slices/arrays are wrapped as {"items": <Out>};
//   - scalars are wrapped as {"value": <Out>};
//   - structs and maps pass through unwrapped.
//
// When Out is the empty interface (`any`) — the shape returned by tools whose
// handler erases the concrete type — the runtime shape is unknown, so a
// permissive schema that accepts any JSON value is returned so output is never
// spuriously rejected.
func outputSchema[Out any]() *jsonschema.Schema {
	rt := reflect.TypeFor[Out]()
	elem := rt
	for elem.Kind() == reflect.Pointer {
		elem = elem.Elem()
	}
	if elem.Kind() == reflect.Interface {
		// Out is `any` (or a nil interface): the runtime shape is unknown, so
		// advertise a permissive schema that accepts any JSON value.

		return &jsonschema.Schema{}
	}
	base, err := jsonschema.ForType(elem, &jsonschema.ForOptions{})
	if err != nil || base == nil {
		return &jsonschema.Schema{}
	}
	switch elem.Kind() {
	case reflect.Slice, reflect.Array:
		return &jsonschema.Schema{
			Type:       "object",
			Properties: map[string]*jsonschema.Schema{"items": base},
		}
	case reflect.Struct, reflect.Map:
		return base
	default:
		// scalar (int, string, bool, float, ...)

		return &jsonschema.Schema{
			Type:       "object",
			Properties: map[string]*jsonschema.Schema{"value": base},
		}
	}
}

// validateOutput reports whether the value that will reach the MCP client
// conforms to the tool's outputSchema (S09). It marshals the (already wrapped
// and sanitized) value to JSON and validates the resulting instance against the
// resolved schema, mirroring the go-sdk's own output validation. A nil schema
// or nil output short-circuits to success (nothing to validate).
func validateOutput(schema *jsonschema.Schema, out any) error {
	if schema == nil || out == nil {
		return nil
	}
	resolved, err := schema.Resolve(&jsonschema.ResolveOptions{ValidateDefaults: true})
	if err != nil {
		return fmt.Errorf("resolving output schema: %w", err)
	}
	b, err := json.Marshal(out)
	if err != nil {
		return fmt.Errorf("marshaling output: %w", err)
	}
	var unmarshaled any
	if err := json.Unmarshal(b, &unmarshaled); err != nil {
		return fmt.Errorf("unmarshaling output: %w", err)
	}
	if err := resolved.Validate(&unmarshaled); err != nil {
		return fmt.Errorf("output does not conform to outputSchema: %w", err)
	}

	return nil
}

// wrapAndValidate wraps the tool output (via wrapOutput) and validates the
// result against the tool's outputSchema before it reaches the client (S09/N22).
// A non-conforming result is logged via toolLogger (with the tool name) and
// returned as an error so the MCP result is flagged IsError instead of emitting
// non-conforming output.
func wrapAndValidate(ctx context.Context, name string, log toolLogger, schema *jsonschema.Schema, out any) (any, error) {
	wrapped := wrapOutput(out)
	if err := validateOutput(schema, wrapped); err != nil {
		log.Errorf(ctx, "%s: %v", name, err)

		return nil, err
	}

	return wrapped, nil
}

// wrapOutput ensures the value returned to the MCP client is a JSON object, as
// required for structuredContent. A top-level slice (a list tool) or scalar
// (e.g. pve_nextid) is wrapped in a small object; structs and maps pass through.
// The result is then passed through sanitizeOutput (S2/S9): secret fields (a
// `secret:"true"` struct tag or a sensitive key name) are redacted and
// ANSI/control escape sequences are stripped from string values, so no secret
// and no raw control bytes can reach output. Redaction is reflect-based, so it
// applies to typed structs and typed slices too, not just map[string]any.
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
// client (the wrapper shapes produced by wrapOutput — maps, slices, structs and
// scalars) and:
//   - redacts any secret field to "[redacted]" (S2): a struct field tagged
//     `secret:"true"`, a struct field whose json name looks sensitive, or a map
//     key whose name looks sensitive, so a secret can never reach tool output;
//   - strips ANSI/control escape sequences from string values (S9).
//
// Redaction is reflect-based, so typed structs and typed slices are walked as
// well as map[string]any. The critical invariant (S02) is that a value which
// contains no secret/sensitive field and no control character is returned
// unchanged — the original typed value is preserved so `reflect.DeepEqual`
// holds and the validateOutput/outputSchema validation of typed models is
// unaffected. Only when there is actually something to redact or strip is a
// rebuilt (map/[]any) form returned.
func sanitizeOutput(v any) any {
	if v == nil {
		return v
	}
	val := reflect.ValueOf(v)
	if !val.IsValid() {
		return v
	}
	// Dereference pointers/interfaces first; a nil pointer passes through.
	for val.Kind() == reflect.Pointer || val.Kind() == reflect.Interface {
		if val.IsNil() {
			return v
		}
		val = val.Elem()
	}
	// Preserve the original typed value when nothing needs redaction/stripping
	// (S02): only rebuild containers/structs that actually carry a secret or a
	// control character.
	if !needsRedaction(val) {
		return v
	}

	return sanitizeValue(val)
}

// needsRedaction reports whether v (or anything reachable from it) requires a
// transformation by sanitizeValue: a secret field (a `secret:"true"` struct tag
// or a sensitive json/key name) or a string containing ANSI/control escape
// sequences. When false, the original typed value is passed through unchanged,
// preserving `reflect.DeepEqual` and outputSchema validation of typed models.
func needsRedaction(v reflect.Value) bool {
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return false
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.String:
		return strings.ContainsAny(v.String(), controlChars)
	case reflect.Map:
		iter := v.MapRange()
		for iter.Next() {
			if iter.Key().Kind() == reflect.String && isSensitiveKey(iter.Key().String()) {
				return true
			}
			if needsRedaction(iter.Value()) {
				return true
			}
		}

		return false
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			if needsRedaction(v.Index(i)) {
				return true
			}
		}

		return false
	case reflect.Struct:
		return structNeedsRedaction(v)
	default:
		return false
	}
}

// structNeedsRedaction reports whether a struct value carries a secret field
// (a `secret:"true"` tag or a sensitive json name) or a nested value that needs
// redaction.
func structNeedsRedaction(v reflect.Value) bool {
	t := v.Type()
	for i := range v.NumField() {
		sf := t.Field(i)
		if sf.PkgPath != "" {
			continue // skip unexported fields
		}
		if sf.Tag.Get("secret") == "true" || isSensitiveKey(structFieldName(sf)) {
			return true
		}
		if needsRedaction(v.Field(i)) {
			return true
		}
	}

	return false
}

// sanitizeValue builds the redacted/stripped form of a value known to require a
// transformation (needsRedaction returned true). Structs and maps are rebuilt as
// map[string]any keyed by json/field names, slices/arrays as []any, and strings
// are control-stripped; the "[redacted]" marker replaces secret fields. Values
// that individually need no transformation are preserved as their original typed
// interface values.
func sanitizeValue(v reflect.Value) any {
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return v.Interface()
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.String:
		return stripControl(v.String())
	case reflect.Map:
		out := make(map[string]any, v.Len())
		iter := v.MapRange()
		for iter.Next() {
			key := mapKeyString(iter.Key())
			if isSensitiveKey(key) {
				out[key] = "[redacted]"

				continue
			}
			out[key] = sanitizeOrPreserve(iter.Value())
		}

		return out
	case reflect.Slice:
		out := make([]any, v.Len())
		for i := range v.Len() {
			out[i] = sanitizeOrPreserve(v.Index(i))
		}

		return out
	case reflect.Array:
		out := make([]any, v.Len())
		for i := range v.Len() {
			out[i] = sanitizeOrPreserve(v.Index(i))
		}

		return out
	case reflect.Struct:
		return sanitizeStruct(v)
	default:
		return v.Interface()
	}
}

// sanitizeStruct rebuilds a struct as a map[string]any, redacting any field
// tagged secret:"true" or with a sensitive json name, and recursively sanitizing
// the other fields.
func sanitizeStruct(v reflect.Value) map[string]any {
	t := v.Type()
	out := make(map[string]any, v.NumField())
	for i := range v.NumField() {
		sf := t.Field(i)
		if sf.PkgPath != "" {
			continue // skip unexported fields
		}
		name := structFieldName(sf)
		if sf.Tag.Get("secret") == "true" || isSensitiveKey(name) {
			out[name] = "[redacted]"

			continue
		}
		out[name] = sanitizeOrPreserve(v.Field(i))
	}

	return out
}

// sanitizeOrPreserve returns the sanitized form of a nested value, or its
// original typed interface value when it needs no transformation.
func sanitizeOrPreserve(v reflect.Value) any {
	if needsRedaction(v) {
		return sanitizeValue(v)
	}

	return v.Interface()
}

// structFieldName returns the wire name of a struct field: its json tag name
// (with `,omitempty` etc. stripped), falling back to the Go field name. A json
// tag of "-" means the field is not serialized, so its name cannot be sensitive.
func structFieldName(sf reflect.StructField) string {
	if tag := sf.Tag.Get("json"); tag != "" && tag != "-" {
		return strings.Split(tag, ",")[0]
	}

	return sf.Name
}

// mapKeyString renders a map key as a string for sensitivity checks and output.
func mapKeyString(k reflect.Value) string {
	if k.Kind() == reflect.String {
		return k.String()
	}

	return fmt.Sprint(k.Interface())
}

// controlChars is the set of bytes that must never reach tool output: ESC and
// the C0 control characters except whitespace (\t \n \r). Shared by
// stripControl and needsRedaction.
const controlChars = "\x1b\x00\x01\x02\x03\x04\x05\x06\x07\x08\x0b\x0c\x0e\x0f\x10\x11\x12\x13\x14\x15\x16" +
	"\x17\x18\x19\x1a\x1c\x1d\x1e\x1f"

// stripControl removes ANSI escape sequences and C0 control characters (except
// whitespace) from a string, so tool output cannot carry raw escape bytes (S9).
// Handles CSI sequences ("\x1b[31m"), two-char sequences ("\x1bM") and bare
// control bytes; the entire escape sequence is dropped.
func stripControl(s string) string {
	// escByte is the ASCII ESC (0x1b) that begins every escape sequence.
	const escByte = 0x1b
	if !strings.ContainsAny(s, controlChars) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == escByte {
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
