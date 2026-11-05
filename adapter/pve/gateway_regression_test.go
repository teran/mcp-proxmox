package pve

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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

// TestGateway_AddHAResource_NodesCommaJoined pins Defect 4's comma-joining
// guarantee under the Regression 2 contract: preferred nodes must still be sent as a
// single comma-joined STRING (not a JSON array), but — per Regression 2 — on the
// follow-up PUT /cluster/ha/resources/{sid} (the create POST must omit nodes).
func TestGateway_AddHAResource_NodesCommaJoined(t *testing.T) {
	srv, seq := mockServerSeq(t, 200, `{}`)
	g, _ := newTestGateway(t, srv)

	err := g.AddHAResource(context.Background(), model.HAResourceRequest{
		SID: "vm:100", Type: "vm", Nodes: []string{"nuc01", "ms01"}, Comment: "primary",
	})
	require.NoError(t, err)

	require.Len(t, seq.recs, 2, "create + update must both be issued when nodes are provided")
	create := seq.recs[0]
	assert.Equal(t, http.MethodPost, create.method)
	assert.Equal(t, "/api2/json/cluster/ha/resources", create.path)
	_, hasNodes := create.form["nodes"]
	assert.False(t, hasNodes, "nodes must not be on the create (PVE rejects it)")

	update := seq.recs[1]
	assert.Equal(t, http.MethodPut, update.method)
	assert.Equal(t, "/api2/json/cluster/ha/resources/vm:100", update.path)
	assert.Equal(t, "nuc01,ms01", update.form.Get("nodes"),
		"nodes must be a comma-joined string on the update, not a JSON array")
	assert.Equal(t, "PVEAPIToken=user@pam!tokenid=secret", update.auth)
}

// ---------------------------------------------------------------------------
// Regression 1 (regression from e930617) — pve_vm_create with an explicit vmid.
// Real PVE 9.2 returns a task UPID STRING ({"data":"UPID:...:qmcreate:141:..."})
// when a vmid is supplied explicitly, NOT a numeric vmid. The earlier fix made
// CreateVM decode the WHOLE data field as a number via FlexInt/strconv.Atoi, so
// a UPID response now fails with:
//   strconv.Atoi: parsing "UPID:...qmcreate:141:..." invalid syntax
// CreateVM must NOT parse the whole data as an int. Contract: (a) a
// numeric-string data returns that vmid; (b) a UPID data returns the vmid
// extracted from its ":<vmid>:" segment. Both must return the numeric vmid
// without error so the tool keeps surfacing a numeric vmid.
// ---------------------------------------------------------------------------

// TestGateway_CreateVM_UPIDResponse is the RED reproducer for Regression 1: PVE
// returns a task UPID when the VM is created with an explicit vmid. CreateVM
// must not error and must return the vmid (141) extracted from the UPID. Today
// FlexInt.UnmarshalJSON fails strconv.Atoi on the whole UPID, so CreateVM
// errors.
func TestGateway_CreateVM_UPIDResponse(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":"UPID:nuc01:00001111:00000002:66ABCDEF:qmcreate:141:root@pam:"}`, rec)
	g, _ := newTestGateway(t, srv)

	vmid, err := g.CreateVM(context.Background(), "pve1", model.CreateVMRequest{VMID: 141, Name: "web"})
	require.NoError(t, err, "CreateVM must not error on a UPID create response")
	assert.Equal(t, 141, vmid, "vmid must be extracted from the UPID's :<vmid>: segment")

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api2/json/nodes/pve1/qemu", rec.path)
	assert.Equal(t, "PVEAPIToken=user@pam!tokenid=secret", rec.auth)
}

// TestGateway_CreateVM_NumericStringVMID guards that the Regression 1 fix does not
// break the numeric-string response ({"data":"141"}) PVE returns when no vmid
// is supplied — CreateVM must still return 141 (regression guard on the
// existing TestGateway_CreateVM_StringVMID path).
func TestGateway_CreateVM_NumericStringVMID(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":"141"}`, rec)
	g, _ := newTestGateway(t, srv)

	vmid, err := g.CreateVM(context.Background(), "pve1", model.CreateVMRequest{Name: "web"})
	require.NoError(t, err, "CreateVM must decode a numeric-string VMID")
	assert.Equal(t, 141, vmid)
}

// ---------------------------------------------------------------------------
// Regression 2 — pve_ha_add with nodes. Real PVE POST /cluster/ha/resources REJECTS
// `nodes` in the create body:
//   nodes: property is not defined in schema and the schema does not allow
//   additional properties
// PVE only accepts `nodes` on a SEPARATE update call (PUT
// /cluster/ha/resources/{sid}). Contract: AddHAResource must POST the create
// WITHOUT nodes (sid+type+comment only); when preferred nodes are supplied it
// must issue a follow-up PUT /cluster/ha/resources/{sid} setting nodes
// comma-joined. Without nodes, exactly one create POST (no follow-up).
// ---------------------------------------------------------------------------

// seqCapture records every request a multi-call mock server receives, in order.
type seqCapture struct {
	recs []reqCapture
}

// mockServerSeq spins up an httptest server that records every request in order
// and always responds with the given status and body. Used to assert the
// create-then-update sequence of AddHAResource.
func mockServerSeq(t *testing.T, status int, body string) (*httptest.Server, *seqCapture) {
	t.Helper()
	seq := &seqCapture{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rec reqCapture
		rec.method = r.Method
		rec.path = r.URL.Path
		rec.auth = r.Header.Get("Authorization")
		_ = r.ParseForm()
		rec.form = r.PostForm
		if b, err := io.ReadAll(r.Body); err == nil {
			rec.rawBody = string(b)
		}
		seq.recs = append(seq.recs, rec)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	return srv, seq
}

// TestGateway_AddHAResource_CreateOmitsNodes is the RED reproducer for Regression 2's
// first half: the outbound create POST must NOT carry a `nodes` field (PVE
// rejects it). Today AddHAResource sends nodes on the create.
func TestGateway_AddHAResource_CreateOmitsNodes(t *testing.T) {
	srv, seq := mockServerSeq(t, 200, `{}`)
	g, _ := newTestGateway(t, srv)

	err := g.AddHAResource(context.Background(), model.HAResourceRequest{
		SID: "vm:100", Type: "vm", Nodes: []string{"nuc01", "ms01"}, Comment: "primary",
	})
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(seq.recs), 1, "a create request must be made")
	create := seq.recs[0]
	assert.Equal(t, http.MethodPost, create.method)
	assert.Equal(t, "/api2/json/cluster/ha/resources", create.path)
	_, hasNodes := create.form["nodes"]
	assert.False(t, hasNodes, "create body must NOT include nodes (PVE rejects it)")
	assert.Equal(t, "vm:100", create.form.Get("sid"))
	assert.Equal(t, "vm", create.form.Get("type"))
	assert.Equal(t, "primary", create.form.Get("comment"))
}

// TestGateway_AddHAResource_NodesViaFollowUpUpdate is the RED reproducer for
// Regression 2's second half: preferred nodes must be applied via a follow-up
// PUT /cluster/ha/resources/{sid} with the nodes comma-joined, NOT on the
// create. Today only a single POST (with nodes) is sent and no update occurs.
func TestGateway_AddHAResource_NodesViaFollowUpUpdate(t *testing.T) {
	srv, seq := mockServerSeq(t, 200, `{}`)
	g, _ := newTestGateway(t, srv)

	err := g.AddHAResource(context.Background(), model.HAResourceRequest{
		SID: "vm:100", Type: "vm", Nodes: []string{"nuc01", "ms01"}, Comment: "primary",
	})
	require.NoError(t, err)

	require.Len(t, seq.recs, 2, "create + update must both be issued when nodes are provided")

	create := seq.recs[0]
	assert.Equal(t, http.MethodPost, create.method)
	assert.Equal(t, "/api2/json/cluster/ha/resources", create.path)
	_, hasNodes := create.form["nodes"]
	assert.False(t, hasNodes, "nodes must NOT be on the create")

	update := seq.recs[1]
	assert.Equal(t, http.MethodPut, update.method)
	assert.Equal(t, "/api2/json/cluster/ha/resources/vm:100", update.path)
	assert.Equal(t, "nuc01,ms01", update.form.Get("nodes"), "update must set nodes comma-joined")
	assert.Equal(t, "PVEAPIToken=user@pam!tokenid=secret", update.auth)
}

// TestGateway_AddHAResource_NoNodesSinglePost guards Regression 2's no-nodes path:
// without nodes only a single create POST is made (no follow-up update), and
// its body carries sid/type/comment but no nodes.
func TestGateway_AddHAResource_NoNodesSinglePost(t *testing.T) {
	srv, seq := mockServerSeq(t, 200, `{}`)
	g, _ := newTestGateway(t, srv)

	err := g.AddHAResource(context.Background(), model.HAResourceRequest{
		SID: "vm:200", Type: "ct", Comment: "backup",
	})
	require.NoError(t, err)

	require.Len(t, seq.recs, 1, "no follow-up update when nodes are absent")
	create := seq.recs[0]
	assert.Equal(t, http.MethodPost, create.method)
	assert.Equal(t, "/api2/json/cluster/ha/resources", create.path)
	_, hasNodes := create.form["nodes"]
	assert.False(t, hasNodes)
	assert.Equal(t, "vm:200", create.form.Get("sid"))
	assert.Equal(t, "ct", create.form.Get("type"))
	assert.Equal(t, "backup", create.form.Get("comment"))
}
