package model

// Node is a Proxmox VE cluster node as returned by GET /nodes.
type Node struct {
	Node           string  `json:"node"`
	Status         string  `json:"status"`
	CPU            float64 `json:"cpu,omitempty"`
	MaxCPU         int     `json:"maxcpu,omitempty"`
	Mem            int64   `json:"mem,omitempty"`
	MaxMem         int64   `json:"maxmem,omitempty"`
	Disk           int64   `json:"disk,omitempty"`
	MaxDisk        int64   `json:"maxdisk,omitempty"`
	Uptime         int64   `json:"uptime,omitempty"`
	SSLFingerprint string  `json:"sslfingerprint,omitempty"`
	Level          string  `json:"level,omitempty"`
	ID             string  `json:"id,omitempty"`
	Type           string  `json:"type,omitempty"`
}

// SwapInfo is the swap usage of a node from GET /nodes/{node}/status.
type SwapInfo struct {
	Total int64 `json:"total,omitempty"`
	Used  int64 `json:"used,omitempty"`
	Free  int64 `json:"free,omitempty"`
}

// NodeStatus is the status of a single node from GET /nodes/{node}/status.
type NodeStatus struct {
	Node    string    `json:"node"`
	Status  string    `json:"status"`
	CPU     float64   `json:"cpu,omitempty"`
	MaxCPU  int       `json:"maxcpu,omitempty"`
	Mem     int64     `json:"mem,omitempty"`
	MaxMem  int64     `json:"maxmem,omitempty"`
	Disk    int64     `json:"disk,omitempty"`
	MaxDisk int64     `json:"maxdisk,omitempty"`
	Uptime  int64     `json:"uptime,omitempty"`
	LoadAvg []float64 `json:"loadavg,omitempty"`
	KVM     int       `json:"kvm,omitempty"` // Proxmox returns kvm as 0/1
	PVE     int       `json:"pve,omitempty"` // Proxmox returns pve as 0/1
	Swap    SwapInfo  `json:"swap,omitempty"`
	KSmem   int64     `json:"ksm,omitempty"`
	Wait    float64   `json:"wait,omitempty"`
}
