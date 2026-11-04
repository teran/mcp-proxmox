package model

import (
	"bytes"
	"fmt"
)

// FlexBool is a boolean that unmarshals from either a JSON boolean or a JSON
// number (0/1) or a string ("true"/"false"/"0"/"1"), because Proxmox returns
// some boolean config fields as integers (e.g. "enabled":1).
type FlexBool bool

// UnmarshalJSON decodes a FlexBool from a JSON boolean (true), a JSON number
// (1/0), or a quoted string ("true"/"false"/"1"/"0"). JSON null and an empty
// string decode to false.
func (f *FlexBool) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		*f = false

		return nil
	}

	if len(b) >= 2 && b[0] == '"' && b[len(b)-1] == '"' {
		b = b[1 : len(b)-1]
	}

	switch string(b) {
	case "true", "1":
		*f = true
	case "false", "0", "":
		*f = false
	default:
		return fmt.Errorf("invalid value %q for FlexBool", string(b))
	}

	return nil
}

// MarshalJSON serializes a FlexBool as a plain JSON boolean.
func (f FlexBool) MarshalJSON() ([]byte, error) {
	if f {
		return []byte("true"), nil
	}

	return []byte("false"), nil
}
