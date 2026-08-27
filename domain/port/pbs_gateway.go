package port

import (
	"context"

	"github.com/teran/mcp-proxmox/domain/model"
)

// PBSGateway is the secondary port implemented by adapter/pbs. It models the
// Proxmox Backup Server REST API (/api2/json) for the read-only (query)
// surface. Mutation methods (restore/forget/notes-set/verify-start/prune-start)
// will be added when the mutation gate is implemented (SPEC.md §2.4 / §6).
type PBSGateway interface {
	// Datastores.
	ListDatastores(ctx context.Context) ([]model.Datastore, error)
	GetDatastoreStatus(ctx context.Context, store string) (*model.DatastoreStatus, error)

	// Backups / snapshots & notes.
	ListBackups(ctx context.Context, store string) ([]model.Backup, error)
	GetBackup(ctx context.Context, store, snapshot string) (*model.Backup, error)
	GetBackupNotes(ctx context.Context, store, snapshot string) (*model.BackupNotes, error)

	// Verify & prune (status polling is read-only).
	GetVerifyStatus(ctx context.Context, store, upid string) (*model.VerifyStatus, error)
	GetPruneStatus(ctx context.Context, store, upid string) (*model.PruneStatus, error)

	// Version.
	GetPBSVersion(ctx context.Context) (*model.PBSVersion, error)
}
