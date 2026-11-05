package port

import (
	"context"

	"github.com/teran/mcp-proxmox/domain/model"
)

// PBSBackupListOptions filters GET /admin/datastore/{store}/snapshots (the
// backup listing). See SPEC.md §11.1.
type PBSBackupListOptions struct {
	Limit int   `json:"limit,omitempty"` // cap the number of returned snapshots
	Start int64 `json:"start,omitempty"` // unix timestamp (seconds) lower bound
	End   int64 `json:"end,omitempty"`   // unix timestamp (seconds) upper bound
}

// PBSGateway is the secondary port implemented by adapter/pbs. It models the
// Proxmox Backup Server REST API (/api2/json): the read-only (query) surface
// plus the mutation surface (verify/gc/prune/sync-start), which the MCP adapter
// registers only when the ENABLE_MUTATIONS gate is on (SPEC.md §2.5).
type PBSGateway interface {
	// Datastores.
	ListDatastores(ctx context.Context) ([]model.Datastore, error)
	GetDatastoreStatus(ctx context.Context, store string) (*model.DatastoreStatus, error)
	GetDatastoreConfig(ctx context.Context, store string) (*model.DatastoreConfig, error)

	// Backups / snapshots & notes.
	// ListBackups returns the backup snapshots in a datastore, filtered by the
	// pagination options (limit/start/end).
	ListBackups(ctx context.Context, store string, opts PBSBackupListOptions) ([]model.Backup, error)
	// GetBackup returns all snapshots in the datastore whose backup-id matches
	// backupID (the real grouping key, e.g. a VMID). PBS has no single-snapshot
	// GET; snapshots are listed and filtered by backup-id. Returns an empty
	// slice (no error) when nothing matches.
	GetBackup(ctx context.Context, store, backupID string, opts PBSBackupListOptions) ([]model.Backup, error)
	// GetBackupNotes returns the notes of a backup group identified by
	// backup-id and backup-type (vm|ct|host).
	GetBackupNotes(ctx context.Context, store, backupID, backupType string) (*model.BackupNotes, error)

	// Backup file listing & task status (read-only).
	// ListBackupFiles lists the files inside a backup snapshot via
	// GET /admin/datastore/{store}/files. The snapshot is identified by backup
	// type (vm|ct|host), backup ID and backupTime — the snapshot's Unix epoch
	// (seconds) as returned by the backup listing (model.Backup.BackupTime).
	// path optionally restricts the listing to a subdirectory (query `path`,
	// default "/").
	ListBackupFiles(ctx context.Context, store, backupType, backupID string, backupTime int64, path string) ([]model.PBSFile, error)
	// GetTaskStatus returns the status of a task by UPID
	// (GET /nodes/localhost/tasks/{upid}/status).
	GetTaskStatus(ctx context.Context, upid string) (*model.PBSTask, error)
	// GetTaskLog returns the log of a task by UPID
	// (GET /nodes/localhost/tasks/{upid}/log).
	GetTaskLog(ctx context.Context, upid string, limit int) ([]model.TaskLogEntry, error)

	// Verify & prune (status polling is read-only). PBS exposes a verify/prune
	// job only as a task, so both are polled through the task-status endpoint
	// (GET /nodes/localhost/tasks/{upid}/status), which returns a PBSTask.
	GetVerifyStatus(ctx context.Context, upid string) (*model.PBSTask, error)
	GetPruneStatus(ctx context.Context, upid string) (*model.PBSTask, error)

	// RestoreFile restores a single file out of a backup snapshot to a target
	// path on the PBS filesystem. The snapshot is identified by backupTime —
	// its Unix epoch (seconds) as returned by the backup listing. It is a
	// mutation (gated by EnableMutations), never retried, and returns no task
	// UPID.
	RestoreFile(ctx context.Context, store, backupType, backupID string, backupTime int64, req model.PBSFileRestoreRequest) error
	// RestoreVMBackup restores a VM backup snapshot into a PVE datastore. The
	// snapshot is identified by backupTime — its Unix epoch (seconds) as
	// returned by the backup listing. It is a mutation (gated by
	// EnableMutations), never retried, and returns the task UPID to poll with
	// GetTaskStatus / GetTaskLog.
	RestoreVMBackup(
		ctx context.Context, store, backupType, backupID string, backupTime int64, req model.PBSVMRestoreRequest,
	) (string, error)

	// Mutations (gated by EnableMutations). These are never retried and return
	// the task UPID taken from the response `data` field.
	StartVerify(ctx context.Context, store string) (string, error)
	StartGC(ctx context.Context, store string) (string, error)
	StartPrune(ctx context.Context, store string) (string, error)
	StartSync(ctx context.Context, store string, req model.PBSSyncRequest) (string, error)
	// StopTask stops (cancels) a running PBS task by UPID
	// (DELETE /nodes/localhost/tasks/{upid}). It is a destructive mutation
	// (gated by EnableMutations), never retried, and returns no task UPID.
	StopTask(ctx context.Context, upid string) error
	// ForgetBackup forgets (deletes) a single backup snapshot within a group,
	// identified by datastore, backup type (vm|ct|host), backup ID and the
	// snapshot's Unix epoch in seconds (DELETE /admin/datastore/{store}/snapshots/
	// {backup_type}/{backup_id}/{backup_time}). It is a destructive mutation
	// (gated by EnableMutations), never retried, and returns no task UPID.
	ForgetBackup(ctx context.Context, store, backupType, backupID string, backupTime int64) error

	// Version.
	GetPBSVersion(ctx context.Context) (*model.PBSVersion, error)
}
