package model

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestPBSRestoreRoundTrip verifies the PBS restore request and task types
// marshal/unmarshal without loss.
func TestPBSRestoreRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		in   any
	}{
		{
			name: "file-restore",
			in:   PBSFileRestoreRequest{Path: "/etc/passwd", Target: "/restore/passwd"},
		},
		{
			name: "vm-restore",
			in: PBSVMRestoreRequest{
				Target: "local", VMID: "next", Host: "pve1", Password: "secret",
				Fingerprint: "AA:BB", Pool: "pool1", Verbose: true, Reload: true,
			},
		},
		{
			name: "pbs-task",
			in:   PBSTask{UPID: "upid1", Type: "restore", Status: "running", StartTime: 1, EndTime: 2, Worker: "w", Errors: 1},
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

// TestPBSRestoreExactJSON pins the required-field-only wire format.
func TestPBSRestoreExactJSON(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{"file-restore-required", PBSFileRestoreRequest{Path: "/a", Target: "/b"}, `{"path":"/a","target":"/b"}`},
		{"vm-restore-required", PBSVMRestoreRequest{Target: "local"}, `{"target":"local"}`},
		{"pbs-task-required", PBSTask{UPID: "u"}, `{"upid":"u"}`},
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
