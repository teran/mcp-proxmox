package model

import (
	"encoding/json"
	"testing"
)

// TestStorageStatusDecodeRealResponse pins the exact JSON element the real
// Proxmox VE API emits for GET /nodes/{node}/storage/{storage}/status. In
// particular PVE returns "enabled" as an integer (1/0), which StorageStatus
// must decode via FlexBool.
func TestStorageStatusDecodeRealResponse(t *testing.T) {
	const raw = `{"active":1,"total":17614130642944,"enabled":1,"used":11293970490000,` +
		`"type":"zfspool","avail":6320160152944,"content":"images,rootdir","shared":0}`

	var s StorageStatus
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("unmarshal real storage status: %v", err)
	}

	if s.Enabled != FlexBool(true) {
		t.Errorf("Enabled: got %v, want true", s.Enabled)
	}
	if s.Active != 1 {
		t.Errorf("Active: got %d, want 1", s.Active)
	}
	if s.Total != 17614130642944 {
		t.Errorf("Total: got %d, want 17614130642944", s.Total)
	}
	if s.Avail != 6320160152944 {
		t.Errorf("Avail: got %d, want 6320160152944", s.Avail)
	}
	if s.Type != "zfspool" {
		t.Errorf("Type: got %q, want zfspool", s.Type)
	}
}

// TestStorageStatusDecodeDisabled verifies that "enabled":0 decodes to false,
// guarding the integer-to-boolean path for a disabled storage.
func TestStorageStatusDecodeDisabled(t *testing.T) {
	const raw = `{"active":1,"total":1000,"enabled":0,"used":500,"type":"dir","avail":500,"content":"images","shared":0}`

	var s StorageStatus
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if s.Enabled != FlexBool(false) {
		t.Errorf("Enabled: got %v, want false", s.Enabled)
	}
}

// TestStorageDecodeRealResponse pins the real GET /storage element, which also
// carries "enabled" as an integer.
func TestStorageDecodeRealResponse(t *testing.T) {
	const raw = `{"storage":"zfspool","type":"zfspool","status":"available",` +
		`"content":"images,rootdir","nodes":"pve1","shared":0,"active":1,"enabled":1,"path":"/var/lib/vz"}`

	var s Storage
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatalf("unmarshal real storage: %v", err)
	}

	if s.Enabled != FlexBool(true) {
		t.Errorf("Enabled: got %v, want true", s.Enabled)
	}
	if s.Active != 1 {
		t.Errorf("Active: got %d, want 1", s.Active)
	}
	if s.Storage != "zfspool" {
		t.Errorf("Storage: got %q, want zfspool", s.Storage)
	}
}
