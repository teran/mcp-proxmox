package model

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestPVERestoreRequestRoundTrip verifies PVERestoreRequest JSON round-trips and
// that only Archive (required) survives when the optionals are zero.
func TestPVERestoreRequestRoundTrip(t *testing.T) {
	in := PVERestoreRequest{
		Archive: "/var/lib/vz/dump/vzdump-qemu-100.vma.zst", VMID: 100,
		Storage: "local", Unique: true, Force: true, Pool: "pool1", BwLimit: 4096,
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got PVERestoreRequest
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("round-trip mismatch\n got: %#v\nwant: %#v", got, in)
	}

	min, err := json.Marshal(PVERestoreRequest{Archive: "/a.bak"})
	if err != nil {
		t.Fatalf("marshal minimal: %v", err)
	}
	if string(min) != `{"archive":"/a.bak"}` {
		t.Fatalf("minimal mismatch: %s", min)
	}
}
