package model

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestPBSFileRoundTrip verifies PBSFile JSON round-trips and that only Filename
// (required) survives when the optionals are zero.
func TestPBSFileRoundTrip(t *testing.T) {
	in := PBSFile{Filename: "client.logidx", Type: "log", Size: 1024}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got PBSFile
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("round-trip mismatch\n got: %#v\nwant: %#v", got, in)
	}

	min, err := json.Marshal(PBSFile{Filename: "f"})
	if err != nil {
		t.Fatalf("marshal minimal: %v", err)
	}
	if string(min) != `{"filename":"f"}` {
		t.Fatalf("minimal mismatch: %s", min)
	}
}
