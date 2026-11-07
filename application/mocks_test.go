package application

import (
	"context"
	"errors"

	"github.com/teran/mcp-proxmox/domain/model"
	"github.com/teran/mcp-proxmox/domain/port"
)

// errMockNotStubbed is returned by a mock method whose backing field function
// was not configured, so an accidental unstubbed call fails loudly instead of
// silently returning a nil value.
var errMockNotStubbed = errors.New("mock method not stubbed")

// mockPVEGateway is a controllable port.PVEGateway mock. Each method calls a
// field function if set, otherwise returns a zero/empty result.
type mockPVEGateway struct {
	listNodes        func(ctx context.Context) ([]model.Node, error)
	getNodeStatus    func(ctx context.Context, node string) (*model.NodeStatus, error)
	getClusterStatus func(ctx context.Context) ([]model.ClusterStatus, error)
	getClusterRes    func(ctx context.Context) ([]model.ClusterResource, error)
	getNextID        func(ctx context.Context) (string, error)
	listBackupJobs   func(ctx context.Context) ([]model.BackupJob, error)
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
	removeHAResource func(ctx context.Context, sid string) error
	startVMBackup    func(ctx context.Context, node string, req model.VMBackupRequest) (*model.Task, error)
	setVMConfig      func(ctx context.Context, node string, vmid int, req model.SetVMConfigRequest) error
	setLXCConfig     func(ctx context.Context, node string, vmid int, req model.SetLXCConfigRequest) error
	createBackupJob  func(ctx context.Context, req model.CreateBackupJobRequest) (string, error)
	updateBackupJob  func(ctx context.Context, id string, req model.CreateBackupJobRequest) (string, error)
	deleteBackupJob  func(ctx context.Context, id string) error

	// LXC create + storage content (feat/lxc-templates).
	createLXC          func(ctx context.Context, node string, req model.CreateLXCRequest) (int, error)
	listStorageContent func(
		ctx context.Context, node, storage string, opts port.StorageContentListOptions,
	) ([]model.StorageContent, error)
	downloadURL func(ctx context.Context, node, storage string, req model.StorageDownloadURLRequest) (string, error)

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
	startLXC    func(ctx context.Context, node string, vmid int) (*model.Task, error)
	stopLXC     func(ctx context.Context, node string, vmid int, skiplock, forceStop bool) (*model.Task, error)
	shutdownLXC func(ctx context.Context, node string, vmid int, forceStop bool, timeout int) (*model.Task, error)
	rebootLXC   func(ctx context.Context, node string, vmid int, timeout int) (*model.Task, error)
	deleteLXC   func(
		ctx context.Context, node string, vmid int,
		purge, destroyUnreferencedDisks, force bool,
	) (*model.Task, error)
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

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) GetNodeStatus(ctx context.Context, node string) (*model.NodeStatus, error) {
	if m.getNodeStatus != nil {
		return m.getNodeStatus(ctx, node)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) GetClusterStatus(ctx context.Context) ([]model.ClusterStatus, error) {
	if m.getClusterStatus != nil {
		return m.getClusterStatus(ctx)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) GetClusterResources(ctx context.Context) ([]model.ClusterResource, error) {
	if m.getClusterRes != nil {
		return m.getClusterRes(ctx)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) GetNextID(ctx context.Context) (string, error) {
	if m.getNextID != nil {
		return m.getNextID(ctx)
	}

	return "", nil
}

func (m *mockPVEGateway) ListBackupJobs(ctx context.Context) ([]model.BackupJob, error) {
	if m.listBackupJobs != nil {
		return m.listBackupJobs(ctx)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) ListVMs(ctx context.Context, node string) ([]model.VM, error) {
	if m.listVMs != nil {
		return m.listVMs(ctx, node)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) GetVMConfig(ctx context.Context, node string, vmid int) (*model.VMConfig, error) {
	if m.getVMConfig != nil {
		return m.getVMConfig(ctx, node, vmid)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) GetVMStatus(ctx context.Context, node string, vmid int) (*model.VMStatus, error) {
	if m.getVMStatus != nil {
		return m.getVMStatus(ctx, node, vmid)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) ListLXCs(ctx context.Context, node string) ([]model.LXC, error) {
	if m.listLXCs != nil {
		return m.listLXCs(ctx, node)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) GetLXCConfig(ctx context.Context, node string, vmid int) (*model.LXCConfig, error) {
	if m.getLXCConfig != nil {
		return m.getLXCConfig(ctx, node, vmid)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) GetLXCStatus(ctx context.Context, node string, vmid int) (*model.LXCStatus, error) {
	if m.getLXCStatus != nil {
		return m.getLXCStatus(ctx, node, vmid)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) ListStorage(ctx context.Context) ([]model.Storage, error) {
	if m.listStorage != nil {
		return m.listStorage(ctx)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) GetStorageStatus(ctx context.Context, node, storage string) (*model.StorageStatus, error) {
	if m.getStorageStatus != nil {
		return m.getStorageStatus(ctx, node, storage)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) ListNetwork(ctx context.Context, node string) ([]model.NetworkInterface, error) {
	if m.listNetwork != nil {
		return m.listNetwork(ctx, node)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) ListTasks(ctx context.Context, node string, opts port.TaskListOptions) ([]model.Task, error) {
	if m.listTasks != nil {
		return m.listTasks(ctx, node, opts)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) GetTaskStatus(ctx context.Context, node, upid string) (*model.TaskStatus, error) {
	if m.getTaskStatus != nil {
		return m.getTaskStatus(ctx, node, upid)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) GetTaskLog(ctx context.Context, node, upid string, limit int) ([]model.TaskLogEntry, error) {
	if m.getTaskLog != nil {
		return m.getTaskLog(ctx, node, upid, limit)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) GetPVEVersion(ctx context.Context) (*model.PVEVersion, error) {
	if m.getPVEVersion != nil {
		return m.getPVEVersion(ctx)
	}

	return nil, errMockNotStubbed
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

func (m *mockPVEGateway) RemoveHAResource(ctx context.Context, sid string) error {
	if m.removeHAResource != nil {
		return m.removeHAResource(ctx, sid)
	}

	return nil
}

func (m *mockPVEGateway) StartVMBackup(ctx context.Context, node string, req model.VMBackupRequest) (*model.Task, error) {
	if m.startVMBackup != nil {
		return m.startVMBackup(ctx, node, req)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) SetVMConfig(ctx context.Context, node string, vmid int, req model.SetVMConfigRequest) error {
	if m.setVMConfig != nil {
		return m.setVMConfig(ctx, node, vmid, req)
	}

	return nil
}

func (m *mockPVEGateway) SetLXCConfig(ctx context.Context, node string, vmid int, req model.SetLXCConfigRequest) error {
	if m.setLXCConfig != nil {
		return m.setLXCConfig(ctx, node, vmid, req)
	}

	return nil
}

func (m *mockPVEGateway) CreateBackupJob(ctx context.Context, req model.CreateBackupJobRequest) (string, error) {
	if m.createBackupJob != nil {
		return m.createBackupJob(ctx, req)
	}

	return "", nil
}

func (m *mockPVEGateway) UpdateBackupJob(ctx context.Context, id string, req model.CreateBackupJobRequest) (string, error) {
	if m.updateBackupJob != nil {
		return m.updateBackupJob(ctx, id, req)
	}

	return "", nil
}

func (m *mockPVEGateway) DeleteBackupJob(ctx context.Context, id string) error {
	if m.deleteBackupJob != nil {
		return m.deleteBackupJob(ctx, id)
	}

	return nil
}

func (m *mockPVEGateway) CreateLXC(ctx context.Context, node string, req model.CreateLXCRequest) (int, error) {
	if m.createLXC != nil {
		return m.createLXC(ctx, node, req)
	}

	return 0, nil
}

func (m *mockPVEGateway) ListStorageContent(
	ctx context.Context, node, storage string, opts port.StorageContentListOptions,
) ([]model.StorageContent, error) {
	if m.listStorageContent != nil {
		return m.listStorageContent(ctx, node, storage, opts)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) DownloadURL(
	ctx context.Context, node, storage string, req model.StorageDownloadURLRequest,
) (string, error) {
	if m.downloadURL != nil {
		return m.downloadURL(ctx, node, storage, req)
	}

	return "", errMockNotStubbed
}

func (m *mockPVEGateway) ListVMSnapshots(ctx context.Context, node string, vmid int) ([]model.Snapshot, error) {
	if m.listVMSnapshots != nil {
		return m.listVMSnapshots(ctx, node, vmid)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) ListLXCSnapshots(ctx context.Context, node string, vmid int) ([]model.Snapshot, error) {
	if m.listLXCSnapshots != nil {
		return m.listLXCSnapshots(ctx, node, vmid)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) StartVM(ctx context.Context, node string, vmid int) (*model.Task, error) {
	if m.startVM != nil {
		return m.startVM(ctx, node, vmid)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) StopVM(ctx context.Context, node string, vmid int, skiplock bool) (*model.Task, error) {
	if m.stopVM != nil {
		return m.stopVM(ctx, node, vmid, skiplock)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) ShutdownVM(
	ctx context.Context, node string, vmid int, forceStop bool, timeout int,
) (*model.Task, error) {
	if m.shutdownVM != nil {
		return m.shutdownVM(ctx, node, vmid, forceStop, timeout)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) RebootVM(ctx context.Context, node string, vmid int, timeout int) (*model.Task, error) {
	if m.rebootVM != nil {
		return m.rebootVM(ctx, node, vmid, timeout)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) ResetVM(ctx context.Context, node string, vmid int) (*model.Task, error) {
	if m.resetVM != nil {
		return m.resetVM(ctx, node, vmid)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) SuspendVM(ctx context.Context, node string, vmid int, todisk bool) (*model.Task, error) {
	if m.suspendVM != nil {
		return m.suspendVM(ctx, node, vmid, todisk)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) ResumeVM(ctx context.Context, node string, vmid int) (*model.Task, error) {
	if m.resumeVM != nil {
		return m.resumeVM(ctx, node, vmid)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) DeleteVM(
	ctx context.Context, node string, vmid int, purge, destroyUnreferencedDisks bool,
) (*model.Task, error) {
	if m.deleteVM != nil {
		return m.deleteVM(ctx, node, vmid, purge, destroyUnreferencedDisks)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) CloneVM(ctx context.Context, node string, vmid int, req model.CloneVMRequest) (*model.Task, error) {
	if m.cloneVM != nil {
		return m.cloneVM(ctx, node, vmid, req)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) CreateVMSnapshot(
	ctx context.Context, node string, vmid int, req model.SnapshotCreateRequest,
) (*model.Task, error) {
	if m.createVMSnapshot != nil {
		return m.createVMSnapshot(ctx, node, vmid, req)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) DeleteVMSnapshot(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
	if m.deleteVMSnapshot != nil {
		return m.deleteVMSnapshot(ctx, node, vmid, snapname)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) RollbackVMSnapshot(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
	if m.rollbackVMSnapshot != nil {
		return m.rollbackVMSnapshot(ctx, node, vmid, snapname)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) StartLXC(ctx context.Context, node string, vmid int) (*model.Task, error) {
	if m.startLXC != nil {
		return m.startLXC(ctx, node, vmid)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) StopLXC(ctx context.Context, node string, vmid int, skiplock, forceStop bool) (*model.Task, error) {
	if m.stopLXC != nil {
		return m.stopLXC(ctx, node, vmid, skiplock, forceStop)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) ShutdownLXC(
	ctx context.Context, node string, vmid int, forceStop bool, timeout int,
) (*model.Task, error) {
	if m.shutdownLXC != nil {
		return m.shutdownLXC(ctx, node, vmid, forceStop, timeout)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) RebootLXC(ctx context.Context, node string, vmid int, timeout int) (*model.Task, error) {
	if m.rebootLXC != nil {
		return m.rebootLXC(ctx, node, vmid, timeout)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) DeleteLXC(
	ctx context.Context, node string, vmid int,
	purge, destroyUnreferencedDisks, force bool,
) (*model.Task, error) {
	if m.deleteLXC != nil {
		return m.deleteLXC(ctx, node, vmid, purge, destroyUnreferencedDisks, force)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) CloneLXC(ctx context.Context, node string, vmid int, req model.CloneLXCRequest) (*model.Task, error) {
	if m.cloneLXC != nil {
		return m.cloneLXC(ctx, node, vmid, req)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) CreateLXCSnapshot(
	ctx context.Context, node string, vmid int, req model.SnapshotCreateRequest,
) (*model.Task, error) {
	if m.createLXCSnapshot != nil {
		return m.createLXCSnapshot(ctx, node, vmid, req)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) DeleteLXCSnapshot(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
	if m.deleteLXCSnapshot != nil {
		return m.deleteLXCSnapshot(ctx, node, vmid, snapname)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) RollbackLXCSnapshot(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
	if m.rollbackLXCSnapshot != nil {
		return m.rollbackLXCSnapshot(ctx, node, vmid, snapname)
	}

	return nil, errMockNotStubbed
}

func (m *mockPVEGateway) RestoreVM(ctx context.Context, node string, req model.PVERestoreRequest) (*model.Task, error) {
	if m.restoreVM != nil {
		return m.restoreVM(ctx, node, req)
	}

	return nil, errMockNotStubbed
}

var _ port.PVEGateway = (*mockPVEGateway)(nil)

// mockPBSGateway is a controllable port.PBSGateway mock.
type mockPBSGateway struct {
	listDatastores     func(ctx context.Context) ([]model.Datastore, error)
	getDatastoreStats  func(ctx context.Context, store string) (*model.DatastoreStatus, error)
	getDatastoreConfig func(ctx context.Context, store string) (*model.DatastoreConfig, error)
	listBackups        func(ctx context.Context, store string, opts port.PBSBackupListOptions) ([]model.Backup, error)
	getBackup          func(ctx context.Context, store, backupID string, opts port.PBSBackupListOptions) ([]model.Backup, error)
	getBackupNotes     func(ctx context.Context, store, backupID, backupType string) (*model.BackupNotes, error)
	getVerifyStatus    func(ctx context.Context, upid string) (*model.PBSTask, error)
	getPruneStatus     func(ctx context.Context, upid string) (*model.PBSTask, error)
	getPBSVersion      func(ctx context.Context) (*model.PBSVersion, error)
	startVerify        func(ctx context.Context, store string) (string, error)
	startGC            func(ctx context.Context, store string) (string, error)
	startPrune         func(ctx context.Context, store string) (string, error)
	startSync          func(ctx context.Context, store string, req model.PBSSyncRequest) (string, error)
	stopTask           func(ctx context.Context, upid string) error
	forgetBackup       func(ctx context.Context, store, backupType, backupID string, backupTime int64) error

	// Backup file list & task status (read-only).
	listBackupFiles func(
		ctx context.Context, store, backupType, backupID string, backupTime int64, path string,
	) ([]model.PBSFile, error)
	getTaskStatus func(ctx context.Context, upid string) (*model.PBSTask, error)
	getTaskLog    func(ctx context.Context, upid string, limit int) ([]model.TaskLogEntry, error)

	// Restore (gated).
	restoreFile func(
		ctx context.Context, store, backupType, backupID string, backupTime int64, req model.PBSFileRestoreRequest,
	) error
	restoreVMBackup func(
		ctx context.Context, store, backupType, backupID string, backupTime int64,
		req model.PBSVMRestoreRequest,
	) (string, error)
}

func (m *mockPBSGateway) ListDatastores(ctx context.Context) ([]model.Datastore, error) {
	if m.listDatastores != nil {
		return m.listDatastores(ctx)
	}

	return nil, errMockNotStubbed
}

func (m *mockPBSGateway) GetDatastoreStatus(ctx context.Context, store string) (*model.DatastoreStatus, error) {
	if m.getDatastoreStats != nil {
		return m.getDatastoreStats(ctx, store)
	}

	return nil, errMockNotStubbed
}

func (m *mockPBSGateway) GetDatastoreConfig(ctx context.Context, store string) (*model.DatastoreConfig, error) {
	if m.getDatastoreConfig != nil {
		return m.getDatastoreConfig(ctx, store)
	}

	return nil, errMockNotStubbed
}

func (m *mockPBSGateway) ListBackups(ctx context.Context, store string, opts port.PBSBackupListOptions) ([]model.Backup, error) {
	if m.listBackups != nil {
		return m.listBackups(ctx, store, opts)
	}

	return nil, errMockNotStubbed
}

func (m *mockPBSGateway) GetBackup(
	ctx context.Context, store, backupID string, opts port.PBSBackupListOptions,
) ([]model.Backup, error) {
	if m.getBackup != nil {
		return m.getBackup(ctx, store, backupID, opts)
	}

	return nil, errMockNotStubbed
}

func (m *mockPBSGateway) GetBackupNotes(ctx context.Context, store, backupID, backupType string) (*model.BackupNotes, error) {
	if m.getBackupNotes != nil {
		return m.getBackupNotes(ctx, store, backupID, backupType)
	}

	return nil, errMockNotStubbed
}

func (m *mockPBSGateway) GetVerifyStatus(ctx context.Context, upid string) (*model.PBSTask, error) {
	if m.getVerifyStatus != nil {
		return m.getVerifyStatus(ctx, upid)
	}

	return nil, errMockNotStubbed
}

func (m *mockPBSGateway) GetPruneStatus(ctx context.Context, upid string) (*model.PBSTask, error) {
	if m.getPruneStatus != nil {
		return m.getPruneStatus(ctx, upid)
	}

	return nil, errMockNotStubbed
}

func (m *mockPBSGateway) GetPBSVersion(ctx context.Context) (*model.PBSVersion, error) {
	if m.getPBSVersion != nil {
		return m.getPBSVersion(ctx)
	}

	return nil, errMockNotStubbed
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

func (m *mockPBSGateway) StopTask(ctx context.Context, upid string) error {
	if m.stopTask != nil {
		return m.stopTask(ctx, upid)
	}

	return nil
}

func (m *mockPBSGateway) ForgetBackup(
	ctx context.Context, store, backupType, backupID string, backupTime int64,
) error {
	if m.forgetBackup != nil {
		return m.forgetBackup(ctx, store, backupType, backupID, backupTime)
	}

	return nil
}

func (m *mockPBSGateway) ListBackupFiles(
	ctx context.Context, store, backupType, backupID string, backupTime int64, path string,
) ([]model.PBSFile, error) {
	if m.listBackupFiles != nil {
		return m.listBackupFiles(ctx, store, backupType, backupID, backupTime, path)
	}

	return nil, errMockNotStubbed
}

func (m *mockPBSGateway) GetTaskStatus(ctx context.Context, upid string) (*model.PBSTask, error) {
	if m.getTaskStatus != nil {
		return m.getTaskStatus(ctx, upid)
	}

	return nil, errMockNotStubbed
}

func (m *mockPBSGateway) GetTaskLog(ctx context.Context, upid string, limit int) ([]model.TaskLogEntry, error) {
	if m.getTaskLog != nil {
		return m.getTaskLog(ctx, upid, limit)
	}

	return nil, errMockNotStubbed
}

func (m *mockPBSGateway) RestoreFile(
	ctx context.Context, store, backupType, backupID string, backupTime int64, req model.PBSFileRestoreRequest,
) error {
	if m.restoreFile != nil {
		return m.restoreFile(ctx, store, backupType, backupID, backupTime, req)
	}

	return nil
}

func (m *mockPBSGateway) RestoreVMBackup(
	ctx context.Context, store, backupType, backupID string, backupTime int64, req model.PBSVMRestoreRequest,
) (string, error) {
	if m.restoreVMBackup != nil {
		return m.restoreVMBackup(ctx, store, backupType, backupID, backupTime, req)
	}

	return "", nil
}

var _ port.PBSGateway = (*mockPBSGateway)(nil)

// mockCephGateway is a controllable port.CephGateway mock. Each method calls a
// field function if set, otherwise returns a zero/empty result (getters) or
// errMockNotStubbed, so an accidental unstubbed call fails loudly.
type mockCephGateway struct {
	getClusterStatus func(ctx context.Context) (*model.CephClusterStatus, error)
	getClusterConfig func(ctx context.Context) ([]model.CephConfigEntry, error)
	getClusterLog    func(ctx context.Context, limit int) ([]model.CephLogEntry, error)
	getClusterMeta   func(ctx context.Context) (*model.CephClusterMetadata, error)
	getNodeStatus    func(ctx context.Context, node string) (*model.CephNodeStatus, error)
	getNodeConfig    func(ctx context.Context, node string) ([]model.CephConfigEntry, error)
	getNodeConfigDB  func(ctx context.Context, node string) ([]model.CephConfigEntry, error)
	getNodeFlags     func(ctx context.Context, node string) ([]model.CephFlag, error)
	getNodeMeta      func(ctx context.Context, node string) (*model.CephNodeMetadata, error)
	getNodeCrush     func(ctx context.Context, node string) (*model.CephCrushMap, error)
	getNodeRules     func(ctx context.Context, node string) ([]model.CephCrushRule, error)
	listPools        func(ctx context.Context, node string) ([]model.CephPool, error)
	listOSDs         func(ctx context.Context, node string) ([]model.CephOSD, error)
	listMons         func(ctx context.Context, node string) ([]model.CephMon, error)
	listMgrs         func(ctx context.Context, node string) ([]model.CephMgr, error)

	createCrushRule func(ctx context.Context, node string, req model.CephCrushRuleCreateRequest) (string, error)
	deleteCrushRule func(ctx context.Context, node, name string) error
	createPool      func(ctx context.Context, node string, req model.CephPoolCreateRequest) (string, error)
	updatePool      func(ctx context.Context, node, name string, req model.CephPoolUpdateRequest) error
	deletePool      func(ctx context.Context, node, name string) error
	createOSD       func(ctx context.Context, node string, req model.CephOSDCreateRequest) (string, error)
	deleteOSD       func(ctx context.Context, node string, osdid int) error
	createMon       func(ctx context.Context, node string, req model.CephMonCreateRequest) (string, error)
	deleteMon       func(ctx context.Context, node, monid string) error
	createMgr       func(ctx context.Context, node string, req model.CephMgrCreateRequest) (string, error)
	setFlags        func(ctx context.Context, node string, req model.CephFlagsSetRequest) error
	setConfig       func(ctx context.Context, node string, req model.CephConfigSetRequest) error
	initCeph        func(ctx context.Context, node string, req model.CephInitRequest) (string, error)
	destroyCeph     func(ctx context.Context, node string, req model.CephDestroyRequest) error
}

func (m *mockCephGateway) GetClusterStatus(ctx context.Context) (*model.CephClusterStatus, error) {
	if m.getClusterStatus != nil {
		return m.getClusterStatus(ctx)
	}

	return nil, errMockNotStubbed
}

func (m *mockCephGateway) GetClusterConfig(ctx context.Context) ([]model.CephConfigEntry, error) {
	if m.getClusterConfig != nil {
		return m.getClusterConfig(ctx)
	}

	return nil, errMockNotStubbed
}

func (m *mockCephGateway) GetClusterLog(ctx context.Context, limit int) ([]model.CephLogEntry, error) {
	if m.getClusterLog != nil {
		return m.getClusterLog(ctx, limit)
	}

	return nil, errMockNotStubbed
}

func (m *mockCephGateway) GetClusterMetadata(ctx context.Context) (*model.CephClusterMetadata, error) {
	if m.getClusterMeta != nil {
		return m.getClusterMeta(ctx)
	}

	return nil, errMockNotStubbed
}

func (m *mockCephGateway) GetNodeStatus(ctx context.Context, node string) (*model.CephNodeStatus, error) {
	if m.getNodeStatus != nil {
		return m.getNodeStatus(ctx, node)
	}

	return nil, errMockNotStubbed
}

func (m *mockCephGateway) GetNodeConfig(ctx context.Context, node string) ([]model.CephConfigEntry, error) {
	if m.getNodeConfig != nil {
		return m.getNodeConfig(ctx, node)
	}

	return nil, errMockNotStubbed
}

func (m *mockCephGateway) GetNodeConfigDB(ctx context.Context, node string) ([]model.CephConfigEntry, error) {
	if m.getNodeConfigDB != nil {
		return m.getNodeConfigDB(ctx, node)
	}

	return nil, errMockNotStubbed
}

func (m *mockCephGateway) GetNodeFlags(ctx context.Context, node string) ([]model.CephFlag, error) {
	if m.getNodeFlags != nil {
		return m.getNodeFlags(ctx, node)
	}

	return nil, errMockNotStubbed
}

func (m *mockCephGateway) GetNodeMetadata(ctx context.Context, node string) (*model.CephNodeMetadata, error) {
	if m.getNodeMeta != nil {
		return m.getNodeMeta(ctx, node)
	}

	return nil, errMockNotStubbed
}

func (m *mockCephGateway) GetNodeCrush(ctx context.Context, node string) (*model.CephCrushMap, error) {
	if m.getNodeCrush != nil {
		return m.getNodeCrush(ctx, node)
	}

	return nil, errMockNotStubbed
}

func (m *mockCephGateway) GetNodeRules(ctx context.Context, node string) ([]model.CephCrushRule, error) {
	if m.getNodeRules != nil {
		return m.getNodeRules(ctx, node)
	}

	return nil, errMockNotStubbed
}

func (m *mockCephGateway) ListPools(ctx context.Context, node string) ([]model.CephPool, error) {
	if m.listPools != nil {
		return m.listPools(ctx, node)
	}

	return nil, errMockNotStubbed
}

func (m *mockCephGateway) ListOSDs(ctx context.Context, node string) ([]model.CephOSD, error) {
	if m.listOSDs != nil {
		return m.listOSDs(ctx, node)
	}

	return nil, errMockNotStubbed
}

func (m *mockCephGateway) ListMons(ctx context.Context, node string) ([]model.CephMon, error) {
	if m.listMons != nil {
		return m.listMons(ctx, node)
	}

	return nil, errMockNotStubbed
}

func (m *mockCephGateway) ListMgrs(ctx context.Context, node string) ([]model.CephMgr, error) {
	if m.listMgrs != nil {
		return m.listMgrs(ctx, node)
	}

	return nil, errMockNotStubbed
}

func (m *mockCephGateway) CreateCrushRule(
	ctx context.Context, node string, req model.CephCrushRuleCreateRequest,
) (string, error) {
	if m.createCrushRule != nil {
		return m.createCrushRule(ctx, node, req)
	}

	return "", nil
}

func (m *mockCephGateway) DeleteCrushRule(ctx context.Context, node, name string) error {
	if m.deleteCrushRule != nil {
		return m.deleteCrushRule(ctx, node, name)
	}

	return nil
}

func (m *mockCephGateway) CreatePool(ctx context.Context, node string, req model.CephPoolCreateRequest) (string, error) {
	if m.createPool != nil {
		return m.createPool(ctx, node, req)
	}

	return "", nil
}

func (m *mockCephGateway) UpdatePool(
	ctx context.Context, node, name string, req model.CephPoolUpdateRequest,
) error {
	if m.updatePool != nil {
		return m.updatePool(ctx, node, name, req)
	}

	return nil
}

func (m *mockCephGateway) DeletePool(ctx context.Context, node, name string) error {
	if m.deletePool != nil {
		return m.deletePool(ctx, node, name)
	}

	return nil
}

func (m *mockCephGateway) CreateOSD(ctx context.Context, node string, req model.CephOSDCreateRequest) (string, error) {
	if m.createOSD != nil {
		return m.createOSD(ctx, node, req)
	}

	return "", nil
}

func (m *mockCephGateway) DeleteOSD(ctx context.Context, node string, osdid int) error {
	if m.deleteOSD != nil {
		return m.deleteOSD(ctx, node, osdid)
	}

	return nil
}

func (m *mockCephGateway) CreateMon(ctx context.Context, node string, req model.CephMonCreateRequest) (string, error) {
	if m.createMon != nil {
		return m.createMon(ctx, node, req)
	}

	return "", nil
}

func (m *mockCephGateway) DeleteMon(ctx context.Context, node, monid string) error {
	if m.deleteMon != nil {
		return m.deleteMon(ctx, node, monid)
	}

	return nil
}

func (m *mockCephGateway) CreateMgr(ctx context.Context, node string, req model.CephMgrCreateRequest) (string, error) {
	if m.createMgr != nil {
		return m.createMgr(ctx, node, req)
	}

	return "", nil
}

func (m *mockCephGateway) SetFlags(ctx context.Context, node string, req model.CephFlagsSetRequest) error {
	if m.setFlags != nil {
		return m.setFlags(ctx, node, req)
	}

	return nil
}

func (m *mockCephGateway) SetConfig(ctx context.Context, node string, req model.CephConfigSetRequest) error {
	if m.setConfig != nil {
		return m.setConfig(ctx, node, req)
	}

	return nil
}

func (m *mockCephGateway) InitCeph(ctx context.Context, node string, req model.CephInitRequest) (string, error) {
	if m.initCeph != nil {
		return m.initCeph(ctx, node, req)
	}

	return "", nil
}

func (m *mockCephGateway) DestroyCeph(ctx context.Context, node string, req model.CephDestroyRequest) error {
	if m.destroyCeph != nil {
		return m.destroyCeph(ctx, node, req)
	}

	return nil
}

var _ port.CephGateway = (*mockCephGateway)(nil)
