package mcp

import (
	"context"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/model"
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

// createVMIn is the input of pve_vm_create (a node plus the QEMU VM config).
type createVMIn struct {
	Node string `json:"node" jsonschema:"PVE node name"`
	model.CreateVMRequest
}

// resizeVMIn is the input of pve_vm_resize.
type resizeVMIn struct {
	Node   string `json:"node" jsonschema:"PVE node name"`
	VMID   int    `json:"vmid" jsonschema:"VM ID"`
	Disk   string `json:"disk" jsonschema:"Virtual disk to resize (e.g. scsi0)"`
	SizeGB int    `json:"size_gb" jsonschema:"New size in GiB; positive grows, negative shrinks"`
}

// migrateVMIn is the input of pve_vm_migrate.
type migrateVMIn struct {
	Node           string `json:"node" jsonschema:"PVE node name"`
	VMID           int    `json:"vmid" jsonschema:"VM ID"`
	Target         string `json:"target" jsonschema:"Target node name"`
	Online         bool   `json:"online,omitempty" jsonschema:"Live migration (no downtime)"`
	WithLocalDisks bool   `json:"with_local_disks,omitempty" jsonschema:"Migrate local disks"`
}

// haAddIn is the input of pve_ha_add.
type haAddIn struct {
	SID     string   `json:"sid" jsonschema:"HA resource ID (e.g. vm:100)"`
	Type    string   `json:"type" jsonschema:"Resource type: vm | ct"`
	Nodes   []string `json:"nodes,omitempty" jsonschema:"Ordered preferred nodes"`
	Comment string   `json:"comment,omitempty" jsonschema:"Optional comment"`
}

// vmBackupIn is the input of pve_vm_backup.
type vmBackupIn struct {
	Node          string `json:"node" jsonschema:"PVE node name"`
	VMID          int    `json:"vmid" jsonschema:"VM ID"`
	Storage       string `json:"storage" jsonschema:"Backup storage"`
	Mode          string `json:"mode" jsonschema:"Backup mode: snapshot | suspend | stop"`
	NotesTemplate string `json:"notes_template,omitempty" jsonschema:"Template string for the backup notes"`
	Compress      string `json:"compress,omitempty" jsonschema:"Compression: 0 | lzo | gzip | zstd"`
}

// registerPVETools registers the Proxmox VE tools. Read-only (query) tools are
// always registered for an enabled PVE backend. Mutation tools are registered
// only when enableMutations is true (SPEC.md §2.5).
func registerPVETools(s *mcpSDK.Server, app *application.App, log toolLogger, enableMutations bool) {
	// --- nodes & cluster ---
	roTool(s, "pve_node_list", "List PVE nodes",
		"List the Proxmox VE cluster nodes.",
		"Returns all cluster nodes and their status. No arguments; read-only and idempotent — safe to call repeatedly.",
		log, func(ctx context.Context, _ emptyIn) (any, error) {
			return app.PVE.ListNodes(ctx)
		})
	roTool(s, "pve_node_status", "Get PVE node status",
		"Get the status of a single PVE node.",
		"Provide the node name. Returns the node's current status; read-only. Obtain valid node names from pve_node_list.",
		log, func(ctx context.Context, in nodeIn) (any, error) {
			return app.PVE.GetNodeStatus(ctx, in.Node)
		})
	roTool(s, "pve_cluster_status", "Get PVE cluster status",
		"Get the PVE cluster status (quorum/health).",
		"No arguments. Returns cluster quorum and health. Read-only; use it to assess cluster-wide availability before mutations.",
		log, func(ctx context.Context, _ emptyIn) (any, error) {
			return app.PVE.GetClusterStatus(ctx)
		})
	roTool(s, "pve_cluster_resources", "Get PVE cluster resources",
		"Get the PVE cluster resources (VMs/CTs/storage across nodes).",
		"No arguments. Returns VMs, containers and storage resources across all nodes. Read-only and idempotent.",
		log, func(ctx context.Context, _ emptyIn) (any, error) {
			return app.PVE.GetClusterResources(ctx)
		})
	roTool(s, "pve_nextid", "Get next PVE VMID",
		"Get the next free VMID in the PVE cluster.",
		"No arguments. Returns the next free VMID, useful before creating a VM/CT. Read-only; the value may change between calls.",
		log, func(ctx context.Context, _ emptyIn) (any, error) {
			return app.PVE.GetNextID(ctx)
		})

	// --- QEMU VMs ---
	roTool(s, "pve_vm_list", "List QEMU VMs",
		"List the QEMU VMs on a PVE node.",
		"Provide the node name. Returns the VMs on that node. Read-only and idempotent.",
		log, func(ctx context.Context, in nodeIn) (any, error) {
			return app.PVE.ListVMs(ctx, in.Node)
		})
	roTool(s, "pve_vm_get", "Get QEMU VM config",
		"Get the configuration of a QEMU VM.",
		"Provide node and vmid. Returns the VM's full configuration. Read-only; does not change any state.",
		log, func(ctx context.Context, in vmIn) (any, error) {
			return app.PVE.GetVMConfig(ctx, in.Node, in.VMID)
		})
	roTool(s, "pve_vm_status", "Get QEMU VM status",
		"Get the current status of a QEMU VM.",
		"Provide node and vmid. Returns the VM's current runtime status. Read-only.",
		log, func(ctx context.Context, in vmIn) (any, error) {
			return app.PVE.GetVMStatus(ctx, in.Node, in.VMID)
		})

	// --- LXC containers ---
	roTool(s, "pve_lxc_list", "List LXC containers",
		"List the LXC containers on a PVE node.",
		"Provide the node name. Returns the containers on that node. Read-only and idempotent.",
		log, func(ctx context.Context, in nodeIn) (any, error) {
			return app.PVE.ListLXCs(ctx, in.Node)
		})
	roTool(s, "pve_lxc_get", "Get LXC config",
		"Get the configuration of an LXC container.",
		"Provide node and vmid. Returns the container's full configuration. Read-only; does not change any state.",
		log, func(ctx context.Context, in lxcIn) (any, error) {
			return app.PVE.GetLXCConfig(ctx, in.Node, in.VMID)
		})
	roTool(s, "pve_lxc_status", "Get LXC status",
		"Get the current status of an LXC container.",
		"Provide node and vmid. Returns the container's current runtime status. Read-only.",
		log, func(ctx context.Context, in lxcIn) (any, error) {
			return app.PVE.GetLXCStatus(ctx, in.Node, in.VMID)
		})

	// --- storage & network ---
	roTool(s, "pve_storage_list", "List PVE storage",
		"List the PVE cluster storage.",
		"No arguments. Returns the configured cluster storage. Read-only and idempotent.",
		log, func(ctx context.Context, _ emptyIn) (any, error) {
			return app.PVE.ListStorage(ctx)
		})
	roTool(s, "pve_storage_get", "Get PVE storage status",
		"Get the status of a storage on a PVE node.",
		"Provide node and storage. Returns the storage's status and usage. Read-only.",
		log, func(ctx context.Context, in storageIn) (any, error) {
			return app.PVE.GetStorageStatus(ctx, in.Node, in.Storage)
		})
	roTool(s, "pve_network_list", "List PVE network interfaces",
		"List the network interfaces of a PVE node.",
		"Provide the node name. Returns the node's network interfaces. Read-only.",
		log, func(ctx context.Context, in nodeIn) (any, error) {
			return app.PVE.ListNetwork(ctx, in.Node)
		})

	// --- tasks & version ---
	roTool(s, "pve_task_list", "List PVE tasks",
		"List the tasks on a PVE node.",
		"Provide the node name. Optionally filter by vmid, typefilter, since/until (unix seconds), source, or limit. Read-only and idempotent.",
		log, func(ctx context.Context, in taskListIn) (any, error) {
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
	roTool(s, "pve_task_status", "Get PVE task status",
		"Get the status of a task by UPID.",
		"Provide node and the task's UPID (from pve_task_list). Returns the task's current status. Read-only.",
		log, func(ctx context.Context, in taskIn) (any, error) {
			return app.PVE.GetTaskStatus(ctx, in.Node, in.UPID)
		})
	roTool(s, "pve_task_log", "Get PVE task log",
		"Get the log of a task by UPID.",
		"Provide node, the task's UPID, and optionally a limit on the number of log lines. Returns the task log. Read-only.",
		log, func(ctx context.Context, in taskLogIn) (any, error) {
			return app.PVE.GetTaskLog(ctx, in.Node, in.UPID, in.Limit)
		})
	roTool(s, "pve_version", "Get PVE version",
		"Get the Proxmox VE version info.",
		"No arguments. Returns the PVE version. Read-only.",
		log, func(ctx context.Context, _ emptyIn) (any, error) {
			return app.PVE.GetPVEVersion(ctx)
		})

	// --- mutations (gated by EnableMutations) ---
	if !enableMutations {
		return
	}

	mutTool(s, "pve_vm_create", "Create QEMU VM",
		"Create a QEMU VM on a PVE node.",
		"Provide the node and the VM configuration (name, cores, memory, disks, network, etc.). Optionally set vmid; otherwise PVE assigns the next free ID. Returns the new VMID. Idempotent: re-issuing the same create may create a second VM, so verify with pve_vm_list.",
		log, true, func(ctx context.Context, in createVMIn) (any, error) {
			vmid, err := app.PVE.CreateVM(ctx, in.Node, in.CreateVMRequest)
			if err != nil {
				return nil, err
			}
			return map[string]any{"vmid": vmid}, nil
		})

	mutTool(s, "pve_vm_resize", "Resize QEMU VM disk",
		"Resize a QEMU VM disk.",
		"Provide node, vmid, the disk to resize (e.g. scsi0) and the new size in GiB. A positive size grows the disk, a negative size shrinks it. Idempotent.",
		log, true, func(ctx context.Context, in resizeVMIn) (any, error) {
			return nil, app.PVE.ResizeVM(ctx, in.Node, in.VMID, model.ResizeVMRequest{
				Disk:   in.Disk,
				SizeGB: in.SizeGB,
			})
		})

	mutTool(s, "pve_vm_migrate", "Migrate QEMU VM",
		"Migrate a QEMU VM to another node.",
		"Provide node, vmid and the target node. Set online for live migration (no downtime) and with_local_disks to move local disks. Idempotent.",
		log, true, func(ctx context.Context, in migrateVMIn) (any, error) {
			return nil, app.PVE.MigrateVM(ctx, in.Node, in.VMID, model.MigrateVMRequest{
				Target:         in.Target,
				Online:         in.Online,
				WithLocalDisks: in.WithLocalDisks,
			})
		})

	mutTool(s, "pve_ha_add", "Add HA resource",
		"Register a new high-availability resource.",
		"Provide the resource SID (e.g. vm:100), type (vm or ct), optionally an ordered list of preferred nodes and a comment. Idempotent.",
		log, true, func(ctx context.Context, in haAddIn) (any, error) {
			return nil, app.PVE.AddHAResource(ctx, model.HAResourceRequest{
				SID:     in.SID,
				Type:    in.Type,
				Nodes:   in.Nodes,
				Comment: in.Comment,
			})
		})

	mutTool(s, "pve_vm_backup", "Back up QEMU VM",
		"Start a vzdump backup of a QEMU VM.",
		"Provide node, vmid, the backup storage and mode (snapshot|suspend|stop). Optionally set a notes template and compression. Returns the task UPID to poll with pve_task_status. Not idempotent: each call starts a new backup.",
		log, false, func(ctx context.Context, in vmBackupIn) (any, error) {
			return app.PVE.StartVMBackup(ctx, in.Node, model.VMBackupRequest{
				VMID:          in.VMID,
				Storage:       in.Storage,
				Mode:          in.Mode,
				NotesTemplate: in.NotesTemplate,
				Compress:      in.Compress,
			})
		})
}
