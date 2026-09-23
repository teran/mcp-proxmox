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

	// VM/LXC snapshots (read-only).
	listVMSnapshots  func(ctx context.Context, node string, vmid int) ([]model.Snapshot, error)
	listLXCSnapshots func(ctx context.Context, node string, vmid int) ([]model.Snapshot, error)

	// VM lifecycle mutations.
	startVM            func(ctx context.Context, node string, vmid int) (*model.Task, error)
	stopVM             func(ctx context.Context, node string, vmid int, skiplock bool) (*model.Task, error)
	shutdownVM         func(ctx context.Context, node string, vmid int, forceStop bool, timeout int) (*model.Task, error)
	rebootVM           func(ctx context.Context, node string, vmid int, timeout int) (*model.Task, error)
	resetVM            func(ctx context.Context, node string, vmid int) (*model.Task, error)
	suspendVM          func(ctx context.Context, node string, vmid int, todisk bool) (*model.Task, error)
	resumeVM           func(ctx context.Context, node string, vmid int) (*model.Task, error)
	deleteVM           func(ctx context.Context, node string, vmid int, purge, destroyUnreferencedDisks bool) (*model.Task, error)
	cloneVM            func(ctx context.Context, node string, vmid int, req model.CloneVMRequest) (*model.Task, error)
	createVMSnapshot   func(ctx context.Context, node string, vmid int, req model.SnapshotCreateRequest) (*model.Task, error)
	deleteVMSnapshot   func(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error)
	rollbackVMSnapshot func(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error)

	// LXC lifecycle mutations.
	startLXC            func(ctx context.Context, node string, vmid int) (*model.Task, error)
	stopLXC             func(ctx context.Context, node string, vmid int, skiplock, forceStop bool) (*model.Task, error)
	shutdownLXC         func(ctx context.Context, node string, vmid int, forceStop bool, timeout int) (*model.Task, error)
	rebootLXC           func(ctx context.Context, node string, vmid int, timeout int) (*model.Task, error)
	deleteLXC           func(ctx context.Context, node string, vmid int, purge, destroyUnreferencedDisks, force bool) (*model.Task, error)
	cloneLXC            func(ctx context.Context, node string, vmid int, req model.CloneLXCRequest) (*model.Task, error)
	createLXCSnapshot   func(ctx context.Context, node string, vmid int, req model.SnapshotCreateRequest) (*model.Task, error)
	deleteLXCSnapshot   func(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error)
	rollbackLXCSnapshot func(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error)

	// Restore.
	restoreVM func(ctx context.Context, node string, req model.PVERestoreRequest) (*model.Task, error)
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
func (m *mockPVEGateway) ListVMSnapshots(ctx context.Context, node string, vmid int) ([]model.Snapshot, error) {
	if m.listVMSnapshots != nil {
		return m.listVMSnapshots(ctx, node, vmid)
	}
	return nil, nil
}
func (m *mockPVEGateway) ListLXCSnapshots(ctx context.Context, node string, vmid int) ([]model.Snapshot, error) {
	if m.listLXCSnapshots != nil {
		return m.listLXCSnapshots(ctx, node, vmid)
	}
	return nil, nil
}
func (m *mockPVEGateway) StartVM(ctx context.Context, node string, vmid int) (*model.Task, error) {
	if m.startVM != nil {
		return m.startVM(ctx, node, vmid)
	}
	return nil, nil
}
func (m *mockPVEGateway) StopVM(ctx context.Context, node string, vmid int, skiplock bool) (*model.Task, error) {
	if m.stopVM != nil {
		return m.stopVM(ctx, node, vmid, skiplock)
	}
	return nil, nil
}
func (m *mockPVEGateway) ShutdownVM(ctx context.Context, node string, vmid int, forceStop bool, timeout int) (*model.Task, error) {
	if m.shutdownVM != nil {
		return m.shutdownVM(ctx, node, vmid, forceStop, timeout)
	}
	return nil, nil
}
func (m *mockPVEGateway) RebootVM(ctx context.Context, node string, vmid int, timeout int) (*model.Task, error) {
	if m.rebootVM != nil {
		return m.rebootVM(ctx, node, vmid, timeout)
	}
	return nil, nil
}
func (m *mockPVEGateway) ResetVM(ctx context.Context, node string, vmid int) (*model.Task, error) {
	if m.resetVM != nil {
		return m.resetVM(ctx, node, vmid)
	}
	return nil, nil
}
func (m *mockPVEGateway) SuspendVM(ctx context.Context, node string, vmid int, todisk bool) (*model.Task, error) {
	if m.suspendVM != nil {
		return m.suspendVM(ctx, node, vmid, todisk)
	}
	return nil, nil
}
func (m *mockPVEGateway) ResumeVM(ctx context.Context, node string, vmid int) (*model.Task, error) {
	if m.resumeVM != nil {
		return m.resumeVM(ctx, node, vmid)
	}
	return nil, nil
}
func (m *mockPVEGateway) DeleteVM(ctx context.Context, node string, vmid int, purge, destroyUnreferencedDisks bool) (*model.Task, error) {
	if m.deleteVM != nil {
		return m.deleteVM(ctx, node, vmid, purge, destroyUnreferencedDisks)
	}
	return nil, nil
}
func (m *mockPVEGateway) CloneVM(ctx context.Context, node string, vmid int, req model.CloneVMRequest) (*model.Task, error) {
	if m.cloneVM != nil {
		return m.cloneVM(ctx, node, vmid, req)
	}
	return nil, nil
}
func (m *mockPVEGateway) CreateVMSnapshot(ctx context.Context, node string, vmid int, req model.SnapshotCreateRequest) (*model.Task, error) {
	if m.createVMSnapshot != nil {
		return m.createVMSnapshot(ctx, node, vmid, req)
	}
	return nil, nil
}
func (m *mockPVEGateway) DeleteVMSnapshot(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
	if m.deleteVMSnapshot != nil {
		return m.deleteVMSnapshot(ctx, node, vmid, snapname)
	}
	return nil, nil
}
func (m *mockPVEGateway) RollbackVMSnapshot(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
	if m.rollbackVMSnapshot != nil {
		return m.rollbackVMSnapshot(ctx, node, vmid, snapname)
	}
	return nil, nil
}
func (m *mockPVEGateway) StartLXC(ctx context.Context, node string, vmid int) (*model.Task, error) {
	if m.startLXC != nil {
		return m.startLXC(ctx, node, vmid)
	}
	return nil, nil
}
func (m *mockPVEGateway) StopLXC(ctx context.Context, node string, vmid int, skiplock, forceStop bool) (*model.Task, error) {
	if m.stopLXC != nil {
		return m.stopLXC(ctx, node, vmid, skiplock, forceStop)
	}
	return nil, nil
}
func (m *mockPVEGateway) ShutdownLXC(ctx context.Context, node string, vmid int, forceStop bool, timeout int) (*model.Task, error) {
	if m.shutdownLXC != nil {
		return m.shutdownLXC(ctx, node, vmid, forceStop, timeout)
	}
	return nil, nil
}
func (m *mockPVEGateway) RebootLXC(ctx context.Context, node string, vmid int, timeout int) (*model.Task, error) {
	if m.rebootLXC != nil {
		return m.rebootLXC(ctx, node, vmid, timeout)
	}
	return nil, nil
}
func (m *mockPVEGateway) DeleteLXC(ctx context.Context, node string, vmid int, purge, destroyUnreferencedDisks, force bool) (*model.Task, error) {
	if m.deleteLXC != nil {
		return m.deleteLXC(ctx, node, vmid, purge, destroyUnreferencedDisks, force)
	}
	return nil, nil
}
func (m *mockPVEGateway) CloneLXC(ctx context.Context, node string, vmid int, req model.CloneLXCRequest) (*model.Task, error) {
	if m.cloneLXC != nil {
		return m.cloneLXC(ctx, node, vmid, req)
	}
	return nil, nil
}
func (m *mockPVEGateway) CreateLXCSnapshot(ctx context.Context, node string, vmid int, req model.SnapshotCreateRequest) (*model.Task, error) {
	if m.createLXCSnapshot != nil {
		return m.createLXCSnapshot(ctx, node, vmid, req)
	}
	return nil, nil
}
func (m *mockPVEGateway) DeleteLXCSnapshot(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
	if m.deleteLXCSnapshot != nil {
		return m.deleteLXCSnapshot(ctx, node, vmid, snapname)
	}
	return nil, nil
}
func (m *mockPVEGateway) RollbackLXCSnapshot(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
	if m.rollbackLXCSnapshot != nil {
		return m.rollbackLXCSnapshot(ctx, node, vmid, snapname)
	}
	return nil, nil
}
func (m *mockPVEGateway) RestoreVM(ctx context.Context, node string, req model.PVERestoreRequest) (*model.Task, error) {
	if m.restoreVM != nil {
		return m.restoreVM(ctx, node, req)
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

	// Backup file list & task status (read-only).
	listBackupFiles func(ctx context.Context, store, backupType, backupID, snapshot, path string) ([]model.PBSFile, error)
	getTaskStatus   func(ctx context.Context, upid string) (*model.PBSTask, error)
	getTaskLog      func(ctx context.Context, upid string, limit int) ([]model.TaskLogEntry, error)

	// Restore (gated).
	restoreFile     func(ctx context.Context, store, backupType, backupID, snapshot string, req model.PBSFileRestoreRequest) error
	restoreVMBackup func(ctx context.Context, store, backupType, backupID, snapshot string, req model.PBSVMRestoreRequest) (string, error)
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
func (m *mockPBSGateway) ListBackupFiles(ctx context.Context, store, backupType, backupID, snapshot, path string) ([]model.PBSFile, error) {
	if m.listBackupFiles != nil {
		return m.listBackupFiles(ctx, store, backupType, backupID, snapshot, path)
	}
	return nil, nil
}
func (m *mockPBSGateway) GetTaskStatus(ctx context.Context, upid string) (*model.PBSTask, error) {
	if m.getTaskStatus != nil {
		return m.getTaskStatus(ctx, upid)
	}
	return nil, nil
}
func (m *mockPBSGateway) GetTaskLog(ctx context.Context, upid string, limit int) ([]model.TaskLogEntry, error) {
	if m.getTaskLog != nil {
		return m.getTaskLog(ctx, upid, limit)
	}
	return nil, nil
}
func (m *mockPBSGateway) RestoreFile(ctx context.Context, store, backupType, backupID, snapshot string, req model.PBSFileRestoreRequest) error {
	if m.restoreFile != nil {
		return m.restoreFile(ctx, store, backupType, backupID, snapshot, req)
	}
	return nil
}
func (m *mockPBSGateway) RestoreVMBackup(ctx context.Context, store, backupType, backupID, snapshot string, req model.PBSVMRestoreRequest) (string, error) {
	if m.restoreVMBackup != nil {
		return m.restoreVMBackup(ctx, store, backupType, backupID, snapshot, req)
	}
	return "", nil
}

var _ port.PBSGateway = (*mockPBSGateway)(nil)
