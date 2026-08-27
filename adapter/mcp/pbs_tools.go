package mcp

import (
	"context"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-proxmox/application"
)

// storeIn identifies a datastore (pbs_datastore_status / pbs_backup_list).
type storeIn struct {
	Store string `json:"store" jsonschema:"PBS datastore name"`
}

// backupIn identifies a backup snapshot in a datastore (pbs_backup_get /
// pbs_backup_notes_get).
type backupIn struct {
	Store    string `json:"store" jsonschema:"PBS datastore name"`
	Snapshot string `json:"snapshot" jsonschema:"Backup snapshot ID"`
}

// jobIn identifies a verify/prune job by UPID (pbs_verify_status /
// pbs_prune_status).
type jobIn struct {
	Store string `json:"store" jsonschema:"PBS datastore name"`
	UPID  string `json:"upid" jsonschema:"Job UPID"`
}

// registerPBSTools registers the Proxmox Backup Server tools. Read-only (query)
// tools are always registered for an enabled PBS backend. Mutation tools are
// registered only when enableMutations is true (gate UX TBD — SPEC.md §2.4);
// for this milestone only the read-only surface is present.
func registerPBSTools(s *mcpSDK.Server, app *application.App, log toolLogger, enableMutations bool) {
	// --- datastores ---
	roTool(s, "pbs_datastore_list", "List the PBS datastores.", log, func(ctx context.Context, _ emptyIn) (any, error) {
		return app.PBS.ListDatastores(ctx)
	})
	roTool(s, "pbs_datastore_status", "Get the status of a PBS datastore.", log, func(ctx context.Context, in storeIn) (any, error) {
		return app.PBS.GetDatastoreStatus(ctx, in.Store)
	})

	// --- backups / snapshots & notes ---
	roTool(s, "pbs_backup_list", "List the backup snapshots in a PBS datastore.", log, func(ctx context.Context, in storeIn) (any, error) {
		return app.PBS.ListBackups(ctx, in.Store)
	})
	roTool(s, "pbs_backup_get", "Get a single backup snapshot.", log, func(ctx context.Context, in backupIn) (any, error) {
		return app.PBS.GetBackup(ctx, in.Store, in.Snapshot)
	})
	roTool(s, "pbs_backup_notes_get", "Get the notes of a backup snapshot.", log, func(ctx context.Context, in backupIn) (any, error) {
		return app.PBS.GetBackupNotes(ctx, in.Store, in.Snapshot)
	})

	// --- verify & prune (status polling is read-only) ---
	roTool(s, "pbs_verify_status", "Get the status of a verify job.", log, func(ctx context.Context, in jobIn) (any, error) {
		return app.PBS.GetVerifyStatus(ctx, in.Store, in.UPID)
	})
	roTool(s, "pbs_prune_status", "Get the status of a prune job.", log, func(ctx context.Context, in jobIn) (any, error) {
		return app.PBS.GetPruneStatus(ctx, in.Store, in.UPID)
	})

	// --- version ---
	roTool(s, "pbs_version", "Get the Proxmox Backup Server version info.", log, func(ctx context.Context, _ emptyIn) (any, error) {
		return app.PBS.GetPBSVersion(ctx)
	})

	// --- mutations (gated) ---
	if !enableMutations {
		return
	}
	// Mutation tools (pbs_backup_restore/forget, pbs_backup_notes_set,
	// pbs_verify_start, pbs_prune_start) are registered here when the gate is
	// enabled. See SPEC.md §6.
}
