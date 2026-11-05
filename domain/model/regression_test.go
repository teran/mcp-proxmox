package model

import (
	"encoding/json"
	"testing"
)

// These regression tests reproduce decode defects found during a live integration
// run against real Proxmox VE. They pin the model struct tags / JSON decode
// behavior at the domain layer. Each is RED against the current production code
// and must go green once @developer applies the matching fix.

// TestNetworkInterface_StringVLANID is the RED reproducer for the shared
// Defect 1 / Defect 2 root cause: real PVE returns the `vlan-id` field of a network
// (and storage) object as a STRING (e.g. "100"), but model.NetworkInterface
// declares VLANID as int. Unmarshalling such a payload currently fails with:
//
//	json: cannot unmarshal string into Go struct field .0.vlan-id of type int
//
// Decoding must succeed and surface the numeric VLAN ID.
func TestNetworkInterface_StringVLANID(t *testing.T) {
	payload := `[{"iface":"vmbr0.100","type":"vlan","vlan-id":"100","active":1}]`

	var ifaces []NetworkInterface
	if err := json.Unmarshal([]byte(payload), &ifaces); err != nil {
		t.Fatalf("unmarshal string vlan-id must succeed, got: %v", err)
	}
	if len(ifaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(ifaces))
	}
	if ifaces[0].VLANID != 100 {
		t.Fatalf("expected numeric VLANID 100, got %d", ifaces[0].VLANID)
	}
}
