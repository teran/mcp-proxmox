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

// TestStorageContentDecodeRealResponse pins the exact JSON element the real
// Proxmox VE API emits for GET /nodes/{node}/storage/{storage}/content
// (SPEC.md §6.x). Every field of StorageContent must decode.
func TestStorageContentDecodeRealResponse(t *testing.T) {
	const raw = `{"volid":"local:iso/debian-12.iso","content":"iso","format":"iso",` +
		`"size":513802240,"ctime":1700000000,"vmid":0}`

	var c StorageContent
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("unmarshal real storage content: %v", err)
	}

	if c.VolID != "local:iso/debian-12.iso" {
		t.Errorf("VolID: got %q, want local:iso/debian-12.iso", c.VolID)
	}
	if c.Content != "iso" {
		t.Errorf("Content: got %q, want iso", c.Content)
	}
	if c.Format != "iso" {
		t.Errorf("Format: got %q, want iso", c.Format)
	}
	if c.Size != 513802240 {
		t.Errorf("Size: got %d, want 513802240", c.Size)
	}
	if c.CTime != 1700000000 {
		t.Errorf("CTime: got %d, want 1700000000", c.CTime)
	}
	if c.VMID != 0 {
		t.Errorf("VMID: got %d, want 0", c.VMID)
	}
}

// TestStorageContentDecodeVMEntry verifies a VM image entry (content=rootdir,
// non-zero vmid) decodes correctly.
func TestStorageContentDecodeVMEntry(t *testing.T) {
	const raw = `{"volid":"local:203/vm-203-disk-0.raw","content":"rootdir","format":"raw",` +
		`"size":8589934592,"ctime":1700000001,"vmid":203}`

	var c StorageContent
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if c.VolID != "local:203/vm-203-disk-0.raw" {
		t.Errorf("VolID: got %q", c.VolID)
	}
	if c.Content != "rootdir" {
		t.Errorf("Content: got %q", c.Content)
	}
	if c.Format != "raw" {
		t.Errorf("Format: got %q", c.Format)
	}
	if c.Size != 8589934592 {
		t.Errorf("Size: got %d", c.Size)
	}
	if c.VMID != 203 {
		t.Errorf("VMID: got %d, want 203", c.VMID)
	}
}

// TestStorageContentDecodeMissingFields verifies that an element missing the
// optional fields (content/format/size/ctime/vmid) decodes to a zero struct
// without error, so a sparse API element cannot break the whole list.
func TestStorageContentDecodeMissingFields(t *testing.T) {
	const raw = `{"volid":"local:iso/tiny.iso"}`

	var c StorageContent
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("unmarshal sparse element: %v", err)
	}

	if c.VolID != "local:iso/tiny.iso" {
		t.Errorf("VolID: got %q", c.VolID)
	}
	if c.Content != "" || c.Format != "" || c.Size != 0 || c.CTime != 0 || c.VMID != 0 {
		t.Errorf("missing fields must decode to zero values, got %+v", c)
	}
}

// TestStorageContentRoundTrip verifies StorageContent marshals and unmarshals
// without loss.
func TestStorageContentRoundTrip(t *testing.T) {
	in := StorageContent{
		VolID: "local:203/vm-203-disk-0.raw", Content: "rootdir", Format: "raw",
		Size: 8589934592, CTime: 1700000001, VMID: 203,
	}

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out StorageContent
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out != in {
		t.Fatalf("round-trip mismatch\n got: %#v\nwant: %#v", out, in)
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
