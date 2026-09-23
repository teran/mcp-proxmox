package application

import (
	"context"

	"github.com/teran/mcp-proxmox/domain/model"
	"github.com/teran/mcp-proxmox/domain/port"
)

// PVEService is the PVE use cases. It is a thin orchestration layer over
// port.PVEGateway; it exists so adapter/mcp talks to the application layer
// rather than the adapter directly, and so QA can test it against a PVEGateway
// mock. See SPEC.md §3.1 / §6.
type PVEService struct {
	gw  port.PVEGateway
	log port.CtxLogger
}

// ListNodes returns the PVE cluster nodes.
func (s *PVEService) ListNodes(ctx context.Context) ([]model.Node, error) {
	return s.gw.ListNodes(ctx)
}

// GetNodeStatus returns the status of a single node.
func (s *PVEService) GetNodeStatus(ctx context.Context, node string) (*model.NodeStatus, error) {
	return s.gw.GetNodeStatus(ctx, node)
}

// GetClusterStatus returns the cluster status (quorum/health).
func (s *PVEService) GetClusterStatus(ctx context.Context) ([]model.ClusterStatus, error) {
	return s.gw.GetClusterStatus(ctx)
}

// GetClusterResources returns the cluster resources (VMs/CTs/storage).
func (s *PVEService) GetClusterResources(ctx context.Context) ([]model.ClusterResource, error) {
	return s.gw.GetClusterResources(ctx)
}

// GetNextID returns the next free VMID.
func (s *PVEService) GetNextID(ctx context.Context) (string, error) {
	return s.gw.GetNextID(ctx)
}

// ListVMs returns the QEMU VMs on a node.
func (s *PVEService) ListVMs(ctx context.Context, node string) ([]model.VM, error) {
	return s.gw.ListVMs(ctx, node)
}

// GetVMConfig returns the config of a VM.
func (s *PVEService) GetVMConfig(ctx context.Context, node string, vmid int) (*model.VMConfig, error) {
	return s.gw.GetVMConfig(ctx, node, vmid)
}

// GetVMStatus returns the status of a VM.
func (s *PVEService) GetVMStatus(ctx context.Context, node string, vmid int) (*model.VMStatus, error) {
	return s.gw.GetVMStatus(ctx, node, vmid)
}

// ListVMSnapshots lists the snapshots of a QEMU VM on a node.
func (s *PVEService) ListVMSnapshots(ctx context.Context, node string, vmid int) ([]model.Snapshot, error) {
	return s.gw.ListVMSnapshots(ctx, node, vmid)
}

// ListLXCs returns the LXC containers on a node.
func (s *PVEService) ListLXCs(ctx context.Context, node string) ([]model.LXC, error) {
	return s.gw.ListLXCs(ctx, node)
}

// GetLXCConfig returns the config of a container.
func (s *PVEService) GetLXCConfig(ctx context.Context, node string, vmid int) (*model.LXCConfig, error) {
	return s.gw.GetLXCConfig(ctx, node, vmid)
}

// GetLXCStatus returns the status of a container.
func (s *PVEService) GetLXCStatus(ctx context.Context, node string, vmid int) (*model.LXCStatus, error) {
	return s.gw.GetLXCStatus(ctx, node, vmid)
}

// ListStorage returns the cluster storage.
func (s *PVEService) ListStorage(ctx context.Context) ([]model.Storage, error) {
	return s.gw.ListStorage(ctx)
}

// GetStorageStatus returns the status of a storage on a node.
func (s *PVEService) GetStorageStatus(ctx context.Context, node, storage string) (*model.StorageStatus, error) {
	return s.gw.GetStorageStatus(ctx, node, storage)
}

// ListNetwork returns the network interfaces of a node.
func (s *PVEService) ListNetwork(ctx context.Context, node string) ([]model.NetworkInterface, error) {
	return s.gw.ListNetwork(ctx, node)
}

// ListTasks returns the tasks on a node, filtered by opts.
func (s *PVEService) ListTasks(ctx context.Context, node string, opts port.TaskListOptions) ([]model.Task, error) {
	return s.gw.ListTasks(ctx, node, opts)
}

// GetTaskStatus returns the status of a task by UPID.
func (s *PVEService) GetTaskStatus(ctx context.Context, node, upid string) (*model.TaskStatus, error) {
	return s.gw.GetTaskStatus(ctx, node, upid)
}

// GetTaskLog returns the log of a task by UPID.
func (s *PVEService) GetTaskLog(ctx context.Context, node, upid string, limit int) ([]model.TaskLogEntry, error) {
	return s.gw.GetTaskLog(ctx, node, upid, limit)
}

// GetPVEVersion returns the PVE version info.
func (s *PVEService) GetPVEVersion(ctx context.Context) (*model.PVEVersion, error) {
	return s.gw.GetPVEVersion(ctx)
}

// CreateVM creates a QEMU VM on a node and returns its new VMID.
func (s *PVEService) CreateVM(ctx context.Context, node string, req model.CreateVMRequest) (int, error) {
	return s.gw.CreateVM(ctx, node, req)
}

// ResizeVM resizes a VM disk on a node.
func (s *PVEService) ResizeVM(ctx context.Context, node string, vmid int, req model.ResizeVMRequest) error {
	return s.gw.ResizeVM(ctx, node, vmid, req)
}

// MigrateVM migrates a VM to another node.
func (s *PVEService) MigrateVM(ctx context.Context, node string, vmid int, req model.MigrateVMRequest) error {
	return s.gw.MigrateVM(ctx, node, vmid, req)
}

// AddHAResource registers a new HA resource in the cluster.
func (s *PVEService) AddHAResource(ctx context.Context, req model.HAResourceRequest) error {
	return s.gw.AddHAResource(ctx, req)
}

// StartVMBackup starts a vzdump backup of a VM on a node.
func (s *PVEService) StartVMBackup(ctx context.Context, node string, req model.VMBackupRequest) (*model.Task, error) {
	return s.gw.StartVMBackup(ctx, node, req)
}

// StartVM starts a QEMU VM on a node.
func (s *PVEService) StartVM(ctx context.Context, node string, vmid int) (*model.Task, error) {
	return s.gw.StartVM(ctx, node, vmid)
}

// StopVM stops a QEMU VM on a node (hard power off).
func (s *PVEService) StopVM(ctx context.Context, node string, vmid int, skiplock bool) (*model.Task, error) {
	return s.gw.StopVM(ctx, node, vmid, skiplock)
}

// ShutdownVM gracefully shuts down a QEMU VM on a node.
func (s *PVEService) ShutdownVM(ctx context.Context, node string, vmid int, forceStop bool, timeout int) (*model.Task, error) {
	return s.gw.ShutdownVM(ctx, node, vmid, forceStop, timeout)
}

// RebootVM reboots a QEMU VM on a node.
func (s *PVEService) RebootVM(ctx context.Context, node string, vmid int, timeout int) (*model.Task, error) {
	return s.gw.RebootVM(ctx, node, vmid, timeout)
}

// ResetVM resets a QEMU VM on a node.
func (s *PVEService) ResetVM(ctx context.Context, node string, vmid int) (*model.Task, error) {
	return s.gw.ResetVM(ctx, node, vmid)
}

// SuspendVM suspends a QEMU VM on a node.
func (s *PVEService) SuspendVM(ctx context.Context, node string, vmid int, todisk bool) (*model.Task, error) {
	return s.gw.SuspendVM(ctx, node, vmid, todisk)
}

// ResumeVM resumes a suspended QEMU VM on a node.
func (s *PVEService) ResumeVM(ctx context.Context, node string, vmid int) (*model.Task, error) {
	return s.gw.ResumeVM(ctx, node, vmid)
}

// DeleteVM deletes a QEMU VM on a node.
func (s *PVEService) DeleteVM(ctx context.Context, node string, vmid int, purge, destroyUnreferencedDisks bool) (*model.Task, error) {
	return s.gw.DeleteVM(ctx, node, vmid, purge, destroyUnreferencedDisks)
}

// CloneVM clones a QEMU VM on a node.
func (s *PVEService) CloneVM(ctx context.Context, node string, vmid int, req model.CloneVMRequest) (*model.Task, error) {
	return s.gw.CloneVM(ctx, node, vmid, req)
}

// CreateVMSnapshot creates a snapshot of a QEMU VM on a node.
func (s *PVEService) CreateVMSnapshot(ctx context.Context, node string, vmid int, req model.SnapshotCreateRequest) (*model.Task, error) {
	return s.gw.CreateVMSnapshot(ctx, node, vmid, req)
}

// DeleteVMSnapshot deletes a snapshot of a QEMU VM on a node.
func (s *PVEService) DeleteVMSnapshot(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
	return s.gw.DeleteVMSnapshot(ctx, node, vmid, snapname)
}

// RollbackVMSnapshot rolls a QEMU VM back to a snapshot on a node.
func (s *PVEService) RollbackVMSnapshot(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
	return s.gw.RollbackVMSnapshot(ctx, node, vmid, snapname)
}
