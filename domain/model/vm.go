package model

// VM is a QEMU virtual machine as returned by GET /nodes/{node}/qemu.
type VM struct {
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
	CPU     int    `json:"cpus,omitempty"`
	Tags    string `json:"tags,omitempty"`
}

// VMConfig is the configuration of a VM from GET /nodes/{node}/qemu/{vmid}/config.
type VMConfig struct {
	VMID     int               `json:"vmid"`
	Name     string            `json:"name,omitempty"`
	Cores    int               `json:"cores,omitempty"`
	Sockets  int               `json:"sockets,omitempty"`
	Memory   int               `json:"memory,omitempty"`
	Balloon  int               `json:"balloon,omitempty"`
	OSType   string            `json:"ostype,omitempty"`
	Boot     string            `json:"boot,omitempty"`
	Template int               `json:"template,omitempty"`
	Agent    string            `json:"agent,omitempty"`
	Networks map[string]string `json:"net,omitempty"`
	Disks    map[string]string `json:"scsi,omitempty"`
	Storage  map[string]string `json:"ide,omitempty"`
	Tags     string            `json:"tags,omitempty"`
}

// VMStatus is the current status of a VM from
// GET /nodes/{node}/qemu/{vmid}/status/current.
type VMStatus struct {
	VMID      int     `json:"vmid"`
	Status    string  `json:"status"`
	Name      string  `json:"name,omitempty"`
	Node      string  `json:"node,omitempty"`
	Uptime    int64   `json:"uptime,omitempty"`
	CPU       float64 `json:"cpu,omitempty"`
	Mem       int64   `json:"mem,omitempty"`
	MaxMem    int64   `json:"maxmem,omitempty"`
	Disk      int64   `json:"disk,omitempty"`
	MaxDisk   int64   `json:"maxdisk,omitempty"`
	QMPStatus string  `json:"qmpstatus,omitempty"`
	Lock      string  `json:"lock,omitempty"`
	Agent     int     `json:"agent,omitempty"` // Proxmox returns agent as 0/1 in status
}
