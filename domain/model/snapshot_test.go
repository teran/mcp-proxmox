package model

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestSnapshotRoundTrip verifies Snapshot and SnapshotCreateRequest marshal to
// the exact JSON the PVE snapshot endpoints emit and unmarshal without loss.
func TestSnapshotRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		in   any
	}{
		{
			name: "snapshot",
			in: Snapshot{
				Name: "snap1", Description: "pre-upgrade", Snaptime: 1704067200,
				VMState: 1, Parent: "current", Type: "snapshot", Ram: 256, Disk: 10,
			},
		},
		{
			name: "snapshot-create",
			in:   SnapshotCreateRequest{Snapname: "snap2", VMState: true, Description: "with ram"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			typ := reflect.TypeOf(tt.in)
			rv := reflect.New(typ)
			if err := json.Unmarshal(b, rv.Interface()); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if !reflect.DeepEqual(rv.Elem().Interface(), tt.in) {
				t.Fatalf("round-trip mismatch\n got: %#v\nwant: %#v", rv.Elem().Interface(), tt.in)
			}
		})
	}
}

// TestSnapshotExactJSON pins the exact wire format for the snapshot types.
func TestSnapshotExactJSON(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{
			name: "snapshot-full",
			in: Snapshot{
				Name: "snap1", Description: "d", Snaptime: 1704067200,
				VMState: 1, Parent: "p", Type: "snapshot", Ram: 256, Disk: 10,
			},
			want: `{"name":"snap1","description":"d","snaptime":1704067200,"vmstate":1,"parent":"p","type":"snapshot","ram":256,"disk":10}`,
		},
		{
			name: "snapshot-create-required",
			in:   SnapshotCreateRequest{Snapname: "s"},
			want: `{"snapname":"s"}`,
		},
		{
			name: "snapshot-create-full",
			in:   SnapshotCreateRequest{Snapname: "s", VMState: true, Description: "d"},
			want: `{"snapname":"s","vmstate":true,"description":"d"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(b) != tt.want {
				t.Fatalf("mismatch\n got: %s\nwant: %s", b, tt.want)
			}
		})
	}
}

// TestCloneVMRequestRoundTrip verifies CloneVMRequest JSON round-trips and that
// only NewID (required) survives when the optionals are zero.
func TestCloneVMRequestRoundTrip(t *testing.T) {
	in := CloneVMRequest{
		NewID: 101, Name: "web-clone", Full: true, Storage: "local",
		Pool: "pool1", Description: "d", Format: "qcow2", Snapname: "snap1",
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got CloneVMRequest
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("round-trip mismatch\n got: %#v\nwant: %#v", got, in)
	}

	min, err := json.Marshal(CloneVMRequest{NewID: 101})
	if err != nil {
		t.Fatalf("marshal minimal: %v", err)
	}
	if string(min) != `{"newid":101}` {
		t.Fatalf("minimal mismatch: %s", min)
	}
}
