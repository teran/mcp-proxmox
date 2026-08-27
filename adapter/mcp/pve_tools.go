package mcp

import (
	"context"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/port"
)

// nodeIn identifies a node (pve_node_status).
type nodeIn struct {
	Node string `json:"node" jsonschema:"PVE node name"`
}

// vmIn identifies a VM on a node (pve_vm_get / pve_vm_status).
type vmIn struct {
	Node string `json:"node" jsonschema:"PVE node name"`
	VMID int    `json:"vmid" jsonschema:"VM ID"`
}

// lxcIn identifies an LXC container on a node (pve_lxc_get / pve_lxc_status).
type lxcIn struct {
	Node string `json:"node" jsonschema:"PVE node name"`
	VMID int    `json:"vmid" jsonschema:"Container ID"`
}

// storageIn identifies a storage on a node (pve_storage_get).
type storageIn struct {
	Node    string `json:"node" jsonschema:"PVE node name"`
	Storage string `json:"storage" jsonschema:"Storage name"`
}

// taskListIn filters pve_task_list.
type taskListIn struct {
	Node       string `json:"node" jsonschema:"PVE node name"`
	VMID       int    `json:"vmid,omitempty" jsonschema:"Filter by VMID"`
	TypeFilter string `json:"typefilter,omitempty" jsonschema:"Filter by task type (e.g. qmstart)"`
	Since      int64  `json:"since,omitempty" jsonschema:"Unix timestamp lower bound (seconds)"`
	Until      int64  `json:"until,omitempty" jsonschema:"Unix timestamp upper bound (seconds)"`
	Source     string `json:"source,omitempty" jsonschema:"Filter by origin node"`
	Limit      int    `json:"limit,omitempty" jsonschema:"Cap the number of returned tasks"`
}

// taskIn identifies a task by UPID (pve_task_status / pve_task_log).
type taskIn struct {
	Node string `json:"node" jsonschema:"PVE node name"`
	UPID string `json:"upid" jsonschema:"Task UPID"`
}

// taskLogIn identifies a task and optionally caps the log length (pve_task_log).
type taskLogIn struct {
	Node  string `json:"node" jsonschema:"PVE node name"`
	UPID  string `json:"upid" jsonschema:"Task UPID"`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum number of log lines"`
}

// registerPVETools registers the Proxmox VE tools. Read-only (query) tools are
// always registered for an enabled PVE backend. Mutation tools are registered
// only when enableMutations is true (gate UX TBD — SPEC.md §2.4); for this
// milestone only the read-only surface is present, pve_vm_migrate being the
// next planned addition.
func registerPVETools(s *mcpSDK.Server, app *application.App, log toolLogger, enableMutations bool) {
	// --- nodes & cluster ---
	roTool(s, "pve_node_list", "List the Proxmox VE cluster nodes.", log, func(ctx context.Context, _ emptyIn) (any, error) {
		return app.PVE.ListNodes(ctx)
	})
	roTool(s, "pve_node_status", "Get the status of a single PVE node.", log, func(ctx context.Context, in nodeIn) (any, error) {
		return app.PVE.GetNodeStatus(ctx, in.Node)
	})
	roTool(s, "pve_cluster_status", "Get the PVE cluster status (quorum/health).", log, func(ctx context.Context, _ emptyIn) (any, error) {
		return app.PVE.GetClusterStatus(ctx)
	})
	roTool(s, "pve_cluster_resources", "Get the PVE cluster resources (VMs/CTs/storage across nodes).", log, func(ctx context.Context, _ emptyIn) (any, error) {
		return app.PVE.GetClusterResources(ctx)
	})
	roTool(s, "pve_nextid", "Get the next free VMID in the PVE cluster.", log, func(ctx context.Context, _ emptyIn) (any, error) {
		return app.PVE.GetNextID(ctx)
	})

	// --- QEMU VMs ---
	roTool(s, "pve_vm_list", "List the QEMU VMs on a PVE node.", log, func(ctx context.Context, in nodeIn) (any, error) {
		return app.PVE.ListVMs(ctx, in.Node)
	})
	roTool(s, "pve_vm_get", "Get the configuration of a QEMU VM.", log, func(ctx context.Context, in vmIn) (any, error) {
		return app.PVE.GetVMConfig(ctx, in.Node, in.VMID)
	})
	roTool(s, "pve_vm_status", "Get the current status of a QEMU VM.", log, func(ctx context.Context, in vmIn) (any, error) {
		return app.PVE.GetVMStatus(ctx, in.Node, in.VMID)
	})

	// --- LXC containers ---
	roTool(s, "pve_lxc_list", "List the LXC containers on a PVE node.", log, func(ctx context.Context, in nodeIn) (any, error) {
		return app.PVE.ListLXCs(ctx, in.Node)
	})
	roTool(s, "pve_lxc_get", "Get the configuration of an LXC container.", log, func(ctx context.Context, in lxcIn) (any, error) {
		return app.PVE.GetLXCConfig(ctx, in.Node, in.VMID)
	})
	roTool(s, "pve_lxc_status", "Get the current status of an LXC container.", log, func(ctx context.Context, in lxcIn) (any, error) {
		return app.PVE.GetLXCStatus(ctx, in.Node, in.VMID)
	})

	// --- storage & network ---
	roTool(s, "pve_storage_list", "List the PVE cluster storage.", log, func(ctx context.Context, _ emptyIn) (any, error) {
		return app.PVE.ListStorage(ctx)
	})
	roTool(s, "pve_storage_get", "Get the status of a storage on a PVE node.", log, func(ctx context.Context, in storageIn) (any, error) {
		return app.PVE.GetStorageStatus(ctx, in.Node, in.Storage)
	})
	roTool(s, "pve_network_list", "List the network interfaces of a PVE node.", log, func(ctx context.Context, in nodeIn) (any, error) {
		return app.PVE.ListNetwork(ctx, in.Node)
	})

	// --- tasks & version ---
	roTool(s, "pve_task_list", "List the tasks on a PVE node.", log, func(ctx context.Context, in taskListIn) (any, error) {
		opts := port.TaskListOptions{
			VMD:        in.VMID,
			TypeFilter: in.TypeFilter,
			Since:      in.Since,
			Until:      in.Until,
			Source:     in.Source,
			Limit:      in.Limit,
		}
		return app.PVE.ListTasks(ctx, in.Node, opts)
	})
	roTool(s, "pve_task_status", "Get the status of a task by UPID.", log, func(ctx context.Context, in taskIn) (any, error) {
		return app.PVE.GetTaskStatus(ctx, in.Node, in.UPID)
	})
	roTool(s, "pve_task_log", "Get the log of a task by UPID.", log, func(ctx context.Context, in taskLogIn) (any, error) {
		return app.PVE.GetTaskLog(ctx, in.Node, in.UPID, in.Limit)
	})
	roTool(s, "pve_version", "Get the Proxmox VE version info.", log, func(ctx context.Context, _ emptyIn) (any, error) {
		return app.PVE.GetPVEVersion(ctx)
	})

	// --- mutations (gated; pve_vm_migrate is the next addition) ---
	if !enableMutations {
		return
	}
	// Mutation tools (pve_vm_create/start/stop/reboot/shutdown/migrate/delete,
	// pve_lxc_create/start/stop/reboot/shutdown/delete) are registered here when
	// the gate is enabled. See SPEC.md §6.
}
