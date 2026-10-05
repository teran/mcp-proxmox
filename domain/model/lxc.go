package model

// LXC is an LXC container as returned by GET /nodes/{node}/lxc.
type LXC struct {
	VMID    int    `json:"vmid"`
	Name    string `json:"name,omitempty"`
	Status  string `json:"status,omitempty"`
	Node    string `json:"node,omitempty"`
	Type    string `json:"type,omitempty"`
	Lock    string `json:"lock,omitempty"`
	Uptime  int64  `json:"uptime,omitempty"`
	Mem     int64  `json:"mem,omitempty"`
	MaxMem  int64  `json:"maxmem,omitempty"`
	Disk    int64  `json:"disk,omitempty"`
	MaxDisk int64  `json:"maxdisk,omitempty"`
	CPUs    int    `json:"cpus,omitempty"`
	Tags    string `json:"tags,omitempty"`
}

// LXCConfig is the configuration of a container from
// GET /nodes/{node}/lxc/{vmid}/config.
type LXCConfig struct {
	VMID     int               `json:"vmid"`
	Hostname string            `json:"hostname,omitempty"`
	OSType   string            `json:"ostype,omitempty"`
	Cores    FlexInt           `json:"cores,omitempty"`
	Memory   FlexInt           `json:"memory,omitempty"`
	Swap     FlexInt           `json:"swap,omitempty"`
	RootFS   string            `json:"rootfs,omitempty"`
	Networks map[string]string `json:"net,omitempty"`
	Tags     string            `json:"tags,omitempty"`
}

// LXCStatus is the current status of a container from
// GET /nodes/{node}/lxc/{vmid}/status/current.
type LXCStatus struct {
	VMID    int     `json:"vmid"`
	Status  string  `json:"status"`
	Name    string  `json:"name,omitempty"`
	Type    string  `json:"type,omitempty"`
	Uptime  int64   `json:"uptime,omitempty"`
	CPU     float64 `json:"cpu,omitempty"`
	Mem     int64   `json:"mem,omitempty"`
	MaxMem  int64   `json:"maxmem,omitempty"`
	Disk    int64   `json:"disk,omitempty"`
	MaxDisk int64   `json:"maxdisk,omitempty"`
	Swap    int64   `json:"swap,omitempty"`
	Lock    string  `json:"lock,omitempty"`
}
