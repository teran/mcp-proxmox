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

// BackupJob is one entry of GET /cluster/backup: a configured vzdump backup
// job (schedule) on the cluster.
type BackupJob struct {
	ID               string `json:"id"`
	Comment          string `json:"comment,omitempty"`
	Enabled          bool   `json:"enabled,omitempty"`
	Schedule         string `json:"schedule,omitempty"`
	Node             string `json:"node,omitempty"`
	Mode             string `json:"mode,omitempty"` // snapshot | suspend | stop
	Storage          string `json:"storage,omitempty"`
	All              bool   `json:"all,omitempty"` // backup all VMs/CTs
	Pool             string `json:"pool,omitempty"`
	Protected        bool   `json:"protected,omitempty"`
	VMID             string `json:"vmid,omitempty"` // comma-separated list, empty when All
	NextRun          int64  `json:"next-run,omitempty"`
	MailNotification string `json:"mailnotification,omitempty"`
	MailTo           string `json:"mailto,omitempty"`
	Notify           string `json:"notify,omitempty"`
	Compress         string `json:"compress,omitempty"`
	BwLimit          int64  `json:"bwlimit,omitempty"`
	Fleecing         bool   `json:"fleecing,omitempty"`
	Template         bool   `json:"template,omitempty"`
	NotesTemplate    string `json:"notes-template,omitempty"`
	PruneBackups     string `json:"prune-backups,omitempty"`
	ExcludePath      string `json:"exclude-path,omitempty"`
}
