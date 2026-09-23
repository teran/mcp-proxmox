package port

import (
	"context"

	"github.com/teran/mcp-proxmox/domain/model"
)

// TaskListOptions filters GET /nodes/{node}/tasks. See SPEC.md §6.6.
type TaskListOptions struct {
	VMD        int    `json:"vmid,omitempty"`       // filter by VMID
	TypeFilter string `json:"typefilter,omitempty"` // filter by task type (e.g. qmstart)
	Since      int64  `json:"since,omitempty"`      // unix timestamp (seconds) lower bound
	Until      int64  `json:"until,omitempty"`      // unix timestamp (seconds) upper bound
	Source     string `json:"source,omitempty"`     // filter by origin node
	Limit      int    `json:"limit,omitempty"`      // cap the number of returned tasks
}

// PVEGateway is the secondary port implemented by adapter/pve. It models the
// Proxmox VE REST API (/api2/json): the read-only (query) surface plus the
// mutation surface (create/resize/migrate/ha/backup), which the MCP adapter
// registers only when the ENABLE_MUTATIONS gate is on (SPEC.md §2.5).
type PVEGateway interface {
	// Nodes & cluster.
	ListNodes(ctx context.Context) ([]model.Node, error)
	GetNodeStatus(ctx context.Context, node string) (*model.NodeStatus, error)
	GetClusterStatus(ctx context.Context) ([]model.ClusterStatus, error)
	GetClusterResources(ctx context.Context) ([]model.ClusterResource, error)
	GetNextID(ctx context.Context) (string, error)

	// QEMU VMs.
	ListVMs(ctx context.Context, node string) ([]model.VM, error)
	GetVMConfig(ctx context.Context, node string, vmid int) (*model.VMConfig, error)
	GetVMStatus(ctx context.Context, node string, vmid int) (*model.VMStatus, error)
	ListVMSnapshots(ctx context.Context, node string, vmid int) ([]model.Snapshot, error)

	// LXC containers.
	ListLXCs(ctx context.Context, node string) ([]model.LXC, error)
	GetLXCConfig(ctx context.Context, node string, vmid int) (*model.LXCConfig, error)
	GetLXCStatus(ctx context.Context, node string, vmid int) (*model.LXCStatus, error)
	ListLXCSnapshots(ctx context.Context, node string, vmid int) ([]model.Snapshot, error)

	// LXC lifecycle. These are mutations (gated by EnableMutations) and are never
	// retried. Each runs as a PVE task, so they return a Task whose UPID can be
	// polled with GetTaskStatus / GetTaskLog.
	StartLXC(ctx context.Context, node string, vmid int) (*model.Task, error)
	StopLXC(ctx context.Context, node string, vmid int, skiplock, forceStop bool) (*model.Task, error)
	ShutdownLXC(ctx context.Context, node string, vmid int, forceStop bool, timeout int) (*model.Task, error)
	RebootLXC(ctx context.Context, node string, vmid int, timeout int) (*model.Task, error)
	DeleteLXC(ctx context.Context, node string, vmid int, purge, destroyUnreferencedDisks, force bool) (*model.Task, error)
	CloneLXC(ctx context.Context, node string, vmid int, req model.CloneLXCRequest) (*model.Task, error)
	CreateLXCSnapshot(ctx context.Context, node string, vmid int, req model.SnapshotCreateRequest) (*model.Task, error)
	DeleteLXCSnapshot(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error)
	RollbackLXCSnapshot(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error)

	// Storage & network.
	ListStorage(ctx context.Context) ([]model.Storage, error)
	GetStorageStatus(ctx context.Context, node, storage string) (*model.StorageStatus, error)
	ListNetwork(ctx context.Context, node string) ([]model.NetworkInterface, error)

	// Tasks & version.
	ListTasks(ctx context.Context, node string, opts TaskListOptions) ([]model.Task, error)
	GetTaskStatus(ctx context.Context, node, upid string) (*model.TaskStatus, error)
	GetTaskLog(ctx context.Context, node, upid string, limit int) ([]model.TaskLogEntry, error)
	GetPVEVersion(ctx context.Context) (*model.PVEVersion, error)

	// Mutations (gated by EnableMutations). These are never retried.
	CreateVM(ctx context.Context, node string, req model.CreateVMRequest) (int, error)
	ResizeVM(ctx context.Context, node string, vmid int, req model.ResizeVMRequest) error
	MigrateVM(ctx context.Context, node string, vmid int, req model.MigrateVMRequest) error
	AddHAResource(ctx context.Context, req model.HAResourceRequest) error
	StartVMBackup(ctx context.Context, node string, req model.VMBackupRequest) (*model.Task, error)

	// VM lifecycle. These are mutations (gated by EnableMutations) and are never
	// retried. PVE runs each operation as a task, so they return a Task whose
	// UPID can be polled with GetTaskStatus / GetTaskLog.
	StartVM(ctx context.Context, node string, vmid int) (*model.Task, error)
	StopVM(ctx context.Context, node string, vmid int, skiplock bool) (*model.Task, error)
	ShutdownVM(ctx context.Context, node string, vmid int, forceStop bool, timeout int) (*model.Task, error)
	RebootVM(ctx context.Context, node string, vmid int, timeout int) (*model.Task, error)
	ResetVM(ctx context.Context, node string, vmid int) (*model.Task, error)
	SuspendVM(ctx context.Context, node string, vmid int, todisk bool) (*model.Task, error)
	ResumeVM(ctx context.Context, node string, vmid int) (*model.Task, error)
	DeleteVM(ctx context.Context, node string, vmid int, purge, destroyUnreferencedDisks bool) (*model.Task, error)
	CloneVM(ctx context.Context, node string, vmid int, req model.CloneVMRequest) (*model.Task, error)
	CreateVMSnapshot(ctx context.Context, node string, vmid int, req model.SnapshotCreateRequest) (*model.Task, error)
	DeleteVMSnapshot(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error)
	RollbackVMSnapshot(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error)
}
