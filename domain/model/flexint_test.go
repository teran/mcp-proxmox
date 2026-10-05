package model

import (
	"encoding/json"
	"testing"
)

// TestFlexIntUnmarshalStringNumber verifies that FlexInt accepts a JSON
// string containing an integer (as Proxmox VE's /config endpoints emit it,
// e.g. "memory":"8192").
func TestFlexIntUnmarshalStringNumber(t *testing.T) {
	var f FlexInt
	if err := json.Unmarshal([]byte(`"8192"`), &f); err != nil {
		t.Fatalf("unmarshal string number: %v", err)
	}
	if f != FlexInt(8192) {
		t.Fatalf("got %d, want 8192", f)
	}
}

// TestFlexIntUnmarshalNumber verifies that FlexInt also accepts a plain JSON
// number.
func TestFlexIntUnmarshalNumber(t *testing.T) {
	var f FlexInt
	if err := json.Unmarshal([]byte(`8192`), &f); err != nil {
		t.Fatalf("unmarshal number: %v", err)
	}
	if f != FlexInt(8192) {
		t.Fatalf("got %d, want 8192", f)
	}
}

// TestFlexIntUnmarshalNullAndEmpty verifies that JSON null and an empty string
// both decode to zero.
func TestFlexIntUnmarshalNullAndEmpty(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
	}{
		{"null", `null`},
		{"empty-string", `""`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var f FlexInt = 42
			if err := json.Unmarshal([]byte(tc.in), &f); err != nil {
				t.Fatalf("unmarshal %q: %v", tc.in, err)
			}
			if f != FlexInt(0) {
				t.Fatalf("got %d, want 0", f)
			}
		})
	}
}
