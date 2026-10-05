package model

// Snapshot is a QEMU VM / LXC container snapshot as returned by
// GET /nodes/{node}/qemu/{vmid}/snapshot (or the LXC equivalent). Snaptime is a
// Unix timestamp (seconds). VMState is 1 when the snapshot includes the guest
// RAM state (QEMU only).
type Snapshot struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Snaptime    int64  `json:"snaptime,omitempty"`
	VMState     int    `json:"vmstate,omitempty"`
	Parent      string `json:"parent,omitempty"`
	Type        string `json:"type,omitempty"`
	Ram         int64  `json:"ram,omitempty"`
	Disk        int64  `json:"disk,omitempty"`
}

// SnapshotCreateRequest is the form body of
// POST /nodes/{node}/qemu/{vmid}/snapshot (or the LXC equivalent). Snapname is
// required; VMState captures the guest RAM state (QEMU only).
type SnapshotCreateRequest struct {
	Snapname    string `json:"snapname"`
	VMState     bool   `json:"vmstate,omitempty"`
	Description string `json:"description,omitempty"`
}
