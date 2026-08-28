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
	Local  bool   `json:"local,omitempty"`
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
