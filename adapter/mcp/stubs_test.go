package mcp

import (
	"context"
	"testing"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/model"
	"github.com/teran/mcp-proxmox/domain/port"
)

// stubPVEGateway is a controllable port.PVEGateway stub for adapter/mcp tests.
// Every method defaults to a canned success value; a test can override any
// method to force an error.
type stubPVEGateway struct {
	// err is returned by every method when set (forces error propagation).
	err error

	listNodes        func(ctx context.Context) ([]model.Node, error)
	getNodeStatus    func(ctx context.Context, node string) (*model.NodeStatus, error)
	getClusterStatus func(ctx context.Context) ([]model.ClusterStatus, error)
	getClusterRes    func(ctx context.Context) ([]model.ClusterResource, error)
	getNextID        func(ctx context.Context) (int, error)
	listVMs          func(ctx context.Context, node string) ([]model.VM, error)
	getVMConfig      func(ctx context.Context, node string, vmid int) (*model.VMConfig, error)
	getVMStatus      func(ctx context.Context, node string, vmid int) (*model.VMStatus, error)
	listLXCs         func(ctx context.Context, node string) ([]model.LXC, error)
	getLXCConfig     func(ctx context.Context, node string, vmid int) (*model.LXCConfig, error)
	getLXCStatus     func(ctx context.Context, node string, vmid int) (*model.LXCStatus, error)
	listStorage      func(ctx context.Context) ([]model.Storage, error)
	getStorageStatus func(ctx context.Context, node, storage string) (*model.StorageStatus, error)
	listNetwork      func(ctx context.Context, node string) ([]model.NetworkInterface, error)
	listTasks        func(ctx context.Context, node string, opts port.TaskListOptions) ([]model.Task, error)
	getTaskStatus    func(ctx context.Context, node, upid string) (*model.TaskStatus, error)
	getTaskLog       func(ctx context.Context, node, upid string, limit int) ([]model.TaskLogEntry, error)
	getPVEVersion    func(ctx context.Context) (*model.PVEVersion, error)
}

func (s *stubPVEGateway) fail() bool { return s.err != nil }

func (s *stubPVEGateway) ListNodes(ctx context.Context) ([]model.Node, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.listNodes != nil {
		return s.listNodes(ctx)
	}
	return []model.Node{{Node: "pve1", Status: "online"}}, nil
}
func (s *stubPVEGateway) GetNodeStatus(ctx context.Context, node string) (*model.NodeStatus, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getNodeStatus != nil {
		return s.getNodeStatus(ctx, node)
	}
	return &model.NodeStatus{Node: node, Status: "online"}, nil
}
func (s *stubPVEGateway) GetClusterStatus(ctx context.Context) ([]model.ClusterStatus, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getClusterStatus != nil {
		return s.getClusterStatus(ctx)
	}
	return []model.ClusterStatus{{Name: "cluster", Quorum: true}}, nil
}
func (s *stubPVEGateway) GetClusterResources(ctx context.Context) ([]model.ClusterResource, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getClusterRes != nil {
		return s.getClusterRes(ctx)
	}
	return []model.ClusterResource{{ID: "qemu/100", Type: "qemu"}}, nil
}
func (s *stubPVEGateway) GetNextID(ctx context.Context) (int, error) {
	if s.fail() {
		return 0, s.err
	}
	if s.getNextID != nil {
		return s.getNextID(ctx)
	}
	return 201, nil
}
func (s *stubPVEGateway) ListVMs(ctx context.Context, node string) ([]model.VM, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.listVMs != nil {
		return s.listVMs(ctx, node)
	}
	return []model.VM{{VMID: 100, Name: "web"}}, nil
}
func (s *stubPVEGateway) GetVMConfig(ctx context.Context, node string, vmid int) (*model.VMConfig, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getVMConfig != nil {
		return s.getVMConfig(ctx, node, vmid)
	}
	return &model.VMConfig{VMID: vmid, Name: "web"}, nil
}
func (s *stubPVEGateway) GetVMStatus(ctx context.Context, node string, vmid int) (*model.VMStatus, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getVMStatus != nil {
		return s.getVMStatus(ctx, node, vmid)
	}
	return &model.VMStatus{VMID: vmid, Status: "running"}, nil
}
func (s *stubPVEGateway) ListLXCs(ctx context.Context, node string) ([]model.LXC, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.listLXCs != nil {
		return s.listLXCs(ctx, node)
	}
	return []model.LXC{{VMID: 200, Name: "ct1"}}, nil
}
func (s *stubPVEGateway) GetLXCConfig(ctx context.Context, node string, vmid int) (*model.LXCConfig, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getLXCConfig != nil {
		return s.getLXCConfig(ctx, node, vmid)
	}
	return &model.LXCConfig{VMID: vmid, Hostname: "ct1"}, nil
}
func (s *stubPVEGateway) GetLXCStatus(ctx context.Context, node string, vmid int) (*model.LXCStatus, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getLXCStatus != nil {
		return s.getLXCStatus(ctx, node, vmid)
	}
	return &model.LXCStatus{VMID: vmid, Status: "running"}, nil
}
func (s *stubPVEGateway) ListStorage(ctx context.Context) ([]model.Storage, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.listStorage != nil {
		return s.listStorage(ctx)
	}
	return []model.Storage{{Storage: "local", Type: "dir"}}, nil
}
func (s *stubPVEGateway) GetStorageStatus(ctx context.Context, node, storage string) (*model.StorageStatus, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getStorageStatus != nil {
		return s.getStorageStatus(ctx, node, storage)
	}
	return &model.StorageStatus{Storage: storage, Type: "dir"}, nil
}
func (s *stubPVEGateway) ListNetwork(ctx context.Context, node string) ([]model.NetworkInterface, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.listNetwork != nil {
		return s.listNetwork(ctx, node)
	}
	return []model.NetworkInterface{{Iface: "eth0"}}, nil
}
func (s *stubPVEGateway) ListTasks(ctx context.Context, node string, opts port.TaskListOptions) ([]model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.listTasks != nil {
		return s.listTasks(ctx, node, opts)
	}
	return []model.Task{{UPID: "u", Type: "qmstart"}}, nil
}
func (s *stubPVEGateway) GetTaskStatus(ctx context.Context, node, upid string) (*model.TaskStatus, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getTaskStatus != nil {
		return s.getTaskStatus(ctx, node, upid)
	}
	return &model.TaskStatus{UPID: upid, Status: "stopped"}, nil
}
func (s *stubPVEGateway) GetTaskLog(ctx context.Context, node, upid string, limit int) ([]model.TaskLogEntry, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getTaskLog != nil {
		return s.getTaskLog(ctx, node, upid, limit)
	}
	return []model.TaskLogEntry{{LineNumber: 1, Text: "OK"}}, nil
}
func (s *stubPVEGateway) GetPVEVersion(ctx context.Context) (*model.PVEVersion, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getPVEVersion != nil {
		return s.getPVEVersion(ctx)
	}
	return &model.PVEVersion{Version: "8.2.2"}, nil
}

var _ port.PVEGateway = (*stubPVEGateway)(nil)

// stubPBSGateway is a controllable port.PBSGateway stub for adapter/mcp tests.
type stubPBSGateway struct {
	err error

	listDatastores    func(ctx context.Context) ([]model.Datastore, error)
	getDatastoreStats func(ctx context.Context, store string) (*model.DatastoreStatus, error)
	listBackups       func(ctx context.Context, store string) ([]model.Backup, error)
	getBackup         func(ctx context.Context, store, snapshot string) (*model.Backup, error)
	getBackupNotes    func(ctx context.Context, store, snapshot string) (*model.BackupNotes, error)
	getVerifyStatus   func(ctx context.Context, store, upid string) (*model.VerifyStatus, error)
	getPruneStatus    func(ctx context.Context, store, upid string) (*model.PruneStatus, error)
	getPBSVersion     func(ctx context.Context) (*model.PBSVersion, error)
}

func (s *stubPBSGateway) fail() bool { return s.err != nil }

func (s *stubPBSGateway) ListDatastores(ctx context.Context) ([]model.Datastore, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.listDatastores != nil {
		return s.listDatastores(ctx)
	}
	return []model.Datastore{{Name: "backup"}}, nil
}
func (s *stubPBSGateway) GetDatastoreStatus(ctx context.Context, store string) (*model.DatastoreStatus, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getDatastoreStats != nil {
		return s.getDatastoreStats(ctx, store)
	}
	return &model.DatastoreStatus{Store: store, Total: 1000}, nil
}
func (s *stubPBSGateway) ListBackups(ctx context.Context, store string) ([]model.Backup, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.listBackups != nil {
		return s.listBackups(ctx, store)
	}
	return []model.Backup{{BackupID: "vm/100/..."}}, nil
}
func (s *stubPBSGateway) GetBackup(ctx context.Context, store, snapshot string) (*model.Backup, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getBackup != nil {
		return s.getBackup(ctx, store, snapshot)
	}
	return &model.Backup{BackupID: snapshot}, nil
}
func (s *stubPBSGateway) GetBackupNotes(ctx context.Context, store, snapshot string) (*model.BackupNotes, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getBackupNotes != nil {
		return s.getBackupNotes(ctx, store, snapshot)
	}
	return &model.BackupNotes{Snapshot: snapshot, Notes: "keep"}, nil
}
func (s *stubPBSGateway) GetVerifyStatus(ctx context.Context, store, upid string) (*model.VerifyStatus, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getVerifyStatus != nil {
		return s.getVerifyStatus(ctx, store, upid)
	}
	return &model.VerifyStatus{UPID: upid, Status: "ok"}, nil
}
func (s *stubPBSGateway) GetPruneStatus(ctx context.Context, store, upid string) (*model.PruneStatus, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getPruneStatus != nil {
		return s.getPruneStatus(ctx, store, upid)
	}
	return &model.PruneStatus{UPID: upid, Status: "running"}, nil
}
func (s *stubPBSGateway) GetPBSVersion(ctx context.Context) (*model.PBSVersion, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getPBSVersion != nil {
		return s.getPBSVersion(ctx)
	}
	return &model.PBSVersion{Version: "3.2.3"}, nil
}

var _ port.PBSGateway = (*stubPBSGateway)(nil)

// nopLogger is a no-op port.AppLogger for tests.
type nopLogger struct{}

func (nopLogger) Debugf(string, ...any) {}
func (nopLogger) Infof(string, ...any)  {}
func (nopLogger) Warnf(string, ...any)  {}
func (nopLogger) Errorf(string, ...any) {}

// pveROTools is the read-only PVE tool surface (SPEC.md §6.2–6.6).
var pveROTools = []string{
	"pve_node_list", "pve_node_status", "pve_cluster_status",
	"pve_cluster_resources", "pve_nextid",
	"pve_vm_list", "pve_vm_get", "pve_vm_status",
	"pve_lxc_list", "pve_lxc_get", "pve_lxc_status",
	"pve_storage_list", "pve_storage_get", "pve_network_list",
	"pve_task_list", "pve_task_status", "pve_task_log", "pve_version",
}

// pbsROTools is the read-only PBS tool surface (SPEC.md §6.7–6.9).
var pbsROTools = []string{
	"pbs_datastore_list", "pbs_datastore_status",
	"pbs_backup_list", "pbs_backup_get", "pbs_backup_notes_get",
	"pbs_verify_status", "pbs_prune_status", "pbs_version",
}

// newTestServer builds an mcpSDK.Server wired through the real application.App
// and RegisterTools path, and returns the server plus a connected client
// session for driving tool calls over in-memory transports.
func newTestServer(t *testing.T, app *application.App, enableMutations bool) (*mcpSDK.Server, *mcpSDK.ClientSession) {
	t.Helper()

	s := NewServer(
		&mcpSDK.Implementation{Name: "mcp-proxmox-test", Version: "v0.0.0-test"},
		Deps{
			App:             app,
			Logger:          nopLogger{},
			SessionID:       "test-session",
			EnableMutations: enableMutations,
		},
		nil,
	)

	t1, t2 := mcpSDK.NewInMemoryTransports()
	if _, err := s.Connect(context.Background(), t1, nil); err != nil {
		t.Fatalf("server connect: %v", err)
	}
	client := mcpSDK.NewClient(&mcpSDK.Implementation{Name: "test-client", Version: "v0.0.1"}, nil)
	sess, err := client.Connect(context.Background(), t2, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { _ = sess.Close() })
	return s, sess
}

// toolNames returns the set of registered tool names.
func toolNames(t *testing.T, sess *mcpSDK.ClientSession) map[string]bool {
	t.Helper()
	res, err := sess.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	names := make(map[string]bool, len(res.Tools))
	for _, tl := range res.Tools {
		names[tl.Name] = true
	}
	return names
}
