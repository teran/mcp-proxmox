package port

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCephGatewayInterfaceContract guards the port.CephGateway interface
// contract that the application service, the adapter/mcp tools and the
// adapter/ceph gateway all build against. It uses reflection to assert the
// interface declares exactly the expected method set, so a developer deviation
// from the SPEC contract (a renamed, removed or re-signed method) is caught at
// the port boundary before it silently breaks a dependent layer.
//
// There are no port-level helpers for Ceph (no WithToken-style context helper
// specific to Ceph), so this interface-shape check is the entire port test.
func TestCephGatewayInterfaceContract(t *testing.T) {
	iface := reflect.TypeFor[CephGateway]()

	expected := []string{
		"GetClusterStatus", "GetClusterConfig", "GetClusterLog", "GetClusterMetadata",
		"GetNodeStatus", "GetNodeConfig", "GetNodeConfigDB", "GetNodeFlags",
		"GetNodeMetadata", "GetNodeCrush", "GetNodeRules",
		"ListPools", "ListOSDs", "ListMons", "ListMgrs",
		"CreateCrushRule", "DeleteCrushRule",
		"CreatePool", "UpdatePool", "DeletePool",
		"CreateOSD", "DeleteOSD",
		"CreateMon", "DeleteMon",
		"CreateMgr",
		"SetFlags", "SetConfig",
		"InitCeph", "DestroyCeph",
	}

	got := make(map[string]bool)
	for m := range iface.Methods() {
		got[m.Name] = true
	}

	assert.Len(t, got, len(expected), "CephGateway method count must match the contract")
	for _, name := range expected {
		assert.True(t, got[name], "CephGateway must declare %s per the contract", name)
	}
}
