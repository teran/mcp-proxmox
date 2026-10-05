package model

import (
	"bytes"
	"strconv"
)

// FlexInt is an integer that unmarshals from either a JSON number or a JSON
// string, because Proxmox returns some numeric config fields as strings
// (e.g. "memory":"8192").
type FlexInt int

// UnmarshalJSON decodes a FlexInt from either a JSON number (8192) or a JSON
// string ("8192"). JSON null and an empty string decode to zero.
func (f *FlexInt) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		*f = 0
		return nil
	}

	if len(b) >= 2 && b[0] == '"' && b[len(b)-1] == '"' {
		b = b[1 : len(b)-1]
	}

	if len(b) == 0 {
		*f = 0
		return nil
	}

	n, err := strconv.Atoi(string(b))
	if err != nil {
		return err
	}
	*f = FlexInt(n)
	return nil
}

// MarshalJSON serializes a FlexInt as a plain JSON number.
func (f FlexInt) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Itoa(int(f))), nil
}
