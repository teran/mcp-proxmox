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

// TestFlexIntUnmarshalInvalid verifies that a non-numeric value (after
// trimming optional quotes) returns an error rather than silently decoding to
// zero.
func TestFlexIntUnmarshalInvalid(t *testing.T) {
	var f FlexInt
	if err := json.Unmarshal([]byte(`"not-a-number"`), &f); err == nil {
		t.Fatal("expected an error for a non-numeric FlexInt value")
	}
}

// TestFlexIntMarshalNumber verifies FlexInt serializes back as a plain JSON
// number.
func TestFlexIntMarshalNumber(t *testing.T) {
	b, err := json.Marshal(FlexInt(42))
	if err != nil {
		t.Fatalf("marshal FlexInt: %v", err)
	}
	if string(b) != "42" {
		t.Fatalf("got %s, want 42", b)
	}
}

// TestFlexIntUnmarshalNullAndEmpty verifies that JSON null and an empty string
// both decode to zero.
func TestFlexIntUnmarshalNullAndEmpty(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   string
	}{
		{"null", `null`},
		{"empty-string", `""`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var f FlexInt = 42
			if err := json.Unmarshal([]byte(tt.in), &f); err != nil {
				t.Fatalf("unmarshal %q: %v", tt.in, err)
			}
			if f != FlexInt(0) {
				t.Fatalf("got %d, want 0", f)
			}
		})
	}
}
