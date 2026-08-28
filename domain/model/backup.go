package model

// Backup is a backup snapshot in a datastore as returned by
// GET /admin/datastore/{store}/snapshots.
type Backup struct {
	BackupID    string   `json:"backup-id"`
	BackupTime  int64    `json:"backup-time"`
	BackupType  string   `json:"backup-type"`
	Owner       string   `json:"owner,omitempty"`
	Comment     string   `json:"comment,omitempty"`
	Size        int64    `json:"size,omitempty"`
	Files       []string `json:"files,omitempty"`
	Fingerprint string   `json:"fingerprint,omitempty"`
	Protected   bool     `json:"protected,omitempty"`
}

// BackupNotes are the notes attached to a backup snapshot from
// GET /admin/datastore/{store}/snapshot/{snapshot}/notes.
type BackupNotes struct {
	Snapshot string `json:"snapshot"`
	Notes    string `json:"notes"`
}
