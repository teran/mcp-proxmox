package mcp

import (
	"context"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/model"
)

// storeIn identifies a datastore (pbs_datastore_status / pbs_backup_list).
type storeIn struct {
	Store string `json:"store" jsonschema:"PBS datastore name"`
}

// backupIn identifies a backup group in a datastore (pbs_backup_get).
type backupIn struct {
	Store    string `json:"store" jsonschema:"PBS datastore name"`
	BackupID string `json:"backup_id" jsonschema:"Backup ID / group ID (e.g. a VMID)"`
}

// backupNotesIn identifies a backup group for notes lookup (pbs_backup_notes_get).
type backupNotesIn struct {
	Store      string `json:"store" jsonschema:"PBS datastore name"`
	BackupID   string `json:"backup_id" jsonschema:"Backup ID / group ID (e.g. a VMID)"`
	BackupType string `json:"backup_type" jsonschema:"Backup type: vm | ct | host"`
}

// jobIn identifies a verify/prune job by UPID (pbs_verify_status /
// pbs_prune_status).
type jobIn struct {
	Store string `json:"store" jsonschema:"PBS datastore name"`
	UPID  string `json:"upid" jsonschema:"Job UPID"`
}

// syncIn is the input of pbs_sync_start (a datastore plus sync-job parameters).
type syncIn struct {
	Store          string `json:"store" jsonschema:"PBS datastore name"`
	Remote         string `json:"remote,omitempty" jsonschema:"Remote name"`
	RemoteStore    string `json:"remote_store,omitempty" jsonschema:"Remote datastore name"`
	Owner          string `json:"owner,omitempty" jsonschema:"Owner of the backups to sync"`
	MaxFiles       int    `json:"max_files,omitempty" jsonschema:"Maximum number of files to sync"`
	RemoveVanished bool   `json:"remove_vanished,omitempty" jsonschema:"Remove backups that vanished on the remote"`
	RateIn         int    `json:"rate_in,omitempty" jsonschema:"Limit restore/import bandwidth (KB/s)"`
	RateOut        int    `json:"rate_out,omitempty" jsonschema:"Limit upload bandwidth (KB/s)"`
	SkipLost       bool   `json:"skip_lost,omitempty" jsonschema:"Skip lost backups on the remote"`
	NotifyUser     string `json:"notify_user,omitempty" jsonschema:"Notify user on job completion"`
}

// pbsFileListIn is the input of pbs_backup_files_list.
type pbsFileListIn struct {
	Store      string `json:"store" jsonschema:"PBS datastore name"`
	BackupType string `json:"backup_type" jsonschema:"Backup type: vm | ct | host"`
	BackupID   string `json:"backup_id" jsonschema:"Backup ID / group ID (e.g. a VMID)"`
	Snapshot   string `json:"snapshot" jsonschema:"Backup snapshot identifier (the ISO-8601 timestamp shown in the PBS UI, e.g. 2024-01-01T00:00:00Z); obtain it from the backup listing."`
	Path       string `json:"path,omitempty" jsonschema:"Directory path within the backup to list (default /)"`
}

// pbsTaskIn is the input of pbs_task_status.
type pbsTaskIn struct {
	UPID string `json:"upid" jsonschema:"Task UPID"`
}

// pbsTaskLogIn is the input of pbs_task_log.
type pbsTaskLogIn struct {
	UPID  string `json:"upid" jsonschema:"Task UPID"`
	Limit int    `json:"limit,omitempty" jsonschema:"Maximum number of log lines"`
}

// pbsFileRestoreIn is the input of pbs_backup_restore_file.
type pbsFileRestoreIn struct {
	Store      string `json:"store" jsonschema:"PBS datastore name"`
	BackupType string `json:"backup_type" jsonschema:"Backup type: vm | ct | host"`
	BackupID   string `json:"backup_id" jsonschema:"Backup ID / group ID (e.g. a VMID)"`
	Snapshot   string `json:"snapshot" jsonschema:"Backup snapshot identifier (the ISO-8601 timestamp shown in the PBS UI, e.g. 2024-01-01T00:00:00Z); obtain it from the backup listing."`
	Path       string `json:"path" jsonschema:"Source file path within the backup to restore"`
	Target     string `json:"target" jsonschema:"Destination path on the PBS filesystem"`
}

// pbsVMRestoreIn is the input of pbs_backup_vm_restore.
type pbsVMRestoreIn struct {
	Store       string `json:"store" jsonschema:"PBS datastore name"`
	BackupType  string `json:"backup_type" jsonschema:"Backup type: vm | ct | host"`
	BackupID    string `json:"backup_id" jsonschema:"Backup ID / group ID (e.g. a VMID)"`
	Snapshot    string `json:"snapshot" jsonschema:"Backup snapshot identifier (the ISO-8601 timestamp shown in the PBS UI, e.g. 2024-01-01T00:00:00Z); obtain it from the backup listing."`
	Target      string `json:"target" jsonschema:"PVE storage to restore into"`
	VMID        string `json:"vmid,omitempty" jsonschema:"Target VM ID, or 'next' to let PVE pick the next free ID"`
	Host        string `json:"host,omitempty" jsonschema:"PVE node hostname/IP to restore to"`
	Password    string `json:"password,omitempty" jsonschema:"Password to authenticate to the target PVE node"`
	Fingerprint string `json:"fingerprint,omitempty" jsonschema:"TLS fingerprint of the target PVE node"`
	Pool        string `json:"pool,omitempty" jsonschema:"Resource pool to place the restored VM in"`
	Verbose     bool   `json:"verbose,omitempty" jsonschema:"Verbose restore output"`
	Reload      bool   `json:"reload,omitempty" jsonschema:"Reload the VM configuration after restore"`
}

// registerPBSTools registers the Proxmox Backup Server tools. Read-only (query)
// tools are always registered for an enabled PBS backend. Mutation tools are
// registered only when enableMutations is true (SPEC.md §2.5).
func registerPBSTools(s *mcpSDK.Server, app *application.App, log toolLogger, enableMutations bool) {
	// --- datastores ---
	roTool(s, "pbs_datastore_list", "List PBS datastores",
		"List the PBS datastores.",
		"No arguments. Returns all configured datastores. Read-only and idempotent.",
		log, func(ctx context.Context, _ emptyIn) (any, error) {
			return app.PBS.ListDatastores(ctx)
		})
	roTool(s, "pbs_datastore_status", "Get PBS datastore status",
		"Get the status of a PBS datastore.",
		"Provide the datastore name. Returns its status and capacity usage. Read-only.",
		log, func(ctx context.Context, in storeIn) (any, error) {
			return app.PBS.GetDatastoreStatus(ctx, in.Store)
		})
	roTool(s, "pbs_datastore_config_get", "Get PBS datastore configuration",
		"Get the full configuration of a PBS datastore.",
		"Provide the datastore name. Returns its configuration (path, retention/keep settings, GC schedule, notification options, etc.). Read-only.",
		log, func(ctx context.Context, in storeIn) (any, error) {
			return app.PBS.GetDatastoreConfig(ctx, in.Store)
		})

	// --- backups / snapshots & notes ---
	roTool(s, "pbs_backup_list", "List PBS backup snapshots",
		"List the backup snapshots in a PBS datastore.",
		"Provide the datastore name. Returns all backup snapshots in it. Read-only and idempotent.",
		log, func(ctx context.Context, in storeIn) (any, error) {
			return app.PBS.ListBackups(ctx, in.Store)
		})
	roTool(s, "pbs_backup_get", "Get PBS backup group",
		"Get all backup snapshots in a PBS datastore for a given backup ID / group ID (e.g. a VMID).",
		"Provide the datastore name and a backup ID (group ID, e.g. a VMID). Returns the matching snapshots. Read-only.",
		log, func(ctx context.Context, in backupIn) (any, error) {
			return app.PBS.GetBackup(ctx, in.Store, in.BackupID)
		})
	roTool(s, "pbs_backup_notes_get", "Get PBS backup group notes",
		"Get the notes and comment of a PBS backup group (requires backup ID and backup type vm|ct|host).",
		"Provide the datastore name, backup ID, and backup type (vm, ct, or host). Returns the group's notes. Read-only.",
		log, func(ctx context.Context, in backupNotesIn) (any, error) {
			return app.PBS.GetBackupNotes(ctx, in.Store, in.BackupID, in.BackupType)
		})

	// --- verify & prune (status polling is read-only) ---
	roTool(s, "pbs_verify_status", "Get PBS verify job status",
		"Get the status of a PBS verify job by UPID. The UPID comes from a running/known verify job (PBS has no task-list endpoint in this version).",
		"Provide the datastore name and the verify job's UPID. Returns the job status; use to poll a running verify. Read-only.",
		log, func(ctx context.Context, in jobIn) (any, error) {
			return app.PBS.GetVerifyStatus(ctx, in.Store, in.UPID)
		})
	roTool(s, "pbs_prune_status", "Get PBS prune job status",
		"Get the status of a PBS prune job by UPID. The UPID comes from a running/known prune job (PBS has no task-list endpoint in this version).",
		"Provide the datastore name and the prune job's UPID. Returns the job status; use to poll a running prune. Read-only.",
		log, func(ctx context.Context, in jobIn) (any, error) {
			return app.PBS.GetPruneStatus(ctx, in.Store, in.UPID)
		})

	// --- version ---
	roTool(s, "pbs_version", "Get PBS version",
		"Get the Proxmox Backup Server version info.",
		"No arguments. Returns the PBS version. Read-only.",
		log, func(ctx context.Context, _ emptyIn) (any, error) {
			return app.PBS.GetPBSVersion(ctx)
		})

	// --- restore & task status (read-only) ---
	roTool(s, "pbs_backup_files_list", "List PBS backup files",
		"List the files inside a PBS backup snapshot.",
		"Provide the datastore name, backup type (vm|ct|host), backup ID, and the snapshot identifier (the ISO-8601 timestamp from the backup listing). Optionally set path to list a subdirectory (default /). Read-only and idempotent.",
		log, func(ctx context.Context, in pbsFileListIn) (any, error) {
			return app.PBS.ListBackupFiles(ctx, in.Store, in.BackupType, in.BackupID, in.Snapshot, in.Path)
		})
	roTool(s, "pbs_task_status", "Get PBS task status",
		"Get the status of a PBS task by UPID.",
		"Provide the task UPID (e.g. from a restore or verify job). Returns the task status. Read-only.",
		log, func(ctx context.Context, in pbsTaskIn) (any, error) {
			return app.PBS.GetTaskStatus(ctx, in.UPID)
		})
	roTool(s, "pbs_task_log", "Get PBS task log",
		"Get the log of a PBS task by UPID.",
		"Provide the task UPID and optionally a limit on the number of log lines. Returns the task log. Read-only.",
		log, func(ctx context.Context, in pbsTaskLogIn) (any, error) {
			return app.PBS.GetTaskLog(ctx, in.UPID, in.Limit)
		})

	// --- mutations (gated by EnableMutations) ---
	if !enableMutations {
		return
	}

	mutTool(s, "pbs_verify_start", "Start PBS verify",
		"Start a verify job on a PBS datastore.",
		"Provide the datastore name. Returns the job UPID to poll with pbs_verify_status. Not idempotent: each call starts a new verify.",
		log, false, func(ctx context.Context, in storeIn) (any, error) {
			upid, err := app.PBS.StartVerify(ctx, in.Store)
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": upid}, nil
		})

	mutTool(s, "pbs_gc_start", "Start PBS garbage collection",
		"Start a garbage-collection job on a PBS datastore.",
		"Provide the datastore name. Returns the job UPID. Not idempotent: each call starts a new GC.",
		log, false, func(ctx context.Context, in storeIn) (any, error) {
			upid, err := app.PBS.StartGC(ctx, in.Store)
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": upid}, nil
		})

	mutTool(s, "pbs_prune_start", "Start PBS prune",
		"Start a prune job on a PBS datastore.",
		"Provide the datastore name. Returns the job UPID to poll with pbs_prune_status. Not idempotent: each call starts a new prune.",
		log, false, func(ctx context.Context, in storeIn) (any, error) {
			upid, err := app.PBS.StartPrune(ctx, in.Store)
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": upid}, nil
		})

	mutTool(s, "pbs_sync_start", "Start PBS sync",
		"Start a one-off sync job on a PBS datastore.",
		"Provide the datastore name plus the sync parameters (remote, remote_store, owner, limits, etc.). Returns the job UPID. Not idempotent: each call starts a new sync.",
		log, false, func(ctx context.Context, in syncIn) (any, error) {
			upid, err := app.PBS.StartSync(ctx, in.Store, model.PBSSyncRequest{
				Remote:         in.Remote,
				RemoteStore:    in.RemoteStore,
				Owner:          in.Owner,
				MaxFiles:       in.MaxFiles,
				RemoveVanished: in.RemoveVanished,
				RateIn:         in.RateIn,
				RateOut:        in.RateOut,
				SkipLost:       in.SkipLost,
				NotifyUser:     in.NotifyUser,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": upid}, nil
		})

	mutTool(s, "pbs_backup_restore_file", "Restore single file from PBS backup",
		"Restore a single file out of a PBS backup snapshot to a target path on the PBS filesystem.",
		"Provide the datastore name, backup type (vm|ct|host), backup ID, the snapshot identifier, the source path inside the backup and the target path on the PBS filesystem. Returns no task UPID. Not idempotent.",
		log, false, func(ctx context.Context, in pbsFileRestoreIn) (any, error) {
			return nil, app.PBS.RestoreFile(ctx, in.Store, in.BackupType, in.BackupID, in.Snapshot, model.PBSFileRestoreRequest{
				Path:   in.Path,
				Target: in.Target,
			})
		})

	mutTool(s, "pbs_backup_vm_restore", "Restore PBS VM backup",
		"Restore a PBS VM backup snapshot into a PVE datastore.",
		"Provide the datastore name, backup type (vm|ct|host), backup ID, the snapshot identifier, the target PVE storage, and optionally a VM ID (or 'next'), host, password, fingerprint, pool, verbose and reload. Returns the task UPID to poll with pbs_task_status. Not idempotent: each call starts a new restore.",
		log, false, func(ctx context.Context, in pbsVMRestoreIn) (any, error) {
			upid, err := app.PBS.RestoreVMBackup(ctx, in.Store, in.BackupType, in.BackupID, in.Snapshot, model.PBSVMRestoreRequest{
				Target:      in.Target,
				VMID:        in.VMID,
				Host:        in.Host,
				Password:    in.Password,
				Fingerprint: in.Fingerprint,
				Pool:        in.Pool,
				Verbose:     in.Verbose,
				Reload:      in.Reload,
			})
			if err != nil {
				return nil, err
			}
			return map[string]any{"upid": upid}, nil
		})
}
