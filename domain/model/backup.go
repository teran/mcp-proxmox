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
// PBS returns the notes as a plain string.
type BackupNotes struct {
	Notes string `json:"notes,omitempty"`
}
