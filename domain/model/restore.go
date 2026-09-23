package model

// PBSFile is a file inside a PBS backup snapshot as returned by
// GET /admin/datastore/{store}/backups/{type}/{id}/{snapshot}/files. Size is in
// bytes.
type PBSFile struct {
	Filename string `json:"filename"`
	Type     string `json:"type,omitempty"`
	Size     int64  `json:"size,omitempty"`
}

// PBSFileRestoreRequest is the form body of
// POST /admin/datastore/{store}/backups/{type}/{id}/{snapshot}/files. Path is
// the source file path inside the backup and Target is the destination path on
// the PBS filesystem.
type PBSFileRestoreRequest struct {
	Path   string `json:"path"`
	Target string `json:"target"`
}

// PBSVMRestoreRequest is the form body of
// POST /admin/datastore/{store}/backups/{type}/{id}/{snapshot}/restore. Target
// is the PVE storage to restore into; VMID is either a numeric ID or the string
// "next" to let PVE pick the next free ID. Password/Fingerprint are used to
// authenticate to the target PVE node and are never logged.
type PBSVMRestoreRequest struct {
	Target      string `json:"target"`
	VMID        string `json:"vmid,omitempty"`
	Host        string `json:"host,omitempty"`
	Password    string `json:"password,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Pool        string `json:"pool,omitempty"`
	Verbose     bool   `json:"verbose,omitempty"`
	Reload      bool   `json:"reload,omitempty"`
}

// PBSTask is the status of a task by UPID from GET /admin/tasks/{upid}.
type PBSTask struct {
	UPID      string `json:"upid"`
	Type      string `json:"type,omitempty"`
	Status    string `json:"status,omitempty"`
	StartTime int64  `json:"starttime,omitempty"`
	EndTime   int64  `json:"endtime,omitempty"`
	Worker    string `json:"worker,omitempty"`
	Errors    int    `json:"errors,omitempty"`
}
