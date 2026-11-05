package model

import (
	"bytes"
	"testing"
)

// TestIsEmptyValue verifies the omitempty semantics of isEmptyValue across all
// the concrete types it switches on (string, FlexInt, int, bool) plus a type
// outside the switch (falls through to false). This covers the previously
// uncovered `case int` branch.
func TestIsEmptyValue(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want bool
	}{
		{"int-zero", int(0), true},
		{"int-nonzero", int(5), false},
		{"int-negative", int(-1), false},
		{"string-empty", "", true},
		{"string-nonempty", "web", false},
		{"flexint-zero", FlexInt(0), true},
		{"flexint-nonzero", FlexInt(3), false},
		{"bool-false", false, true},
		{"bool-true", true, false},
		{"other-type-fallback", struct{}{}, false},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := isEmptyValue(tt.in); got != tt.want {
				t.Errorf("isEmptyValue(%#v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestWriteVMConfigFieldMarshalError verifies that writeVMConfigField surfaces
// an error when the value cannot be serialized to JSON (e.g. an unsupported
// type). This exercises the `return err` branches on the value-marshal path,
// which are not reachable via the string-typed device maps.
func TestWriteVMConfigFieldMarshalError(t *testing.T) {
	var buf bytes.Buffer
	first := true
	if err := writeVMConfigField(&buf, &first, "bad", make(chan int), false); err == nil {
		t.Fatal("writeVMConfigField: expected error marshaling unsupported type chan int, got nil")
	}
}
