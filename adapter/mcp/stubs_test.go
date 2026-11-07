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
	removeHAResource func(ctx context.Context, sid string) error
	startVMBackup    func(ctx context.Context, node string, req model.VMBackupRequest) (*model.Task, error)
	createBackupJob  func(ctx context.Context, req model.CreateBackupJobRequest) (string, error)
	updateBackupJob  func(ctx context.Context, id string, req model.CreateBackupJobRequest) (string, error)
	deleteBackupJob  func(ctx context.Context, id string) error
	setVMConfig      func(ctx context.Context, node string, vmid int, req model.SetVMConfigRequest) error
	setLXCConfig     func(ctx context.Context, node string, vmid int, req model.SetLXCConfigRequest) error

	listVMSnapshots    func(ctx context.Context, node string, vmid int) ([]model.Snapshot, error)
	listLXCSnapshots   func(ctx context.Context, node string, vmid int) ([]model.Snapshot, error)
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
	startLXC           func(ctx context.Context, node string, vmid int) (*model.Task, error)
	stopLXC            func(ctx context.Context, node string, vmid int, skiplock, forceStop bool) (*model.Task, error)
	shutdownLXC        func(ctx context.Context, node string, vmid int, forceStop bool, timeout int) (*model.Task, error)
	rebootLXC          func(ctx context.Context, node string, vmid int, timeout int) (*model.Task, error)
	deleteLXC          func(
		ctx context.Context, node string, vmid int, purge, destroyUnreferencedDisks, force bool,
	) (*model.Task, error)
	cloneLXC            func(ctx context.Context, node string, vmid int, req model.CloneLXCRequest) (*model.Task, error)
	createLXCSnapshot   func(ctx context.Context, node string, vmid int, req model.SnapshotCreateRequest) (*model.Task, error)
	deleteLXCSnapshot   func(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error)
	rollbackLXCSnapshot func(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error)
	restoreVM           func(ctx context.Context, node string, req model.PVERestoreRequest) (*model.Task, error)

	// LXC create + storage content (feat/lxc-templates).
	createLXC          func(ctx context.Context, node string, req model.CreateLXCRequest) (int, error)
	listStorageContent func(
		ctx context.Context, node, storage string, opts port.StorageContentListOptions,
	) ([]model.StorageContent, error)
	downloadURL func(ctx context.Context, node, storage string, req model.StorageDownloadURLRequest) (string, error)
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

	return &model.NodeStatus{PVEVersion: "pve-manager/9.2.10"}, nil
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

	return []model.BackupJob{{ID: "backup-100", Enabled: model.FlexBool(true), Schedule: "daily", Storage: "local"}}, nil
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

func (s *stubPVEGateway) RemoveHAResource(ctx context.Context, sid string) error {
	if s.fail() {
		return s.err
	}
	if s.removeHAResource != nil {
		return s.removeHAResource(ctx, sid)
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

func (s *stubPVEGateway) CreateBackupJob(ctx context.Context, req model.CreateBackupJobRequest) (string, error) {
	if s.fail() {
		return "", s.err
	}
	if s.createBackupJob != nil {
		return s.createBackupJob(ctx, req)
	}

	return "backup-100", nil
}

func (s *stubPVEGateway) UpdateBackupJob(ctx context.Context, id string, req model.CreateBackupJobRequest) (string, error) {
	if s.fail() {
		return "", s.err
	}
	if s.updateBackupJob != nil {
		return s.updateBackupJob(ctx, id, req)
	}

	return "backup-100", nil
}

func (s *stubPVEGateway) DeleteBackupJob(ctx context.Context, id string) error {
	if s.fail() {
		return s.err
	}
	if s.deleteBackupJob != nil {
		return s.deleteBackupJob(ctx, id)
	}

	return nil
}

func (s *stubPVEGateway) SetVMConfig(ctx context.Context, node string, vmid int, req model.SetVMConfigRequest) error {
	if s.fail() {
		return s.err
	}
	if s.setVMConfig != nil {
		return s.setVMConfig(ctx, node, vmid, req)
	}

	return nil
}

func (s *stubPVEGateway) SetLXCConfig(ctx context.Context, node string, vmid int, req model.SetLXCConfigRequest) error {
	if s.fail() {
		return s.err
	}
	if s.setLXCConfig != nil {
		return s.setLXCConfig(ctx, node, vmid, req)
	}

	return nil
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

func (s *stubPVEGateway) ShutdownVM(
	ctx context.Context, node string, vmid int, forceStop bool, timeout int,
) (*model.Task, error) {
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

func (s *stubPVEGateway) DeleteVM(
	ctx context.Context, node string, vmid int, purge, destroyUnreferencedDisks bool,
) (*model.Task, error) {
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

func (s *stubPVEGateway) CreateVMSnapshot(
	ctx context.Context, node string, vmid int, req model.SnapshotCreateRequest,
) (*model.Task, error) {
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

func (s *stubPVEGateway) ShutdownLXC(
	ctx context.Context, node string, vmid int, forceStop bool, timeout int,
) (*model.Task, error) {
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

func (s *stubPVEGateway) DeleteLXC(
	ctx context.Context, node string, vmid int, purge, destroyUnreferencedDisks, force bool,
) (*model.Task, error) {
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

func (s *stubPVEGateway) CreateLXCSnapshot(
	ctx context.Context, node string, vmid int, req model.SnapshotCreateRequest,
) (*model.Task, error) {
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

func (s *stubPVEGateway) CreateLXC(ctx context.Context, node string, req model.CreateLXCRequest) (int, error) {
	if s.fail() {
		return 0, s.err
	}
	if s.createLXC != nil {
		return s.createLXC(ctx, node, req)
	}

	return 203, nil
}

func (s *stubPVEGateway) ListStorageContent(
	ctx context.Context, node, storage string, opts port.StorageContentListOptions,
) ([]model.StorageContent, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.listStorageContent != nil {
		return s.listStorageContent(ctx, node, storage, opts)
	}

	return []model.StorageContent{{VolID: "local:iso/debian-12.iso", Content: "iso", Format: "iso", Size: 513802240}}, nil
}

func (s *stubPVEGateway) DownloadURL(
	ctx context.Context, node, storage string, req model.StorageDownloadURLRequest,
) (string, error) {
	if s.fail() {
		return "", s.err
	}
	if s.downloadURL != nil {
		return s.downloadURL(ctx, node, storage, req)
	}

	return "UPID:pve:download-url:...", nil
}

var _ port.PVEGateway = (*stubPVEGateway)(nil)

// stubPBSGateway is a controllable port.PBSGateway stub for adapter/mcp tests.
type stubPBSGateway struct {
	err error

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

	listBackupFiles func(
		ctx context.Context, store, backupType, backupID string, backupTime int64, path string,
	) ([]model.PBSFile, error)
	getTaskStatus func(ctx context.Context, upid string) (*model.PBSTask, error)
	getTaskLog    func(ctx context.Context, upid string, limit int) ([]model.TaskLogEntry, error)
	stopTask      func(ctx context.Context, upid string) error
	forgetBackup  func(ctx context.Context, store, backupType, backupID string, backupTime int64) error
	restoreFile   func(
		ctx context.Context, store, backupType, backupID string, backupTime int64, req model.PBSFileRestoreRequest,
	) error
	restoreVMBackup func(
		ctx context.Context, store, backupType, backupID string, backupTime int64, req model.PBSVMRestoreRequest,
	) (string, error)
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

func (s *stubPBSGateway) ListBackups(ctx context.Context, store string, opts port.PBSBackupListOptions) ([]model.Backup, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.listBackups != nil {
		return s.listBackups(ctx, store, opts)
	}

	return []model.Backup{{BackupID: "vm/100/..."}}, nil
}

func (s *stubPBSGateway) GetBackup(
	ctx context.Context, store, backupID string, opts port.PBSBackupListOptions,
) ([]model.Backup, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getBackup != nil {
		return s.getBackup(ctx, store, backupID, opts)
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

func (s *stubPBSGateway) GetVerifyStatus(ctx context.Context, upid string) (*model.PBSTask, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getVerifyStatus != nil {
		return s.getVerifyStatus(ctx, upid)
	}

	return &model.PBSTask{UPID: upid, Status: "ok"}, nil
}

func (s *stubPBSGateway) GetPruneStatus(ctx context.Context, upid string) (*model.PBSTask, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getPruneStatus != nil {
		return s.getPruneStatus(ctx, upid)
	}

	return &model.PBSTask{UPID: upid, Status: "running"}, nil
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

func (s *stubPBSGateway) StopTask(ctx context.Context, upid string) error {
	if s.fail() {
		return s.err
	}
	if s.stopTask != nil {
		return s.stopTask(ctx, upid)
	}

	return nil
}

func (s *stubPBSGateway) ForgetBackup(
	ctx context.Context, store, backupType, backupID string, backupTime int64,
) error {
	if s.fail() {
		return s.err
	}
	if s.forgetBackup != nil {
		return s.forgetBackup(ctx, store, backupType, backupID, backupTime)
	}

	return nil
}

func (s *stubPBSGateway) ListBackupFiles(
	ctx context.Context, store, backupType, backupID string, backupTime int64, path string,
) ([]model.PBSFile, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.listBackupFiles != nil {
		return s.listBackupFiles(ctx, store, backupType, backupID, backupTime, path)
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

func (s *stubPBSGateway) RestoreFile(
	ctx context.Context, store, backupType, backupID string, backupTime int64, req model.PBSFileRestoreRequest,
) error {
	if s.fail() {
		return s.err
	}
	if s.restoreFile != nil {
		return s.restoreFile(ctx, store, backupType, backupID, backupTime, req)
	}

	return nil
}

func (s *stubPBSGateway) RestoreVMBackup(
	ctx context.Context, store, backupType, backupID string, backupTime int64, req model.PBSVMRestoreRequest,
) (string, error) {
	if s.fail() {
		return "", s.err
	}
	if s.restoreVMBackup != nil {
		return s.restoreVMBackup(ctx, store, backupType, backupID, backupTime, req)
	}

	return "UPID:pbs:restore:...", nil
}

var _ port.PBSGateway = (*stubPBSGateway)(nil)

// stubCephGateway is a controllable port.CephGateway stub for adapter/mcp
// tests. Every method defaults to a canned success value; a test can override
// any method (or set err) to force an error.
type stubCephGateway struct {
	err error

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

func (s *stubCephGateway) fail() bool { return s.err != nil }

func (s *stubCephGateway) GetClusterStatus(ctx context.Context) (*model.CephClusterStatus, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getClusterStatus != nil {
		return s.getClusterStatus(ctx)
	}

	return &model.CephClusterStatus{Status: "HEALTH_OK", Quorum: []int{1}}, nil
}

func (s *stubCephGateway) GetClusterConfig(ctx context.Context) ([]model.CephConfigEntry, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getClusterConfig != nil {
		return s.getClusterConfig(ctx)
	}

	return []model.CephConfigEntry{{Name: "mon_allow_pool_delete", Value: "true"}}, nil
}

func (s *stubCephGateway) GetClusterLog(ctx context.Context, limit int) ([]model.CephLogEntry, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getClusterLog != nil {
		return s.getClusterLog(ctx, limit)
	}

	return []model.CephLogEntry{{Message: "boot"}}, nil
}

func (s *stubCephGateway) GetClusterMetadata(ctx context.Context) (*model.CephClusterMetadata, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getClusterMeta != nil {
		return s.getClusterMeta(ctx)
	}

	return &model.CephClusterMetadata{Fsid: "abc", CephVersion: "17.2.7"}, nil
}

func (s *stubCephGateway) GetNodeStatus(ctx context.Context, node string) (*model.CephNodeStatus, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getNodeStatus != nil {
		return s.getNodeStatus(ctx, node)
	}

	return &model.CephNodeStatus{Status: "HEALTH_WARN"}, nil
}

func (s *stubCephGateway) GetNodeConfig(ctx context.Context, node string) ([]model.CephConfigEntry, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getNodeConfig != nil {
		return s.getNodeConfig(ctx, node)
	}

	return []model.CephConfigEntry{{Name: "n", Value: "v"}}, nil
}

func (s *stubCephGateway) GetNodeConfigDB(ctx context.Context, node string) ([]model.CephConfigEntry, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getNodeConfigDB != nil {
		return s.getNodeConfigDB(ctx, node)
	}

	return []model.CephConfigEntry{{Name: "db", Value: "1"}}, nil
}

func (s *stubCephGateway) GetNodeFlags(ctx context.Context, node string) ([]model.CephFlag, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getNodeFlags != nil {
		return s.getNodeFlags(ctx, node)
	}

	return []model.CephFlag{{ID: 1, Name: "noout", Value: "0", Enabled: 0}}, nil
}

func (s *stubCephGateway) GetNodeMetadata(ctx context.Context, node string) (*model.CephNodeMetadata, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getNodeMeta != nil {
		return s.getNodeMeta(ctx, node)
	}

	return &model.CephNodeMetadata{}, nil
}

func (s *stubCephGateway) GetNodeCrush(ctx context.Context, node string) (*model.CephCrushMap, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getNodeCrush != nil {
		return s.getNodeCrush(ctx, node)
	}

	return &model.CephCrushMap{Rules: []model.CephCrushRule{{ID: 0, RuleName: "replicated_rule"}}}, nil
}

func (s *stubCephGateway) GetNodeRules(ctx context.Context, node string) ([]model.CephCrushRule, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.getNodeRules != nil {
		return s.getNodeRules(ctx, node)
	}

	return []model.CephCrushRule{{ID: 0, RuleName: "replicated_rule"}}, nil
}

func (s *stubCephGateway) ListPools(ctx context.Context, node string) ([]model.CephPool, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.listPools != nil {
		return s.listPools(ctx, node)
	}

	return []model.CephPool{{ID: 1, PoolName: "rbd", Type: "replicated"}}, nil
}

func (s *stubCephGateway) ListOSDs(ctx context.Context, node string) ([]model.CephOSD, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.listOSDs != nil {
		return s.listOSDs(ctx, node)
	}

	return []model.CephOSD{{ID: 0, Name: "osd.0", Status: "up"}}, nil
}

func (s *stubCephGateway) ListMons(ctx context.Context, node string) ([]model.CephMon, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.listMons != nil {
		return s.listMons(ctx, node)
	}

	return []model.CephMon{{ID: 1, Name: "pve1"}}, nil
}

func (s *stubCephGateway) ListMgrs(ctx context.Context, node string) ([]model.CephMgr, error) {
	if s.fail() {
		return nil, s.err
	}
	if s.listMgrs != nil {
		return s.listMgrs(ctx, node)
	}

	return []model.CephMgr{{ID: 1, Name: "pve1", Status: "active"}}, nil
}

func (s *stubCephGateway) CreateCrushRule(
	ctx context.Context, node string, req model.CephCrushRuleCreateRequest,
) (string, error) {
	if s.fail() {
		return "", s.err
	}
	if s.createCrushRule != nil {
		return s.createCrushRule(ctx, node, req)
	}

	return "replicated_rule", nil
}

func (s *stubCephGateway) DeleteCrushRule(ctx context.Context, node, name string) error {
	if s.fail() {
		return s.err
	}
	if s.deleteCrushRule != nil {
		return s.deleteCrushRule(ctx, node, name)
	}

	return nil
}

func (s *stubCephGateway) CreatePool(ctx context.Context, node string, req model.CephPoolCreateRequest) (string, error) {
	if s.fail() {
		return "", s.err
	}
	if s.createPool != nil {
		return s.createPool(ctx, node, req)
	}

	return "pool1", nil
}

func (s *stubCephGateway) UpdatePool(
	ctx context.Context, node, name string, req model.CephPoolUpdateRequest,
) error {
	if s.fail() {
		return s.err
	}
	if s.updatePool != nil {
		return s.updatePool(ctx, node, name, req)
	}

	return nil
}

func (s *stubCephGateway) DeletePool(ctx context.Context, node, name string) error {
	if s.fail() {
		return s.err
	}
	if s.deletePool != nil {
		return s.deletePool(ctx, node, name)
	}

	return nil
}

func (s *stubCephGateway) CreateOSD(ctx context.Context, node string, req model.CephOSDCreateRequest) (string, error) {
	if s.fail() {
		return "", s.err
	}
	if s.createOSD != nil {
		return s.createOSD(ctx, node, req)
	}

	return "osd.0", nil
}

func (s *stubCephGateway) DeleteOSD(ctx context.Context, node string, osdid int) error {
	if s.fail() {
		return s.err
	}
	if s.deleteOSD != nil {
		return s.deleteOSD(ctx, node, osdid)
	}

	return nil
}

func (s *stubCephGateway) CreateMon(ctx context.Context, node string, req model.CephMonCreateRequest) (string, error) {
	if s.fail() {
		return "", s.err
	}
	if s.createMon != nil {
		return s.createMon(ctx, node, req)
	}

	return "pve2", nil
}

func (s *stubCephGateway) DeleteMon(ctx context.Context, node, monid string) error {
	if s.fail() {
		return s.err
	}
	if s.deleteMon != nil {
		return s.deleteMon(ctx, node, monid)
	}

	return nil
}

func (s *stubCephGateway) CreateMgr(ctx context.Context, node string, req model.CephMgrCreateRequest) (string, error) {
	if s.fail() {
		return "", s.err
	}
	if s.createMgr != nil {
		return s.createMgr(ctx, node, req)
	}

	return "pve2", nil
}

func (s *stubCephGateway) SetFlags(ctx context.Context, node string, req model.CephFlagsSetRequest) error {
	if s.fail() {
		return s.err
	}
	if s.setFlags != nil {
		return s.setFlags(ctx, node, req)
	}

	return nil
}

func (s *stubCephGateway) SetConfig(ctx context.Context, node string, req model.CephConfigSetRequest) error {
	if s.fail() {
		return s.err
	}
	if s.setConfig != nil {
		return s.setConfig(ctx, node, req)
	}

	return nil
}

func (s *stubCephGateway) InitCeph(ctx context.Context, node string, req model.CephInitRequest) (string, error) {
	if s.fail() {
		return "", s.err
	}
	if s.initCeph != nil {
		return s.initCeph(ctx, node, req)
	}

	return "UPID:ceph:init:...", nil
}

func (s *stubCephGateway) DestroyCeph(ctx context.Context, node string, req model.CephDestroyRequest) error {
	if s.fail() {
		return s.err
	}
	if s.destroyCeph != nil {
		return s.destroyCeph(ctx, node, req)
	}

	return nil
}

var _ port.CephGateway = (*stubCephGateway)(nil)

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
	"pve_storage_content_list",
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
	"pve_vm_create", "pve_vm_resize", "pve_vm_migrate", "pve_ha_add", "pve_ha_remove", "pve_vm_backup",
	"pve_vm_start", "pve_vm_stop", "pve_vm_shutdown", "pve_vm_reboot",
	"pve_vm_reset", "pve_vm_suspend", "pve_vm_resume", "pve_vm_delete",
	"pve_vm_clone", "pve_vm_snapshot_create", "pve_vm_snapshot_delete",
	"pve_vm_snapshot_rollback", "pve_vm_restore", "pve_vm_set",
	"pve_lxc_start", "pve_lxc_stop", "pve_lxc_shutdown", "pve_lxc_reboot",
	"pve_lxc_delete", "pve_lxc_clone", "pve_lxc_snapshot_create",
	"pve_lxc_snapshot_delete", "pve_lxc_snapshot_rollback", "pve_lxc_set",
	"pve_lxc_create", "pve_storage_download_url",
	"pve_backup_job_create", "pve_backup_job_update", "pve_backup_job_delete",
}

// pbsMutationTools is the gated PBS mutation tool surface (SPEC.md §6.x).
var pbsMutationTools = []string{
	"pbs_verify_start", "pbs_gc_start", "pbs_prune_start", "pbs_sync_start",
	"pbs_backup_restore_file", "pbs_backup_vm_restore", "pbs_task_stop",
	"pbs_backup_forget",
}

// cephROTools is the read-only Ceph tool surface (SPEC.md §6.x).
var cephROTools = []string{
	"ceph_cluster_status", "ceph_cluster_config", "ceph_cluster_log", "ceph_cluster_metadata",
	"ceph_node_status", "ceph_node_config", "ceph_node_configdb", "ceph_node_flags",
	"ceph_node_metadata", "ceph_node_crush", "ceph_node_rules",
	"ceph_pool_list", "ceph_osd_list", "ceph_mon_list", "ceph_mgr_list",
}

// cephMutationTools is the gated Ceph mutation tool surface (SPEC.md §6.x).
var cephMutationTools = []string{
	"ceph_rule_create", "ceph_rule_delete",
	"ceph_pool_create", "ceph_pool_update", "ceph_pool_delete",
	"ceph_osd_create", "ceph_osd_delete",
	"ceph_mon_create", "ceph_mon_delete",
	"ceph_mgr_create",
	"ceph_flags_set", "ceph_config_set",
	"ceph_init", "ceph_destroy",
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
