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

// vmStopIn is the input of pve_vm_stop.
type vmStopIn struct {
	Node     string `json:"node" jsonschema:"PVE node name"`
	VMID     int    `json:"vmid" jsonschema:"VM ID"`
	SkipLock bool   `json:"skiplock,omitempty" jsonschema:"Ignore locks and stop anyway"`
}

// vmShutdownIn is the input of pve_vm_shutdown.
type vmShutdownIn struct {
	Node      string `json:"node" jsonschema:"PVE node name"`
	VMID      int    `json:"vmid" jsonschema:"VM ID"`
	ForceStop bool   `json:"force_stop,omitempty" jsonschema:"Immediately stop if the graceful shutdown times out"`
	Timeout   int    `json:"timeout,omitempty" jsonschema:"Shutdown timeout in seconds"`
}

// vmRebootIn is the input of pve_vm_reboot.
type vmRebootIn struct {
	Node    string `json:"node" jsonschema:"PVE node name"`
	VMID    int    `json:"vmid" jsonschema:"VM ID"`
	Timeout int    `json:"timeout,omitempty" jsonschema:"Reboot timeout in seconds"`
}

// vmSuspendIn is the input of pve_vm_suspend.
type vmSuspendIn struct {
	Node   string `json:"node" jsonschema:"PVE node name"`
	VMID   int    `json:"vmid" jsonschema:"VM ID"`
	ToDisk bool   `json:"to_disk,omitempty" jsonschema:"Suspend to disk (write guest RAM to disk)"`
}

// vmDeleteIn is the input of pve_vm_delete.
type vmDeleteIn struct {
	Node                     string `json:"node" jsonschema:"PVE node name"`
	VMID                     int    `json:"vmid" jsonschema:"VM ID"`
	Purge                    bool   `json:"purge,omitempty" jsonschema:"Remove the VM from any backup jobs"`
	DestroyUnreferencedDisks bool   `json:"destroy_unreferenced_disks,omitempty" jsonschema:"Remove disks not referenced by the VM config"`
}

// vmCloneIn is the input of pve_vm_clone.
type vmCloneIn struct {
	Node        string `json:"node" jsonschema:"PVE node name"`
	VMID        int    `json:"vmid" jsonschema:"VM ID"`
	NewID       int    `json:"newid" jsonschema:"VM ID of the new clone"`
	Name        string `json:"name,omitempty" jsonschema:"Name of the new clone"`
	Full        bool   `json:"full,omitempty" jsonschema:"Full (independent) clone rather than linked clone"`
	Storage     string `json:"storage,omitempty" jsonschema:"Storage for the new clone's disks"`
	Pool        string `json:"pool,omitempty" jsonschema:"Resource pool to place the clone in"`
	Description string `json:"description,omitempty" jsonschema:"Description of the new clone"`
	Format      string `json:"format,omitempty" jsonschema:"Target disk format (raw, qcow2, vmdk)"`
	Snapname    string `json:"snapname,omitempty" jsonschema:"Snapshot name to clone from"`
}

// vmSnapshotCreateIn is the input of pve_vm_snapshot_create.
type vmSnapshotCreateIn struct {
	Node        string `json:"node" jsonschema:"PVE node name"`
	VMID        int    `json:"vmid" jsonschema:"VM ID"`
	Snapname    string `json:"snapname" jsonschema:"Snapshot name"`
	VMState     bool   `json:"vmstate,omitempty" jsonschema:"Include the guest RAM state in the snapshot"`
	Description string `json:"description,omitempty" jsonschema:"Snapshot description"`
}

// vmSnapshotIn is the input of pve_vm_snapshot_delete / pve_vm_snapshot_rollback.
type vmSnapshotIn struct {
	Node     string `json:"node" jsonschema:"PVE node name"`
	VMID     int    `json:"vmid" jsonschema:"VM ID"`
	Snapname string `json:"snapname" jsonschema:"Snapshot name"`
}

// lxcStopIn is the input of pve_lxc_stop.
type lxcStopIn struct {
	Node      string `json:"node" jsonschema:"PVE node name"`
	VMID      int    `json:"vmid" jsonschema:"Container ID"`
	SkipLock  bool   `json:"skiplock,omitempty" jsonschema:"Ignore locks and stop anyway"`
	ForceStop bool   `json:"force_stop,omitempty" jsonschema:"Force immediate stop"`
}

// lxcShutdownIn is the input of pve_lxc_shutdown.
type lxcShutdownIn struct {
	Node      string `json:"node" jsonschema:"PVE node name"`
	VMID      int    `json:"vmid" jsonschema:"Container ID"`
	ForceStop bool   `json:"force_stop,omitempty" jsonschema:"Immediately stop if the graceful shutdown times out"`
	Timeout   int    `json:"timeout,omitempty" jsonschema:"Shutdown timeout in seconds"`
}

// lxcDeleteIn is the input of pve_lxc_delete.
type lxcDeleteIn struct {
	Node                     string `json:"node" jsonschema:"PVE node name"`
	VMID                     int    `json:"vmid" jsonschema:"Container ID"`
	Purge                    bool   `json:"purge,omitempty" jsonschema:"Remove the container from any backup jobs"`
	DestroyUnreferencedDisks bool   `json:"destroy_unreferenced_disks,omitempty" jsonschema:"Remove disks not referenced by the container config"`
	Force                    bool   `json:"force,omitempty" jsonschema:"Delete even if the container is running"`
}

// lxcCloneIn is the input of pve_lxc_clone.
type lxcCloneIn struct {
	Node        string `json:"node" jsonschema:"PVE node name"`
	VMID        int    `json:"vmid" jsonschema:"Container ID"`
	NewID       int    `json:"newid" jsonschema:"Container ID of the new clone"`
	Full        bool   `json:"full,omitempty" jsonschema:"Full (independent) clone rather than linked clone"`
	Storage     string `json:"storage,omitempty" jsonschema:"Storage for the new clone's disks"`
	Hostname    string `json:"hostname,omitempty" jsonschema:"Hostname of the new clone"`
	Description string `json:"description,omitempty" jsonschema:"Description of the new clone"`
	Pool        string `json:"pool,omitempty" jsonschema:"Resource pool to place the clone in"`
	Snapname    string `json:"snapname,omitempty" jsonschema:"Snapshot name to clone from"`
}

// lxcSnapshotCreateIn is the input of pve_lxc_snapshot_create.
type lxcSnapshotCreateIn struct {
	Node        string `json:"node" jsonschema:"PVE node name"`
	VMID        int    `json:"vmid" jsonschema:"Container ID"`
	Snapname    string `json:"snapname" jsonschema:"Snapshot name"`
	Description string `json:"description,omitempty" jsonschema:"Snapshot description"`
}

// lxcSnapshotIn is the input of pve_lxc_snapshot_delete / pve_lxc_snapshot_rollback.
type lxcSnapshotIn struct {
	Node     string `json:"node" jsonschema:"PVE node name"`
	VMID     int    `json:"vmid" jsonschema:"Container ID"`
	Snapname string `json:"snapname" jsonschema:"Snapshot name"`
}

// vmRestoreIn is the input of pve_vm_restore (qmrestore).
type vmRestoreIn struct {
	Node    string `json:"node" jsonschema:"PVE node name"`
	Archive string `json:"archive" jsonschema:"Path of the backup archive to restore (e.g. /var/lib/vz/dump/vzdump-qemu-100-*.vma.zst)"`
	VMID    int    `json:"vmid,omitempty" jsonschema:"Target VM ID; when omitted the VM is restored to its original ID"`
	Storage string `json:"storage,omitempty" jsonschema:"Target storage for the restored disks"`
	Unique  bool   `json:"unique,omitempty" jsonschema:"Restore with a new unique VM ID"`
	Force   bool   `json:"force,omitempty" jsonschema:"Overwrite an existing VM with the same ID"`
	Pool    string `json:"pool,omitempty" jsonschema:"Resource pool to place the restored VM in"`
	BwLimit int    `json:"bwlimit,omitempty" jsonschema:"Restore bandwidth limit (KB/s)"`
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
	roTool(s, "pve_backup_job_list", "List PVE backup jobs",
		"List the configured vzdump backup jobs (schedules) on the PVE cluster.",
		"No arguments. Returns every configured backup job (id, schedule, node, mode, storage, enabled, target VMs/pool, next run, notification settings). Read-only and idempotent.",
		log, func(ctx context.Context, _ emptyIn) (any, error) {
			return app.PVE.ListBackupJobs(ctx)
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
	roTool(s, "pve_vm_snapshot_list", "List QEMU VM snapshots",
		"List the snapshots of a QEMU VM.",
		"Provide node and vmid. Returns the VM's snapshots. Read-only and idempotent.",
		log, func(ctx context.Context, in vmIn) (any, error) {
			return app.PVE.ListVMSnapshots(ctx, in.Node, in.VMID)
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
	roTool(s, "pve_lxc_snapshot_list", "List LXC container snapshots",
		"List the snapshots of an LXC container.",
		"Provide node and vmid (container ID). Returns the container's snapshots. Read-only and idempotent.",
		log, func(ctx context.Context, in lxcIn) (any, error) {
			return app.PVE.ListLXCSnapshots(ctx, in.Node, in.VMID)
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
		"Provide the node and the VM configuration (name, cores, memory, disks, network, etc.). Optionally set vmid; otherwise PVE assigns the next free ID. Returns the new VMID. Not idempotent: re-issuing the same create provisions a second VM, so verify with pve_vm_list.",
		log, false, func(ctx context.Context, in createVMIn) (any, error) {
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

	mutTool(s, "pve_vm_start", "Start QEMU VM",
		"Start a QEMU VM.",
		"Provide node and vmid. Starts the VM; returns the task UPID to poll with pve_task_status. Not idempotent: starting an already-running VM errors.",
		log, false, func(ctx context.Context, in vmIn) (any, error) {
			t, err := app.PVE.StartVM(ctx, in.Node, in.VMID)
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": t.UPID}, nil
		})

	mutTool(s, "pve_vm_stop", "Stop QEMU VM",
		"Stop a QEMU VM (hard power off).",
		"Provide node and vmid. Hard-stops the VM; returns the task UPID. Set skiplock to ignore an active lock. Not idempotent.",
		log, false, func(ctx context.Context, in vmStopIn) (any, error) {
			t, err := app.PVE.StopVM(ctx, in.Node, in.VMID, in.SkipLock)
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": t.UPID}, nil
		})

	mutTool(s, "pve_vm_shutdown", "Shut down QEMU VM",
		"Gracefully shut down a QEMU VM.",
		"Provide node, vmid, optionally a timeout (seconds) and force_stop to immediately stop if the graceful shutdown times out. Returns the task UPID. Not idempotent.",
		log, false, func(ctx context.Context, in vmShutdownIn) (any, error) {
			t, err := app.PVE.ShutdownVM(ctx, in.Node, in.VMID, in.ForceStop, in.Timeout)
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": t.UPID}, nil
		})

	mutTool(s, "pve_vm_reboot", "Reboot QEMU VM",
		"Reboot a QEMU VM.",
		"Provide node, vmid and optionally a timeout (seconds). Returns the task UPID. Not idempotent.",
		log, false, func(ctx context.Context, in vmRebootIn) (any, error) {
			t, err := app.PVE.RebootVM(ctx, in.Node, in.VMID, in.Timeout)
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": t.UPID}, nil
		})

	mutTool(s, "pve_vm_reset", "Reset QEMU VM",
		"Reset a QEMU VM (reboot without graceful shutdown).",
		"Provide node and vmid. Resets the VM; returns the task UPID. Not idempotent.",
		log, false, func(ctx context.Context, in vmIn) (any, error) {
			t, err := app.PVE.ResetVM(ctx, in.Node, in.VMID)
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": t.UPID}, nil
		})

	mutTool(s, "pve_vm_suspend", "Suspend QEMU VM",
		"Suspend a QEMU VM.",
		"Provide node and vmid. Suspend the VM; set to_disk to write guest RAM to disk. Returns the task UPID. Not idempotent.",
		log, false, func(ctx context.Context, in vmSuspendIn) (any, error) {
			t, err := app.PVE.SuspendVM(ctx, in.Node, in.VMID, in.ToDisk)
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": t.UPID}, nil
		})

	mutTool(s, "pve_vm_resume", "Resume QEMU VM",
		"Resume a suspended QEMU VM.",
		"Provide node and vmid. Resumes the VM; returns the task UPID. Not idempotent.",
		log, false, func(ctx context.Context, in vmIn) (any, error) {
			t, err := app.PVE.ResumeVM(ctx, in.Node, in.VMID)
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": t.UPID}, nil
		})

	mutTool(s, "pve_vm_delete", "Delete QEMU VM",
		"Delete a QEMU VM.",
		"Provide node and vmid. Deletes the VM; returns the task UPID. Set purge to remove it from backup jobs and destroy_unreferenced_disks to remove unreferenced disks. Not idempotent and destructive.",
		log, false, func(ctx context.Context, in vmDeleteIn) (any, error) {
			t, err := app.PVE.DeleteVM(ctx, in.Node, in.VMID, in.Purge, in.DestroyUnreferencedDisks)
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": t.UPID}, nil
		}, true)

	mutTool(s, "pve_vm_clone", "Clone QEMU VM",
		"Clone a QEMU VM.",
		"Provide node, vmid and the new VM ID. Optionally set name, full (independent clone), storage, pool, description, format and snapname. Returns the task UPID. Not idempotent: each call creates a new clone.",
		log, false, func(ctx context.Context, in vmCloneIn) (any, error) {
			t, err := app.PVE.CloneVM(ctx, in.Node, in.VMID, model.CloneVMRequest{
				NewID:       in.NewID,
				Name:        in.Name,
				Full:        in.Full,
				Storage:     in.Storage,
				Pool:        in.Pool,
				Description: in.Description,
				Format:      in.Format,
				Snapname:    in.Snapname,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": t.UPID}, nil
		})

	mutTool(s, "pve_vm_snapshot_create", "Create QEMU VM snapshot",
		"Create a snapshot of a QEMU VM.",
		"Provide node, vmid and a snapshot name. Optionally include the guest RAM state (vmstate) and a description. Returns the task UPID. Not idempotent.",
		log, false, func(ctx context.Context, in vmSnapshotCreateIn) (any, error) {
			t, err := app.PVE.CreateVMSnapshot(ctx, in.Node, in.VMID, model.SnapshotCreateRequest{
				Snapname:    in.Snapname,
				VMState:     in.VMState,
				Description: in.Description,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": t.UPID}, nil
		})

	mutTool(s, "pve_vm_snapshot_delete", "Delete QEMU VM snapshot",
		"Delete a snapshot of a QEMU VM.",
		"Provide node, vmid and the snapshot name. Returns the task UPID. Not idempotent and destructive.",
		log, false, func(ctx context.Context, in vmSnapshotIn) (any, error) {
			t, err := app.PVE.DeleteVMSnapshot(ctx, in.Node, in.VMID, in.Snapname)
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": t.UPID}, nil
		}, true)

	mutTool(s, "pve_vm_snapshot_rollback", "Roll back QEMU VM to snapshot",
		"Roll a QEMU VM back to a snapshot.",
		"Provide node, vmid and the snapshot name to roll back to. Returns the task UPID. Not idempotent.",
		log, false, func(ctx context.Context, in vmSnapshotIn) (any, error) {
			t, err := app.PVE.RollbackVMSnapshot(ctx, in.Node, in.VMID, in.Snapname)
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": t.UPID}, nil
		})

	mutTool(s, "pve_lxc_start", "Start LXC container",
		"Start an LXC container.",
		"Provide node and vmid (container ID). Starts the container; returns the task UPID to poll with pve_task_status. Not idempotent: starting an already-running container errors.",
		log, false, func(ctx context.Context, in lxcIn) (any, error) {
			t, err := app.PVE.StartLXC(ctx, in.Node, in.VMID)
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": t.UPID}, nil
		})

	mutTool(s, "pve_lxc_stop", "Stop LXC container",
		"Stop an LXC container (hard stop).",
		"Provide node and vmid (container ID). Hard-stops the container; returns the task UPID. Set skiplock to ignore an active lock and force_stop to force. Not idempotent.",
		log, false, func(ctx context.Context, in lxcStopIn) (any, error) {
			t, err := app.PVE.StopLXC(ctx, in.Node, in.VMID, in.SkipLock, in.ForceStop)
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": t.UPID}, nil
		})

	mutTool(s, "pve_lxc_shutdown", "Shut down LXC container",
		"Gracefully shut down an LXC container.",
		"Provide node, vmid (container ID), optionally a timeout (seconds) and force_stop to immediately stop if the graceful shutdown times out. Returns the task UPID. Not idempotent.",
		log, false, func(ctx context.Context, in lxcShutdownIn) (any, error) {
			t, err := app.PVE.ShutdownLXC(ctx, in.Node, in.VMID, in.ForceStop, in.Timeout)
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": t.UPID}, nil
		})

	mutTool(s, "pve_lxc_reboot", "Reboot LXC container",
		"Reboot an LXC container.",
		"Provide node and vmid (container ID). Returns the task UPID. Not idempotent.",
		log, false, func(ctx context.Context, in lxcIn) (any, error) {
			t, err := app.PVE.RebootLXC(ctx, in.Node, in.VMID, 0)
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": t.UPID}, nil
		})

	mutTool(s, "pve_lxc_delete", "Delete LXC container",
		"Delete an LXC container.",
		"Provide node and vmid (container ID). Deletes the container; returns the task UPID. Set purge to remove it from backup jobs, destroy_unreferenced_disks to remove unreferenced disks, and force to delete a running container. Not idempotent and destructive.",
		log, false, func(ctx context.Context, in lxcDeleteIn) (any, error) {
			t, err := app.PVE.DeleteLXC(ctx, in.Node, in.VMID, in.Purge, in.DestroyUnreferencedDisks, in.Force)
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": t.UPID}, nil
		}, true)

	mutTool(s, "pve_lxc_clone", "Clone LXC container",
		"Clone an LXC container.",
		"Provide node, vmid (container ID) and the new container ID. Optionally set full (independent clone), storage, hostname, description, pool and snapname. Returns the task UPID. Not idempotent: each call creates a new clone.",
		log, false, func(ctx context.Context, in lxcCloneIn) (any, error) {
			t, err := app.PVE.CloneLXC(ctx, in.Node, in.VMID, model.CloneLXCRequest{
				NewID:       in.NewID,
				Full:        in.Full,
				Storage:     in.Storage,
				Hostname:    in.Hostname,
				Description: in.Description,
				Pool:        in.Pool,
				Snapname:    in.Snapname,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": t.UPID}, nil
		})

	mutTool(s, "pve_lxc_snapshot_create", "Create LXC container snapshot",
		"Create a snapshot of an LXC container.",
		"Provide node, vmid (container ID) and a snapshot name, optionally with a description. Returns the task UPID. Not idempotent.",
		log, false, func(ctx context.Context, in lxcSnapshotCreateIn) (any, error) {
			t, err := app.PVE.CreateLXCSnapshot(ctx, in.Node, in.VMID, model.SnapshotCreateRequest{
				Snapname:    in.Snapname,
				Description: in.Description,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": t.UPID}, nil
		})

	mutTool(s, "pve_lxc_snapshot_delete", "Delete LXC container snapshot",
		"Delete a snapshot of an LXC container.",
		"Provide node, vmid (container ID) and the snapshot name. Returns the task UPID. Not idempotent and destructive.",
		log, false, func(ctx context.Context, in lxcSnapshotIn) (any, error) {
			t, err := app.PVE.DeleteLXCSnapshot(ctx, in.Node, in.VMID, in.Snapname)
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": t.UPID}, nil
		}, true)

	mutTool(s, "pve_lxc_snapshot_rollback", "Roll back LXC container to snapshot",
		"Roll an LXC container back to a snapshot.",
		"Provide node, vmid (container ID) and the snapshot name to roll back to. Returns the task UPID. Not idempotent.",
		log, false, func(ctx context.Context, in lxcSnapshotIn) (any, error) {
			t, err := app.PVE.RollbackLXCSnapshot(ctx, in.Node, in.VMID, in.Snapname)
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": t.UPID}, nil
		})

	mutTool(s, "pve_vm_restore", "Restore QEMU VM backup",
		"Restore a QEMU VM from a backup archive (qmrestore).",
		"Provide node and the archive path. Optionally set vmid (otherwise the VM is restored to its original ID), storage, unique, force, pool and bwlimit. Returns the task UPID to poll with pve_task_status. Not idempotent: each call starts a new restore.",
		log, false, func(ctx context.Context, in vmRestoreIn) (any, error) {
			t, err := app.PVE.RestoreVM(ctx, in.Node, model.PVERestoreRequest{
				Archive: in.Archive,
				VMID:    in.VMID,
				Storage: in.Storage,
				Unique:  in.Unique,
				Force:   in.Force,
				Pool:    in.Pool,
				BwLimit: in.BwLimit,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": t.UPID}, nil
		})
}
