package model

// ClusterStatus is one entry of GET /cluster/status. It describes a single
// node (or the cluster as a whole) with its quorum/health state.
type ClusterStatus struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Nodes  int    `json:"nodes,omitempty"`
	Quorum int    `json:"quorate,omitempty"` // Proxmox returns quorate as 0/1
	Online int    `json:"online,omitempty"`
	IP     string `json:"ip,omitempty"`
	Local  int    `json:"local,omitempty"` // Proxmox returns local as 0/1
}

// ClusterResource is one entry of GET /cluster/resources: a VM, CT or storage
// resource across all nodes.
type ClusterResource struct {
	ID      string  `json:"id"`
	Type    string  `json:"type"`
	VMID    int     `json:"vmid,omitempty"`
	Node    string  `json:"node,omitempty"`
	Name    string  `json:"name,omitempty"`
	Status  string  `json:"status,omitempty"`
	Storage string  `json:"storage,omitempty"`
	Disk    int64   `json:"disk,omitempty"`
	MaxDisk int64   `json:"maxdisk,omitempty"`
	Mem     int64   `json:"mem,omitempty"`
	MaxMem  int64   `json:"maxmem,omitempty"`
	CPU     float64 `json:"cpu,omitempty"`
	MaxCPU  int     `json:"maxcpu,omitempty"`
	Uptime  int64   `json:"uptime,omitempty"`
}

// Fleecing is the fleece-disks config of a backup job: PVE returns it as
// an object, e.g. {"enabled":0} (not a bare boolean).
type Fleecing struct {
	Enabled FlexBool `json:"enabled,omitempty"`
}

// PruneBackups is the retention config of a backup job: PVE returns it as
// an object with keep-* keys, e.g. {"keep-all":1}.
type PruneBackups struct {
	KeepAll     FlexInt `json:"keep-all,omitempty"`
	KeepLast    FlexInt `json:"keep-last,omitempty"`
	KeepHourly  FlexInt `json:"keep-hourly,omitempty"`
	KeepDaily   FlexInt `json:"keep-daily,omitempty"`
	KeepWeekly  FlexInt `json:"keep-weekly,omitempty"`
	KeepMonthly FlexInt `json:"keep-monthly,omitempty"`
	KeepYearly  FlexInt `json:"keep-yearly,omitempty"`
}

// BackupJob is one entry of GET /cluster/backup: a configured vzdump backup
// job (schedule) on the cluster.
type BackupJob struct {
	ID               string       `json:"id"`
	Comment          string       `json:"comment,omitempty"`
	Enabled          FlexBool     `json:"enabled,omitempty"`
	Schedule         string       `json:"schedule,omitempty"`
	Node             string       `json:"node,omitempty"`
	Mode             string       `json:"mode,omitempty"` // snapshot | suspend | stop
	Storage          string       `json:"storage,omitempty"`
	All              FlexBool     `json:"all,omitempty"` // backup all VMs/CTs
	Pool             string       `json:"pool,omitempty"`
	Protected        FlexBool     `json:"protected,omitempty"`
	VMID             string       `json:"vmid,omitempty"` // comma-separated list, empty when All
	NextRun          int64        `json:"next-run,omitempty"`
	MailNotification string       `json:"notification-mode,omitempty"`
	MailTo           string       `json:"mailto,omitempty"`
	Notify           string       `json:"notify,omitempty"`
	Compress         string       `json:"compress,omitempty"`
	BwLimit          int64        `json:"bwlimit,omitempty"`
	Fleecing         Fleecing     `json:"fleecing,omitzero"`
	Template         FlexBool     `json:"template,omitempty"`
	NotesTemplate    string       `json:"notes-template,omitempty"`
	PruneBackups     PruneBackups `json:"prune-backups,omitzero"`
	ExcludePath      string       `json:"exclude-path,omitempty"`
}

// CreateBackupJobRequest is the form body of POST /cluster/backup (and the
// updatable subset for PUT /cluster/backup/{id}). ID is the job's natural key
// (resource key for idempotency, X02). Empty/zero fields are omitted.
type CreateBackupJobRequest struct {
	ID            string `json:"id,omitempty"`
	All           bool   `json:"all,omitempty"`
	VMID          string `json:"vmid,omitempty"` // comma-separated IDs or "all"
	Storage       string `json:"storage"`
	Schedule      string `json:"schedule"`
	Mode          string `json:"mode,omitempty"` // snapshot|suspend|stop (default snapshot)
	Compress      string `json:"compress,omitempty"`
	Enabled       *bool  `json:"enabled,omitempty"`
	NotesTemplate string `json:"notes-template,omitempty"`
	Comment       string `json:"comment,omitempty"`
	Node          string `json:"node,omitempty"`
}
