package port

import (
	"errors"
	"testing"
)

// TestSentinelErrors verifies the sentinel errors are defined and carry the
// expected messages (SPEC.md §7.1).
func TestSentinelErrors(t *testing.T) {
	if ErrUnauthorized == nil || ErrForbidden == nil || ErrNotFound == nil {
		t.Fatal("sentinel errors must be non-nil")
	}
	if got := ErrUnauthorized.Error(); got != "proxmox: unauthorized" {
		t.Fatalf("ErrUnauthorized message = %q, want %q", got, "proxmox: unauthorized")
	}
	if got := ErrForbidden.Error(); got != "proxmox: forbidden" {
		t.Fatalf("ErrForbidden message = %q, want %q", got, "proxmox: forbidden")
	}
	if got := ErrNotFound.Error(); got != "proxmox: not found" {
		t.Fatalf("ErrNotFound message = %q, want %q", got, "proxmox: not found")
	}
	if !errors.Is(ErrUnauthorized, ErrUnauthorized) {
		t.Fatal("errors.Is(sentinel, itself) should be true")
	}
}
