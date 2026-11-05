package pve

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/model"
)

// These regression tests reproduce 5 decode/serialization defects found during a
// live integration run against real Proxmox VE 9.2 / PBS 4.2. They follow the
// hermetic convention in this package: httptest mocks of the REAL PVE JSON API
// (/api2/json) with method/path/auth assertions. Each test is RED against the
// current (defective) production code and must go green once @developer applies the
// matching fix.

// ---------------------------------------------------------------------------
// Defect 1 + Defect 2 — vlan-id returned as a STRING by PVE, but model.NetworkInterface
// declares VLANID as int, so decode fails:
//
//	json: cannot unmarshal string into Go struct field .0.vlan-id of type int
//
// Both pve_storage_list and pve_network_list share this root cause: the live
// payloads carried `"vlan-id":"100"` (a string). The struct with the int-typed
// vlan-id field is model.NetworkInterface (VLANID int `json:"vlan-id"`). The
// fix must make decoding a string vlan-id succeed (e.g. a flexible int type).
// ---------------------------------------------------------------------------

// TestGateway_ListNetwork_VLANIDString is the RED reproducer for Defect 2
// (pve_network_list). A real PVE node returns a VLAN device with `vlan-id` as
// a string; the gateway must parse it without error and surface the numeric
// VLAN ID. Today the int-typed field makes json.Unmarshal fail.
func TestGateway_ListNetwork_VLANIDString(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[{"iface":"vmbr0.100","type":"vlan","vlan-id":"100","active":1}]}`, rec)
	g, _ := newTestGateway(t, srv)

	ns, err := g.ListNetwork(context.Background(), "pve1")
	require.NoError(t, err, "ListNetwork must tolerate a string vlan-id")
	require.Len(t, ns, 1)
	assert.Equal(t, 100, ns[0].VLANID, "numeric vlan-id must be surfaced")

	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/api2/json/nodes/pve1/network", rec.path)
	assert.Equal(t, "PVEAPIToken=user@pam!tokenid=secret", rec.auth)
}

// TestGateway_ListStorage_StringVLANID guards Defect 1 (pve_storage_list). The
// live PVE storage payload carried a `vlan-id` string; decoding must succeed.
// NOTE: model.Storage currently has no vlan-id field, so unknown fields are
// ignored and this test is already green today. It is included to (a) pin the
// storage-list path against the reported live failure and (b) fail red if the
// fix introduces an int-typed vlan-id on Storage without a tolerant type. The
// authoritative RED reproducer of the string-vlan-id decode defect is
// TestGateway_ListNetwork_VLANIDString / the model test (shared root cause).
func TestGateway_ListStorage_StringVLANID(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":[{"storage":"local","type":"dir","active":1,"vlan-id":"100"}]}`, rec)
	g, _ := newTestGateway(t, srv)

	ss, err := g.ListStorage(context.Background())
	require.NoError(t, err, "ListStorage must tolerate a string vlan-id")
	require.Len(t, ss, 1)
	assert.Equal(t, "local", ss[0].Storage)

	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/api2/json/storage", rec.path)
	assert.Equal(t, "PVEAPIToken=user@pam!tokenid=secret", rec.auth)
}

// ---------------------------------------------------------------------------
// Defect 3 — pve_vm_create response VMID and pve_vm_get vmid.
//   (a) CreateVM decodes `data` into an int; a real PVE create returns the new
//       VMID as a STRING, so decode fails:
//           json: cannot unmarshal string into Go value of type int
//   (b) GetVMConfig leaves VMID at 0 because a real PVE /qemu/{vmid}/config
//       response does not echo vmid; the gateway must populate it from the
//       request path (mirroring GetStorageStatus).
// ---------------------------------------------------------------------------

// TestGateway_CreateVM_StringVMID is the RED reproducer for Defect 3(a): the
// POST /nodes/{node}/qemu response carries `data` as a string VMID and must
// decode to the numeric VMID without error.
func TestGateway_CreateVM_StringVMID(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":"100"}`, rec)
	g, _ := newTestGateway(t, srv)

	vmid, err := g.CreateVM(context.Background(), "pve1", model.CreateVMRequest{Name: "web"})
	require.NoError(t, err, "CreateVM must decode a string VMID")
	assert.Equal(t, 100, vmid)

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api2/json/nodes/pve1/qemu", rec.path)
	assert.Equal(t, "PVEAPIToken=user@pam!tokenid=secret", rec.auth)
}

// TestGateway_GetVMConfig_VMIDFromPath is the RED reproducer for Defect 3(b):
// a real PVE config response omits `vmid`, so the gateway must set VMID from
// the request path argument (like GetStorageStatus sets Storage). Today the
// returned config carries VMID 0.
func TestGateway_GetVMConfig_VMIDFromPath(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":{"name":"web","cores":2,"memory":1024}}`, rec)
	g, _ := newTestGateway(t, srv)

	cfg, err := g.GetVMConfig(context.Background(), "pve1", 100)
	require.NoError(t, err)
	assert.Equal(t, 100, cfg.VMID, "vmid must be populated from the request path, not left at 0")
	assert.Equal(t, "web", cfg.Name)

	assert.Equal(t, http.MethodGet, rec.method)
	assert.Equal(t, "/api2/json/nodes/pve1/qemu/100/config", rec.path)
	assert.Equal(t, "PVEAPIToken=user@pam!tokenid=secret", rec.auth)
}

// ---------------------------------------------------------------------------
// Defect 5 — pve_backup_job_create returns "pve: empty data (op=data)" even though
// the job IS created. PVE's create-vzdump-job returns empty / `{}` data on
// success; CreateBackupJob decodes `data` into a *string and the gateway's
// decode() maps empty/null data for a non-slice target to EmptyDataError.
// Creating a backup job that returns empty data must NOT error.
// ---------------------------------------------------------------------------

// TestGateway_CreateBackupJob_EmptyDataSuccess is the RED reproducer for
// Defect 5: PVE returns `{}` (no data) on success; CreateBackupJob must succeed
// (returning an empty ID) rather than fail with EmptyDataError.
func TestGateway_CreateBackupJob_EmptyDataSuccess(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{}`, rec)
	g, _ := newTestGateway(t, srv)

	id, err := g.CreateBackupJob(context.Background(), model.CreateBackupJobRequest{
		Storage: "local", Schedule: "00 2 * * 1",
	})
	require.NoError(t, err, "creating a backup job that returns empty data must not error")
	assert.Equal(t, "", id)

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api2/json/cluster/backup", rec.path)
	assert.Equal(t, "PVEAPIToken=user@pam!tokenid=secret", rec.auth)
	assert.Equal(t, "local", rec.form.Get("storage"))
	assert.Equal(t, "00 2 * * 1", rec.form.Get("schedule"))
}

// TestGateway_CreateBackupJob_EmptyObjectDataSuccess covers the `{"data":{}}`
// variant PVE may return on success: it must also not error.
func TestGateway_CreateBackupJob_EmptyObjectDataSuccess(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":{}}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.CreateBackupJob(context.Background(), model.CreateBackupJobRequest{
		Storage: "local", Schedule: "00 2 * * 1",
	})
	require.NoError(t, err, "creating a backup job that returns {} data must not error")
}

// ---------------------------------------------------------------------------
// Defect 4 — pve_ha_add nodes. PVE HA /cluster/ha/resources expects `nodes` as a
// comma-separated STRING (e.g. "nuc01,ms01"), not a JSON array. This guard
// asserts the outbound request serializes the node list as a comma-joined
// string for the exact live scenario (["nuc01","ms01"]).
// ---------------------------------------------------------------------------

// TestGateway_AddHAResource_NodesCommaJoined pins Defect 4's gateway behavior: the
// outbound form body must carry `nodes` as a single comma-joined string.
func TestGateway_AddHAResource_NodesCommaJoined(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{}`, rec)
	g, _ := newTestGateway(t, srv)

	err := g.AddHAResource(context.Background(), model.HAResourceRequest{
		SID: "vm:100", Type: "vm", Nodes: []string{"nuc01", "ms01"}, Comment: "primary",
	})
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api2/json/cluster/ha/resources", rec.path)
	assert.Equal(t, "PVEAPIToken=user@pam!tokenid=secret", rec.auth)
	assert.Equal(t, "nuc01,ms01", rec.form.Get("nodes"),
		"nodes must be a comma-joined string, not a JSON array")
	assert.Equal(t, "vm:100", rec.form.Get("sid"))
	assert.Equal(t, "vm", rec.form.Get("type"))
}
