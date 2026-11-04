package model

import (
	"encoding/json"
	"testing"
)

// TestFlexBoolUnmarshal verifies that FlexBool accepts every form Proxmox VE
// emits for boolean config fields: a JSON boolean, an integer 0/1, a quoted
// "0"/"1"/"true"/"false", and null/empty-string (all -> false).
func TestFlexBoolUnmarshal(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   string
		want bool
	}{
		{"json-true", `true`, true},
		{"json-false", `false`, false},
		{"zero", `0`, false},
		{"one", `1`, true},
		{"quoted-zero", `"0"`, false},
		{"quoted-one", `"1"`, true},
		{"quoted-true", `"true"`, true},
		{"quoted-false", `"false"`, false},
		{"null", `null`, false},
		{"empty-string", `""`, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var f FlexBool
			if err := json.Unmarshal([]byte(tt.in), &f); err != nil {
				t.Fatalf("unmarshal %s: %v", tt.in, err)
			}
			if bool(f) != tt.want {
				t.Fatalf("got %v, want %v", bool(f), tt.want)
			}
		})
	}
}

// TestFlexBoolUnmarshalStartsFromZero verifies that an explicit false input
// resets a previously-true value (guards against a no-op unmarshal).
func TestFlexBoolUnmarshalStartsFromZero(t *testing.T) {
	f := FlexBool(true)
	if err := json.Unmarshal([]byte(`0`), &f); err != nil {
		t.Fatalf("unmarshal 0: %v", err)
	}
	if bool(f) != false {
		t.Fatalf("got %v, want false", bool(f))
	}
}

// TestFlexBoolUnmarshalInvalid verifies that an unrecognized value (after
// trimming optional quotes) returns an error rather than silently decoding to
// false.
func TestFlexBoolUnmarshalInvalid(t *testing.T) {
	for _, in := range []string{`"yes"`, `2`, `"on"`, `"1.0"`} {
		var f FlexBool
		if err := json.Unmarshal([]byte(in), &f); err == nil {
			t.Fatalf("expected an error for %s", in)
		}
	}
}

// TestFlexBoolMarshal verifies FlexBool serializes back as a plain JSON
// boolean.
func TestFlexBoolMarshal(t *testing.T) {
	for _, tt := range []struct {
		in   FlexBool
		want string
	}{
		{FlexBool(true), "true"},
		{FlexBool(false), "false"},
	} {
		b, err := json.Marshal(tt.in)
		if err != nil {
			t.Fatalf("marshal FlexBool(%v): %v", tt.in, err)
		}
		if string(b) != tt.want {
			t.Fatalf("got %s, want %s", b, tt.want)
		}
	}
}
