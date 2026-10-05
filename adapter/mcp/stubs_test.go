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
	startVMBackup    func(ctx context.Context, node string, req model.VMBackupRequest) (*model.Task, error)

	listVMSnapshots     func(ctx context.Context, node string, vmid int) ([]model.Snapshot, error)
	listLXCSnapshots    func(ctx context.Context, node string, vmid int) ([]model.Snapshot, error)
	startVM             func(ctx context.Context, node string, vmid int) (*model.Task, error)
	stopVM              func(ctx context.Context, node string, vmid int, skiplock bool) (*model.Task, error)
	shutdownVM          func(ctx context.Context, node string, vmid int, forceStop bool, timeout int) (*model.Task, error)
	rebootVM            func(ctx context.Context, node string, vmid int, timeout int) (*model.Task, error)
	resetVM             func(ctx context.Context, node string, vmid int) (*model.Task, error)
	suspendVM           func(ctx context.Context, node string, vmid int, todisk bool) (*model.Task, error)
	resumeVM            func(ctx context.Context, node string, vmid int) (*model.Task, error)
	deleteVM            func(ctx context.Context, node string, vmid int, purge, destroyUnreferencedDisks bool) (*model.Task, error)
	cloneVM             func(ctx context.Context, node string, vmid int, req model.CloneVMRequest) (*model.Task, error)
	createVMSnapshot    func(ctx context.Context, node string, vmid int, req model.SnapshotCreateRequest) (*model.Task, error)
	deleteVMSnapshot    func(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error)
	rollbackVMSnapshot  func(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error)
	startLXC            func(ctx context.Context, node string, vmid int) (*model.Task, error)
	stopLXC             func(ctx context.Context, node string, vmid int, skiplock, forceStop bool) (*model.Task, error)
	shutdownLXC         func(ctx context.Context, node string, vmid int, forceStop bool, timeout int) (*model.Task, error)
	rebootLXC           func(ctx context.Context, node string, vmid int, timeout int) (*model.Task, error)
	deleteLXC           func(ctx context.Context, node string, vmid int, purge, destroyUnreferencedDisks, force bool) (*model.Task, error)
	cloneLXC            func(ctx context.Context, node string, vmid int, req model.CloneLXCRequest) (*model.Task, error)
	createLXCSnapshot   func(ctx context.Context, node string, vmid int, req model.SnapshotCreateRequest) (*model.Task, error)
	deleteLXCSnapshot   func(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error)
	rollbackLXCSnapshot func(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error)
	restoreVM           func(ctx context.Context, node string, req model.PVERestoreRequest) (*model.Task, error)
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
	return []model.ClusterStatus{{Name: "cluster", Quorum: 1}}, nil
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
func (s *stubPVEGateway) GetNextID(ctx context.Context) (string, error) {
	if s.fail() {
		return "", s.err
	}
	if s.getNextID != nil {
		return s.getNextID(ctx)
	}
	return "201", nil
}
func (s *stubPVEGateway) ListBackupJobs(ctx context.Context) ([]model.BackupJob, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.listBackupJobs != nil {
		return s.listBackupJobs(ctx)
	}
	return []model.BackupJob{{ID: "backup-100", Enabled: true, Schedule: "daily", Storage: "local"}}, nil
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
func (s *stubPVEGateway) CreateVM(ctx context.Context, node string, req model.CreateVMRequest) (int, error) {
	if s.fail() {
		return 0, s.err
	}
	if s.createVM != nil {
		return s.createVM(ctx, node, req)
	}
	return 100, nil
}
func (s *stubPVEGateway) ResizeVM(ctx context.Context, node string, vmid int, req model.ResizeVMRequest) error {
	if s.fail() {
		return s.err
	}
	if s.resizeVM != nil {
		return s.resizeVM(ctx, node, vmid, req)
	}
	return nil
}
func (s *stubPVEGateway) MigrateVM(ctx context.Context, node string, vmid int, req model.MigrateVMRequest) error {
	if s.fail() {
		return s.err
	}
	if s.migrateVM != nil {
		return s.migrateVM(ctx, node, vmid, req)
	}
	return nil
}
func (s *stubPVEGateway) AddHAResource(ctx context.Context, req model.HAResourceRequest) error {
	if s.fail() {
		return s.err
	}
	if s.addHAResource != nil {
		return s.addHAResource(ctx, req)
	}
	return nil
}
func (s *stubPVEGateway) StartVMBackup(ctx context.Context, node string, req model.VMBackupRequest) (*model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.startVMBackup != nil {
		return s.startVMBackup(ctx, node, req)
	}
	return &model.Task{UPID: "UPID:pve:backup:..."}, nil
}
func (s *stubPVEGateway) ListVMSnapshots(ctx context.Context, node string, vmid int) ([]model.Snapshot, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.listVMSnapshots != nil {
		return s.listVMSnapshots(ctx, node, vmid)
	}
	return []model.Snapshot{{Name: "snap1"}}, nil
}
func (s *stubPVEGateway) ListLXCSnapshots(ctx context.Context, node string, vmid int) ([]model.Snapshot, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.listLXCSnapshots != nil {
		return s.listLXCSnapshots(ctx, node, vmid)
	}
	return []model.Snapshot{{Name: "snap1"}}, nil
}
func (s *stubPVEGateway) StartVM(ctx context.Context, node string, vmid int) (*model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.startVM != nil {
		return s.startVM(ctx, node, vmid)
	}
	return &model.Task{UPID: "UPID:pve:start:..."}, nil
}
func (s *stubPVEGateway) StopVM(ctx context.Context, node string, vmid int, skiplock bool) (*model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.stopVM != nil {
		return s.stopVM(ctx, node, vmid, skiplock)
	}
	return &model.Task{UPID: "UPID:pve:stop:..."}, nil
}
func (s *stubPVEGateway) ShutdownVM(ctx context.Context, node string, vmid int, forceStop bool, timeout int) (*model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.shutdownVM != nil {
		return s.shutdownVM(ctx, node, vmid, forceStop, timeout)
	}
	return &model.Task{UPID: "UPID:pve:shutdown:..."}, nil
}
func (s *stubPVEGateway) RebootVM(ctx context.Context, node string, vmid int, timeout int) (*model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.rebootVM != nil {
		return s.rebootVM(ctx, node, vmid, timeout)
	}
	return &model.Task{UPID: "UPID:pve:reboot:..."}, nil
}
func (s *stubPVEGateway) ResetVM(ctx context.Context, node string, vmid int) (*model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.resetVM != nil {
		return s.resetVM(ctx, node, vmid)
	}
	return &model.Task{UPID: "UPID:pve:reset:..."}, nil
}
func (s *stubPVEGateway) SuspendVM(ctx context.Context, node string, vmid int, todisk bool) (*model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.suspendVM != nil {
		return s.suspendVM(ctx, node, vmid, todisk)
	}
	return &model.Task{UPID: "UPID:pve:suspend:..."}, nil
}
func (s *stubPVEGateway) ResumeVM(ctx context.Context, node string, vmid int) (*model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.resumeVM != nil {
		return s.resumeVM(ctx, node, vmid)
	}
	return &model.Task{UPID: "UPID:pve:resume:..."}, nil
}
func (s *stubPVEGateway) DeleteVM(ctx context.Context, node string, vmid int, purge, destroyUnreferencedDisks bool) (*model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.deleteVM != nil {
		return s.deleteVM(ctx, node, vmid, purge, destroyUnreferencedDisks)
	}
	return &model.Task{UPID: "UPID:pve:delete:..."}, nil
}
func (s *stubPVEGateway) CloneVM(ctx context.Context, node string, vmid int, req model.CloneVMRequest) (*model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.cloneVM != nil {
		return s.cloneVM(ctx, node, vmid, req)
	}
	return &model.Task{UPID: "UPID:pve:clone:..."}, nil
}
func (s *stubPVEGateway) CreateVMSnapshot(ctx context.Context, node string, vmid int, req model.SnapshotCreateRequest) (*model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.createVMSnapshot != nil {
		return s.createVMSnapshot(ctx, node, vmid, req)
	}
	return &model.Task{UPID: "UPID:pve:snapcreate:..."}, nil
}
func (s *stubPVEGateway) DeleteVMSnapshot(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.deleteVMSnapshot != nil {
		return s.deleteVMSnapshot(ctx, node, vmid, snapname)
	}
	return &model.Task{UPID: "UPID:pve:snapdelete:..."}, nil
}
func (s *stubPVEGateway) RollbackVMSnapshot(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.rollbackVMSnapshot != nil {
		return s.rollbackVMSnapshot(ctx, node, vmid, snapname)
	}
	return &model.Task{UPID: "UPID:pve:snaprollback:..."}, nil
}
func (s *stubPVEGateway) StartLXC(ctx context.Context, node string, vmid int) (*model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.startLXC != nil {
		return s.startLXC(ctx, node, vmid)
	}
	return &model.Task{UPID: "UPID:pve:lxcstart:..."}, nil
}
func (s *stubPVEGateway) StopLXC(ctx context.Context, node string, vmid int, skiplock, forceStop bool) (*model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.stopLXC != nil {
		return s.stopLXC(ctx, node, vmid, skiplock, forceStop)
	}
	return &model.Task{UPID: "UPID:pve:lxcstop:..."}, nil
}
func (s *stubPVEGateway) ShutdownLXC(ctx context.Context, node string, vmid int, forceStop bool, timeout int) (*model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.shutdownLXC != nil {
		return s.shutdownLXC(ctx, node, vmid, forceStop, timeout)
	}
	return &model.Task{UPID: "UPID:pve:lxcshutdown:..."}, nil
}
func (s *stubPVEGateway) RebootLXC(ctx context.Context, node string, vmid int, timeout int) (*model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.rebootLXC != nil {
		return s.rebootLXC(ctx, node, vmid, timeout)
	}
	return &model.Task{UPID: "UPID:pve:lxcreboot:..."}, nil
}
func (s *stubPVEGateway) DeleteLXC(ctx context.Context, node string, vmid int, purge, destroyUnreferencedDisks, force bool) (*model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.deleteLXC != nil {
		return s.deleteLXC(ctx, node, vmid, purge, destroyUnreferencedDisks, force)
	}
	return &model.Task{UPID: "UPID:pve:lxcdelete:..."}, nil
}
func (s *stubPVEGateway) CloneLXC(ctx context.Context, node string, vmid int, req model.CloneLXCRequest) (*model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.cloneLXC != nil {
		return s.cloneLXC(ctx, node, vmid, req)
	}
	return &model.Task{UPID: "UPID:pve:lxcclone:..."}, nil
}
func (s *stubPVEGateway) CreateLXCSnapshot(ctx context.Context, node string, vmid int, req model.SnapshotCreateRequest) (*model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.createLXCSnapshot != nil {
		return s.createLXCSnapshot(ctx, node, vmid, req)
	}
	return &model.Task{UPID: "UPID:pve:lxcsnapcreate:..."}, nil
}
func (s *stubPVEGateway) DeleteLXCSnapshot(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.deleteLXCSnapshot != nil {
		return s.deleteLXCSnapshot(ctx, node, vmid, snapname)
	}
	return &model.Task{UPID: "UPID:pve:lxcsnapdelete:..."}, nil
}
func (s *stubPVEGateway) RollbackLXCSnapshot(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.rollbackLXCSnapshot != nil {
		return s.rollbackLXCSnapshot(ctx, node, vmid, snapname)
	}
	return &model.Task{UPID: "UPID:pve:lxcsnaprollback:..."}, nil
}
func (s *stubPVEGateway) RestoreVM(ctx context.Context, node string, req model.PVERestoreRequest) (*model.Task, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.restoreVM != nil {
		return s.restoreVM(ctx, node, req)
	}
	return &model.Task{UPID: "UPID:pve:restore:..."}, nil
}

var _ port.PVEGateway = (*stubPVEGateway)(nil)

// stubPBSGateway is a controllable port.PBSGateway stub for adapter/mcp tests.
type stubPBSGateway struct {
	err error

	listDatastores     func(ctx context.Context) ([]model.Datastore, error)
	getDatastoreStats  func(ctx context.Context, store string) (*model.DatastoreStatus, error)
	getDatastoreConfig func(ctx context.Context, store string) (*model.DatastoreConfig, error)
	listBackups        func(ctx context.Context, store string) ([]model.Backup, error)
	getBackup          func(ctx context.Context, store, backupID string) ([]model.Backup, error)
	getBackupNotes     func(ctx context.Context, store, backupID, backupType string) (*model.BackupNotes, error)
	getVerifyStatus    func(ctx context.Context, store, upid string) (*model.VerifyStatus, error)
	getPruneStatus     func(ctx context.Context, store, upid string) (*model.PruneStatus, error)
	getPBSVersion      func(ctx context.Context) (*model.PBSVersion, error)
	startVerify        func(ctx context.Context, store string) (string, error)
	startGC            func(ctx context.Context, store string) (string, error)
	startPrune         func(ctx context.Context, store string) (string, error)
	startSync          func(ctx context.Context, store string, req model.PBSSyncRequest) (string, error)

	listBackupFiles func(ctx context.Context, store, backupType, backupID, snapshot, path string) ([]model.PBSFile, error)
	getTaskStatus   func(ctx context.Context, upid string) (*model.PBSTask, error)
	getTaskLog      func(ctx context.Context, upid string, limit int) ([]model.TaskLogEntry, error)
	restoreFile     func(ctx context.Context, store, backupType, backupID, snapshot string, req model.PBSFileRestoreRequest) error
	restoreVMBackup func(ctx context.Context, store, backupType, backupID, snapshot string, req model.PBSVMRestoreRequest) (string, error)
}

func (s *stubPBSGateway) fail() bool { return s.err != nil }

func (s *stubPBSGateway) ListDatastores(ctx context.Context) ([]model.Datastore, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.listDatastores != nil {
		return s.listDatastores(ctx)
	}
	return []model.Datastore{{Store: "backup"}}, nil
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
func (s *stubPBSGateway) GetDatastoreConfig(ctx context.Context, store string) (*model.DatastoreConfig, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getDatastoreConfig != nil {
		return s.getDatastoreConfig(ctx, store)
	}
	return &model.DatastoreConfig{Path: "/backup"}, nil
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
func (s *stubPBSGateway) GetBackup(ctx context.Context, store, backupID string) ([]model.Backup, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getBackup != nil {
		return s.getBackup(ctx, store, backupID)
	}
	return []model.Backup{{BackupID: backupID}}, nil
}
func (s *stubPBSGateway) GetBackupNotes(ctx context.Context, store, backupID, backupType string) (*model.BackupNotes, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getBackupNotes != nil {
		return s.getBackupNotes(ctx, store, backupID, backupType)
	}
	return &model.BackupNotes{Notes: "keep"}, nil
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
func (s *stubPBSGateway) StartVerify(ctx context.Context, store string) (string, error) {
	if s.fail() {
		return "", s.err
	}
	if s.startVerify != nil {
		return s.startVerify(ctx, store)
	}
	return "UPID:pbs:verify:...", nil
}
func (s *stubPBSGateway) StartGC(ctx context.Context, store string) (string, error) {
	if s.fail() {
		return "", s.err
	}
	if s.startGC != nil {
		return s.startGC(ctx, store)
	}
	return "UPID:pbs:gc:...", nil
}
func (s *stubPBSGateway) StartPrune(ctx context.Context, store string) (string, error) {
	if s.fail() {
		return "", s.err
	}
	if s.startPrune != nil {
		return s.startPrune(ctx, store)
	}
	return "UPID:pbs:prune:...", nil
}
func (s *stubPBSGateway) StartSync(ctx context.Context, store string, req model.PBSSyncRequest) (string, error) {
	if s.fail() {
		return "", s.err
	}
	if s.startSync != nil {
		return s.startSync(ctx, store, req)
	}
	return "UPID:pbs:sync:...", nil
}
func (s *stubPBSGateway) ListBackupFiles(ctx context.Context, store, backupType, backupID, snapshot, path string) ([]model.PBSFile, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.listBackupFiles != nil {
		return s.listBackupFiles(ctx, store, backupType, backupID, snapshot, path)
	}
	return []model.PBSFile{{Filename: "client.logidx"}}, nil
}
func (s *stubPBSGateway) GetTaskStatus(ctx context.Context, upid string) (*model.PBSTask, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getTaskStatus != nil {
		return s.getTaskStatus(ctx, upid)
	}
	return &model.PBSTask{UPID: upid, Status: "running"}, nil
}
func (s *stubPBSGateway) GetTaskLog(ctx context.Context, upid string, limit int) ([]model.TaskLogEntry, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getTaskLog != nil {
		return s.getTaskLog(ctx, upid, limit)
	}
	return []model.TaskLogEntry{{LineNumber: 1, Text: "OK"}}, nil
}
func (s *stubPBSGateway) RestoreFile(ctx context.Context, store, backupType, backupID, snapshot string, req model.PBSFileRestoreRequest) error {
	if s.fail() {
		return s.err
	}
	if s.restoreFile != nil {
		return s.restoreFile(ctx, store, backupType, backupID, snapshot, req)
	}
	return nil
}
func (s *stubPBSGateway) RestoreVMBackup(ctx context.Context, store, backupType, backupID, snapshot string, req model.PBSVMRestoreRequest) (string, error) {
	if s.fail() {
		return "", s.err
	}
	if s.restoreVMBackup != nil {
		return s.restoreVMBackup(ctx, store, backupType, backupID, snapshot, req)
	}
	return "UPID:pbs:restore:...", nil
}

var _ port.PBSGateway = (*stubPBSGateway)(nil)

// nopLogger is a no-op port.AppLogger for tests.
type nopLogger struct{}

func (nopLogger) Tracef(string, ...any) {}
func (nopLogger) Debugf(string, ...any) {}
func (nopLogger) Infof(string, ...any)  {}
func (nopLogger) Warnf(string, ...any)  {}
func (nopLogger) Errorf(string, ...any) {}

// pveROTools is the read-only PVE tool surface (SPEC.md §6.2–6.6).
var pveROTools = []string{
	"pve_node_list", "pve_node_status", "pve_cluster_status",
	"pve_cluster_resources", "pve_nextid", "pve_backup_job_list",
	"pve_vm_list", "pve_vm_get", "pve_vm_status",
	"pve_lxc_list", "pve_lxc_get", "pve_lxc_status",
	"pve_storage_list", "pve_storage_get", "pve_network_list",
	"pve_task_list", "pve_task_status", "pve_task_log", "pve_version",
	"pve_vm_snapshot_list", "pve_lxc_snapshot_list",
}

// pbsROTools is the read-only PBS tool surface (SPEC.md §6.7–6.9).
var pbsROTools = []string{
	"pbs_datastore_list", "pbs_datastore_status", "pbs_datastore_config_get",
	"pbs_backup_list", "pbs_backup_get", "pbs_backup_notes_get",
	"pbs_verify_status", "pbs_prune_status", "pbs_version",
	"pbs_backup_files_list", "pbs_task_status", "pbs_task_log",
}

// pveMutationTools is the gated PVE mutation tool surface (SPEC.md §6.x).
var pveMutationTools = []string{
	"pve_vm_create", "pve_vm_resize", "pve_vm_migrate", "pve_ha_add", "pve_vm_backup",
	"pve_vm_start", "pve_vm_stop", "pve_vm_shutdown", "pve_vm_reboot",
	"pve_vm_reset", "pve_vm_suspend", "pve_vm_resume", "pve_vm_delete",
	"pve_vm_clone", "pve_vm_snapshot_create", "pve_vm_snapshot_delete",
	"pve_vm_snapshot_rollback", "pve_vm_restore",
	"pve_lxc_start", "pve_lxc_stop", "pve_lxc_shutdown", "pve_lxc_reboot",
	"pve_lxc_delete", "pve_lxc_clone", "pve_lxc_snapshot_create",
	"pve_lxc_snapshot_delete", "pve_lxc_snapshot_rollback",
}

// pbsMutationTools is the gated PBS mutation tool surface (SPEC.md §6.x).
var pbsMutationTools = []string{
	"pbs_verify_start", "pbs_gc_start", "pbs_prune_start", "pbs_sync_start",
	"pbs_backup_restore_file", "pbs_backup_vm_restore",
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
