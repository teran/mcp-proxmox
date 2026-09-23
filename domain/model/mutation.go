package model

// CreateVMRequest is the form body of POST /nodes/{node}/qemu. The json tags
// mirror the real Proxmox VE create-VM form parameters; empty/zero fields are
// omitted (omitempty) so the adapter only sends what was requested. The VMID is
// optional — when omitted PVE assigns the next free ID (which the adapter
// returns in the response).
type CreateVMRequest struct {
	VMID        int    `json:"vmid,omitempty"`
	Name        string `json:"name,omitempty"`
	Cores       int    `json:"cores,omitempty"`
	Memory      int    `json:"memory,omitempty"`
	Sockets     int    `json:"sockets,omitempty"`
	Ostype      string `json:"ostype,omitempty"`
	Net0        string `json:"net0,omitempty"`
	Scsi0       string `json:"scsi0,omitempty"`
	Ide2        string `json:"ide2,omitempty"`
	Boot        string `json:"boot,omitempty"`
	Balloon     int    `json:"balloon,omitempty"`
	Description string `json:"description,omitempty"`
	Tags        string `json:"tags,omitempty"`
	Agent       int    `json:"agent,omitempty"`
	OnBoot      int    `json:"onboot,omitempty"`
	CPU         string `json:"cpu,omitempty"`
	Machine     string `json:"machine,omitempty"`
	VGA         string `json:"vga,omitempty"`
}

// ResizeVMRequest is the form body of PUT /nodes/{node}/qemu/{vmid}/resize.
// Disk names the virtual disk (e.g. "scsi0") and SizeGB the new size in GiB,
// with the sign indicating grow (+NG) or shrink (-NG).
type ResizeVMRequest struct {
	Disk   string `json:"disk"`
	SizeGB int    `json:"size_gb,omitempty"`
}

// MigrateVMRequest is the form body of POST /nodes/{node}/qemu/{vmid}/migrate.
type MigrateVMRequest struct {
	Target         string `json:"target"`
	Online         bool   `json:"online,omitempty"`
	WithLocalDisks bool   `json:"with-local-disks,omitempty"`
}

// HAResourceRequest is the form body of POST /cluster/ha/resources. SID is the
// HA resource ID (e.g. "vm:100"), Type the resource type ("vm" | "ct"),
// Nodes the ordered list of preferred nodes, and Comment an optional note.
type HAResourceRequest struct {
	SID     string   `json:"sid"`
	Type    string   `json:"type"`
	Nodes   []string `json:"nodes,omitempty"`
	Comment string   `json:"comment,omitempty"`
}

// VMBackupRequest is the form body of POST /nodes/{node}/vzdump. Mode is one of
// snapshot|suspend|stop; Storage is the backup storage; VMID the VM to back up.
type VMBackupRequest struct {
	VMID          int    `json:"vmid"`
	Storage       string `json:"storage"`
	Mode          string `json:"mode"`
	NotesTemplate string `json:"notes-template,omitempty"`
	Compress      string `json:"compress,omitempty"`
}

// CloneVMRequest is the form body of POST /nodes/{node}/qemu/{vmid}/clone.
// NewID is required (the new VMID); the other fields are optional clone
// overrides.
type CloneVMRequest struct {
	NewID       int    `json:"newid"`
	Name        string `json:"name,omitempty"`
	Full        bool   `json:"full,omitempty"`
	Storage     string `json:"storage,omitempty"`
	Pool        string `json:"pool,omitempty"`
	Description string `json:"description,omitempty"`
	Format      string `json:"format,omitempty"`
	Snapname    string `json:"snapname,omitempty"`
}

// PBSSyncRequest is the form body of POST /admin/datastore/{store}/sync. It
// describes a one-off sync job that pulls from a remote PBS datastore.
type PBSSyncRequest struct {
	Remote         string `json:"remote,omitempty"`
	RemoteStore    string `json:"remote-store,omitempty"`
	Owner          string `json:"owner,omitempty"`
	MaxFiles       int    `json:"maxfiles,omitempty"`
	RemoveVanished bool   `json:"remove-vanished,omitempty"`
	RateIn         int    `json:"ratein,omitempty"`
	RateOut        int    `json:"rateout,omitempty"`
	SkipLost       bool   `json:"skip-lost,omitempty"`
	NotifyUser     string `json:"notify-user,omitempty"`
}
