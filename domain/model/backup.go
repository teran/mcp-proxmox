package model

import "encoding/json"

// Backup is a backup snapshot in a datastore as returned by
// GET /admin/datastore/{store}/snapshots.
type Backup struct {
	BackupID    string          `json:"backup-id"`
	BackupTime  int64           `json:"backup-time"`
	BackupType  string          `json:"backup-type"`
	Owner       string          `json:"owner,omitempty"`
	Comment     string          `json:"comment,omitempty"`
	Size        int64           `json:"size,omitempty"`
	Files       json.RawMessage `json:"files,omitempty"` // array of file objects (shape varies)
	Fingerprint string          `json:"fingerprint,omitempty"`
	Protected   bool            `json:"protected,omitempty"`
}

// BackupNotes are the notes of a backup group as returned by
// GET /admin/datastore/{store}/group-notes?backup-id=<id>&backup-type=<type>.
// PBS keeps a comment and free-form notes per backup group.
type BackupNotes struct {
	Comment string `json:"comment,omitempty"`
	Notes   string `json:"notes,omitempty"`
}
