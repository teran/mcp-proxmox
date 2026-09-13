package application

import (
	"context"

	"github.com/teran/mcp-proxmox/domain/model"
	"github.com/teran/mcp-proxmox/domain/port"
)

// mockPVEGateway is a controllable port.PVEGateway mock. Each method calls a
// field function if set, otherwise returns a zero/empty result.
type mockPVEGateway struct {
	listNodes        func(ctx context.Context) ([]model.Node, error)
	getNodeStatus    func(ctx context.Context, node string) (*model.NodeStatus, error)
	getClusterStatus func(ctx context.Context) ([]model.ClusterStatus, error)
	getClusterRes    func(ctx context.Context) ([]model.ClusterResource, error)
	getNextID        func(ctx context.Context) (string, error)
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
	createVM         func(ctx context.Context, node string, req model.CreateVMRequest) (int, error)
	resizeVM         func(ctx context.Context, node string, vmid int, req model.ResizeVMRequest) error
	migrateVM        func(ctx context.Context, node string, vmid int, req model.MigrateVMRequest) error
	addHAResource    func(ctx context.Context, req model.HAResourceRequest) error
	startVMBackup    func(ctx context.Context, node string, req model.VMBackupRequest) (*model.Task, error)
}

func (m *mockPVEGateway) ListNodes(ctx context.Context) ([]model.Node, error) {
	if m.listNodes != nil {
		return m.listNodes(ctx)
	}
	return nil, nil
}
func (m *mockPVEGateway) GetNodeStatus(ctx context.Context, node string) (*model.NodeStatus, error) {
	if m.getNodeStatus != nil {
		return m.getNodeStatus(ctx, node)
	}
	return nil, nil
}
func (m *mockPVEGateway) GetClusterStatus(ctx context.Context) ([]model.ClusterStatus, error) {
	if m.getClusterStatus != nil {
		return m.getClusterStatus(ctx)
	}
	return nil, nil
}
func (m *mockPVEGateway) GetClusterResources(ctx context.Context) ([]model.ClusterResource, error) {
	if m.getClusterRes != nil {
		return m.getClusterRes(ctx)
	}
	return nil, nil
}
func (m *mockPVEGateway) GetNextID(ctx context.Context) (string, error) {
	if m.getNextID != nil {
		return m.getNextID(ctx)
	}
	return "", nil
}
func (m *mockPVEGateway) ListVMs(ctx context.Context, node string) ([]model.VM, error) {
	if m.listVMs != nil {
		return m.listVMs(ctx, node)
	}
	return nil, nil
}
func (m *mockPVEGateway) GetVMConfig(ctx context.Context, node string, vmid int) (*model.VMConfig, error) {
	if m.getVMConfig != nil {
		return m.getVMConfig(ctx, node, vmid)
	}
	return nil, nil
}
func (m *mockPVEGateway) GetVMStatus(ctx context.Context, node string, vmid int) (*model.VMStatus, error) {
	if m.getVMStatus != nil {
		return m.getVMStatus(ctx, node, vmid)
	}
	return nil, nil
}
func (m *mockPVEGateway) ListLXCs(ctx context.Context, node string) ([]model.LXC, error) {
	if m.listLXCs != nil {
		return m.listLXCs(ctx, node)
	}
	return nil, nil
}
func (m *mockPVEGateway) GetLXCConfig(ctx context.Context, node string, vmid int) (*model.LXCConfig, error) {
	if m.getLXCConfig != nil {
		return m.getLXCConfig(ctx, node, vmid)
	}
	return nil, nil
}
func (m *mockPVEGateway) GetLXCStatus(ctx context.Context, node string, vmid int) (*model.LXCStatus, error) {
	if m.getLXCStatus != nil {
		return m.getLXCStatus(ctx, node, vmid)
	}
	return nil, nil
}
func (m *mockPVEGateway) ListStorage(ctx context.Context) ([]model.Storage, error) {
	if m.listStorage != nil {
		return m.listStorage(ctx)
	}
	return nil, nil
}
func (m *mockPVEGateway) GetStorageStatus(ctx context.Context, node, storage string) (*model.StorageStatus, error) {
	if m.getStorageStatus != nil {
		return m.getStorageStatus(ctx, node, storage)
	}
	return nil, nil
}
func (m *mockPVEGateway) ListNetwork(ctx context.Context, node string) ([]model.NetworkInterface, error) {
	if m.listNetwork != nil {
		return m.listNetwork(ctx, node)
	}
	return nil, nil
}
func (m *mockPVEGateway) ListTasks(ctx context.Context, node string, opts port.TaskListOptions) ([]model.Task, error) {
	if m.listTasks != nil {
		return m.listTasks(ctx, node, opts)
	}
	return nil, nil
}
func (m *mockPVEGateway) GetTaskStatus(ctx context.Context, node, upid string) (*model.TaskStatus, error) {
	if m.getTaskStatus != nil {
		return m.getTaskStatus(ctx, node, upid)
	}
	return nil, nil
}
func (m *mockPVEGateway) GetTaskLog(ctx context.Context, node, upid string, limit int) ([]model.TaskLogEntry, error) {
	if m.getTaskLog != nil {
		return m.getTaskLog(ctx, node, upid, limit)
	}
	return nil, nil
}
func (m *mockPVEGateway) GetPVEVersion(ctx context.Context) (*model.PVEVersion, error) {
	if m.getPVEVersion != nil {
		return m.getPVEVersion(ctx)
	}
	return nil, nil
}
func (m *mockPVEGateway) CreateVM(ctx context.Context, node string, req model.CreateVMRequest) (int, error) {
	if m.createVM != nil {
		return m.createVM(ctx, node, req)
	}
	return 0, nil
}
func (m *mockPVEGateway) ResizeVM(ctx context.Context, node string, vmid int, req model.ResizeVMRequest) error {
	if m.resizeVM != nil {
		return m.resizeVM(ctx, node, vmid, req)
	}
	return nil
}
func (m *mockPVEGateway) MigrateVM(ctx context.Context, node string, vmid int, req model.MigrateVMRequest) error {
	if m.migrateVM != nil {
		return m.migrateVM(ctx, node, vmid, req)
	}
	return nil
}
func (m *mockPVEGateway) AddHAResource(ctx context.Context, req model.HAResourceRequest) error {
	if m.addHAResource != nil {
		return m.addHAResource(ctx, req)
	}
	return nil
}
func (m *mockPVEGateway) StartVMBackup(ctx context.Context, node string, req model.VMBackupRequest) (*model.Task, error) {
	if m.startVMBackup != nil {
		return m.startVMBackup(ctx, node, req)
	}
	return nil, nil
}

var _ port.PVEGateway = (*mockPVEGateway)(nil)

// mockPBSGateway is a controllable port.PBSGateway mock.
type mockPBSGateway struct {
	listDatastores    func(ctx context.Context) ([]model.Datastore, error)
	getDatastoreStats func(ctx context.Context, store string) (*model.DatastoreStatus, error)
	listBackups       func(ctx context.Context, store string) ([]model.Backup, error)
	getBackup         func(ctx context.Context, store, backupID string) ([]model.Backup, error)
	getBackupNotes    func(ctx context.Context, store, backupID, backupType string) (*model.BackupNotes, error)
	getVerifyStatus   func(ctx context.Context, store, upid string) (*model.VerifyStatus, error)
	getPruneStatus    func(ctx context.Context, store, upid string) (*model.PruneStatus, error)
	getPBSVersion     func(ctx context.Context) (*model.PBSVersion, error)
	startVerify       func(ctx context.Context, store string) (string, error)
	startGC           func(ctx context.Context, store string) (string, error)
	startPrune        func(ctx context.Context, store string) (string, error)
	startSync         func(ctx context.Context, store string, req model.PBSSyncRequest) (string, error)
}

func (m *mockPBSGateway) ListDatastores(ctx context.Context) ([]model.Datastore, error) {
	if m.listDatastores != nil {
		return m.listDatastores(ctx)
	}
	return nil, nil
}
func (m *mockPBSGateway) GetDatastoreStatus(ctx context.Context, store string) (*model.DatastoreStatus, error) {
	if m.getDatastoreStats != nil {
		return m.getDatastoreStats(ctx, store)
	}
	return nil, nil
}
func (m *mockPBSGateway) ListBackups(ctx context.Context, store string) ([]model.Backup, error) {
	if m.listBackups != nil {
		return m.listBackups(ctx, store)
	}
	return nil, nil
}
func (m *mockPBSGateway) GetBackup(ctx context.Context, store, backupID string) ([]model.Backup, error) {
	if m.getBackup != nil {
		return m.getBackup(ctx, store, backupID)
	}
	return nil, nil
}
func (m *mockPBSGateway) GetBackupNotes(ctx context.Context, store, backupID, backupType string) (*model.BackupNotes, error) {
	if m.getBackupNotes != nil {
		return m.getBackupNotes(ctx, store, backupID, backupType)
	}
	return nil, nil
}
func (m *mockPBSGateway) GetVerifyStatus(ctx context.Context, store, upid string) (*model.VerifyStatus, error) {
	if m.getVerifyStatus != nil {
		return m.getVerifyStatus(ctx, store, upid)
	}
	return nil, nil
}
func (m *mockPBSGateway) GetPruneStatus(ctx context.Context, store, upid string) (*model.PruneStatus, error) {
	if m.getPruneStatus != nil {
		return m.getPruneStatus(ctx, store, upid)
	}
	return nil, nil
}
func (m *mockPBSGateway) GetPBSVersion(ctx context.Context) (*model.PBSVersion, error) {
	if m.getPBSVersion != nil {
		return m.getPBSVersion(ctx)
	}
	return nil, nil
}
func (m *mockPBSGateway) StartVerify(ctx context.Context, store string) (string, error) {
	if m.startVerify != nil {
		return m.startVerify(ctx, store)
	}
	return "", nil
}
func (m *mockPBSGateway) StartGC(ctx context.Context, store string) (string, error) {
	if m.startGC != nil {
		return m.startGC(ctx, store)
	}
	return "", nil
}
func (m *mockPBSGateway) StartPrune(ctx context.Context, store string) (string, error) {
	if m.startPrune != nil {
		return m.startPrune(ctx, store)
	}
	return "", nil
}
func (m *mockPBSGateway) StartSync(ctx context.Context, store string, req model.PBSSyncRequest) (string, error) {
	if m.startSync != nil {
		return m.startSync(ctx, store, req)
	}
	return "", nil
}

var _ port.PBSGateway = (*mockPBSGateway)(nil)
