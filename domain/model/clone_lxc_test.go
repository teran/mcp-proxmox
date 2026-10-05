package model

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestCloneLXCRequestRoundTrip verifies CloneLXCRequest JSON round-trips and that
// only NewID (required) survives when the optionals are zero.
func TestCloneLXCRequestRoundTrip(t *testing.T) {
	in := CloneLXCRequest{
		NewID: 201, Full: true, Storage: "local", Hostname: "ct-clone",
		Description: "d", Pool: "pool1", Snapname: "snap1",
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got CloneLXCRequest
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("round-trip mismatch\n got: %#v\nwant: %#v", got, in)
	}

	min, err := json.Marshal(CloneLXCRequest{NewID: 201})
	if err != nil {
		t.Fatalf("marshal minimal: %v", err)
	}
	if string(min) != `{"newid":201}` {
		t.Fatalf("minimal mismatch: %s", min)
	}
}
