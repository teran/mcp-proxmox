package application

import (
	"context"

	"github.com/teran/mcp-proxmox/domain/model"
	"github.com/teran/mcp-proxmox/domain/port"
)

// PBSService is the PBS use cases (read-only surface for this milestone). It is
// a thin orchestration layer over port.PBSGateway. See SPEC.md §3.2 / §6.
type PBSService struct {
	gw  port.PBSGateway
	log port.CtxLogger
}

// ListDatastores returns the PBS datastores.
func (s *PBSService) ListDatastores(ctx context.Context) ([]model.Datastore, error) {
	return s.gw.ListDatastores(ctx)
}

// GetDatastoreStatus returns the status of a single datastore.
func (s *PBSService) GetDatastoreStatus(ctx context.Context, store string) (*model.DatastoreStatus, error) {
	return s.gw.GetDatastoreStatus(ctx, store)
}

// ListBackups returns the backup snapshots in a datastore.
func (s *PBSService) ListBackups(ctx context.Context, store string) ([]model.Backup, error) {
	return s.gw.ListBackups(ctx, store)
}

// GetBackup returns a single backup snapshot.
func (s *PBSService) GetBackup(ctx context.Context, store, snapshot string) (*model.Backup, error) {
	return s.gw.GetBackup(ctx, store, snapshot)
}

// GetBackupNotes returns the notes of a backup snapshot.
func (s *PBSService) GetBackupNotes(ctx context.Context, store, snapshot string) (*model.BackupNotes, error) {
	return s.gw.GetBackupNotes(ctx, store, snapshot)
}

// GetVerifyStatus returns the status of a verify job by UPID.
func (s *PBSService) GetVerifyStatus(ctx context.Context, store, upid string) (*model.VerifyStatus, error) {
	return s.gw.GetVerifyStatus(ctx, store, upid)
}

// GetPruneStatus returns the status of a prune job by UPID.
func (s *PBSService) GetPruneStatus(ctx context.Context, store, upid string) (*model.PruneStatus, error) {
	return s.gw.GetPruneStatus(ctx, store, upid)
}

// GetPBSVersion returns the PBS version info.
func (s *PBSService) GetPBSVersion(ctx context.Context) (*model.PBSVersion, error) {
	return s.gw.GetPBSVersion(ctx)
}
