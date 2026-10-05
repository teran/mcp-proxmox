package model

import (
	"reflect"
	"testing"
)

// TestPBSVMRestoreRequestSecretTags verifies the S02 conformance fix: the
// PBSVMRestoreRequest fields that carry authentication material
// (Password/Fingerprint) must be tagged `secret:"true"` so any redaction path
// that honors the tag never logs their values. Non-secret fields must NOT carry
// the tag.
func TestPBSVMRestoreRequestSecretTags(t *testing.T) {
	typ := reflect.TypeOf(PBSVMRestoreRequest{})

	for _, field := range []string{"Password", "Fingerprint"} {
		sf, ok := typ.FieldByName(field)
		if !ok {
			t.Fatalf("field %s not found on PBSVMRestoreRequest", field)
		}
		if got := sf.Tag.Get("secret"); got != "true" {
			t.Errorf("PBSVMRestoreRequest.%s must be tagged secret:\"true\", got %q", field, got)
		}
	}

	// A representative non-secret field must not be tagged secret.
	for _, field := range []string{"Target", "VMID", "Host", "Pool", "Verbose", "Reload"} {
		sf, ok := typ.FieldByName(field)
		if !ok {
			t.Fatalf("field %s not found on PBSVMRestoreRequest", field)
		}
		if got := sf.Tag.Get("secret"); got != "" {
			t.Errorf("PBSVMRestoreRequest.%s must NOT be tagged secret, got %q", field, got)
		}
	}
}
