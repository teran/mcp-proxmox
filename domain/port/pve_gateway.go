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
// Proxmox VE REST API (/api2/json) for the read-only (query) surface. Mutation
// methods (start/stop/create/migrate/delete/...) will be added when the
// mutation gate is implemented (SPEC.md §2.4 / §6).
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

	// LXC containers.
	ListLXCs(ctx context.Context, node string) ([]model.LXC, error)
	GetLXCConfig(ctx context.Context, node string, vmid int) (*model.LXCConfig, error)
	GetLXCStatus(ctx context.Context, node string, vmid int) (*model.LXCStatus, error)

	// Storage & network.
	ListStorage(ctx context.Context) ([]model.Storage, error)
	GetStorageStatus(ctx context.Context, node, storage string) (*model.StorageStatus, error)
	ListNetwork(ctx context.Context, node string) ([]model.NetworkInterface, error)

	// Tasks & version.
	ListTasks(ctx context.Context, node string, opts TaskListOptions) ([]model.Task, error)
	GetTaskStatus(ctx context.Context, node, upid string) (*model.TaskStatus, error)
	GetTaskLog(ctx context.Context, node, upid string, limit int) ([]model.TaskLogEntry, error)
	GetPVEVersion(ctx context.Context) (*model.PVEVersion, error)
}
