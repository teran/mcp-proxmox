package application

import (
	"context"

	"github.com/teran/mcp-proxmox/domain/model"
	"github.com/teran/mcp-proxmox/domain/port"
)

// PBSService is the PBS use cases. It is a thin orchestration layer over
// port.PBSGateway. See SPEC.md §3.2 / §6.
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

// GetBackup returns all backup snapshots in the datastore matching backupID.
func (s *PBSService) GetBackup(ctx context.Context, store, backupID string) ([]model.Backup, error) {
	return s.gw.GetBackup(ctx, store, backupID)
}

// GetBackupNotes returns the notes of a backup group.
func (s *PBSService) GetBackupNotes(ctx context.Context, store, backupID, backupType string) (*model.BackupNotes, error) {
	return s.gw.GetBackupNotes(ctx, store, backupID, backupType)
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

// ListBackupFiles lists the files inside a PBS backup snapshot.
func (s *PBSService) ListBackupFiles(ctx context.Context, store, backupType, backupID, snapshot, path string) ([]model.PBSFile, error) {
	return s.gw.ListBackupFiles(ctx, store, backupType, backupID, snapshot, path)
}

// GetTaskStatus returns the status of a PBS task by UPID.
func (s *PBSService) GetTaskStatus(ctx context.Context, upid string) (*model.PBSTask, error) {
	return s.gw.GetTaskStatus(ctx, upid)
}

// GetTaskLog returns the log of a PBS task by UPID.
func (s *PBSService) GetTaskLog(ctx context.Context, upid string, limit int) ([]model.TaskLogEntry, error) {
	return s.gw.GetTaskLog(ctx, upid, limit)
}

// RestoreFile restores a single file out of a PBS backup snapshot.
func (s *PBSService) RestoreFile(ctx context.Context, store, backupType, backupID, snapshot string, req model.PBSFileRestoreRequest) error {
	return s.gw.RestoreFile(ctx, store, backupType, backupID, snapshot, req)
}

// RestoreVMBackup restores a PBS VM backup snapshot into a PVE datastore and
// returns the task UPID.
func (s *PBSService) RestoreVMBackup(ctx context.Context, store, backupType, backupID, snapshot string, req model.PBSVMRestoreRequest) (string, error) {
	return s.gw.RestoreVMBackup(ctx, store, backupType, backupID, snapshot, req)
}

// StartVerify starts a verify job on a datastore and returns its UPID.
func (s *PBSService) StartVerify(ctx context.Context, store string) (string, error) {
	return s.gw.StartVerify(ctx, store)
}

// StartGC starts a garbage-collection job on a datastore and returns its UPID.
func (s *PBSService) StartGC(ctx context.Context, store string) (string, error) {
	return s.gw.StartGC(ctx, store)
}

// StartPrune starts a prune job on a datastore and returns its UPID.
func (s *PBSService) StartPrune(ctx context.Context, store string) (string, error) {
	return s.gw.StartPrune(ctx, store)
}

// StartSync starts a one-off sync job on a datastore and returns its UPID.
func (s *PBSService) StartSync(ctx context.Context, store string, req model.PBSSyncRequest) (string, error) {
	return s.gw.StartSync(ctx, store, req)
}
