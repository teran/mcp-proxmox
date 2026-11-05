package model

// Storage is a cluster storage entry as returned by GET /storage.
type Storage struct {
	Storage string   `json:"storage"`
	Type    string   `json:"type"`
	Status  string   `json:"status,omitempty"`
	Content string   `json:"content,omitempty"`
	Nodes   string   `json:"nodes,omitempty"`
	Shared  int      `json:"shared,omitempty"` // Proxmox returns shared as 0/1
	Active  int      `json:"active,omitempty"` // Proxmox returns active as 0/1
	Enabled FlexBool `json:"enabled,omitempty"`
	Path    string   `json:"path,omitempty"`
}

// StorageStatus is the status of a single storage from
// GET /nodes/{node}/storage/{storage}/status.
type StorageStatus struct {
	Storage  string   `json:"storage"`
	Type     string   `json:"type"`
	Total    int64    `json:"total,omitempty"`
	Used     int64    `json:"used,omitempty"`
	Avail    int64    `json:"avail,omitempty"`
	UsedFrac float64  `json:"used_fraction,omitempty"`
	Enabled  FlexBool `json:"enabled,omitempty"`
	Active   int      `json:"active,omitempty"` // Proxmox returns active as 0/1
}

// StorageContent is a single volume/entry on a storage, as returned by
// GET /nodes/{node}/storage/{storage}/content. VolID is the storage-qualified
// volume ID (e.g. "local:iso/debian-12.iso"); the remaining fields are optional
// depending on the content type.
type StorageContent struct {
	VolID   string `json:"volid"`
	Content string `json:"content,omitempty"`
	Format  string `json:"format,omitempty"`
	Size    int64  `json:"size,omitempty"`
	CTime   int64  `json:"ctime,omitempty"`
	VMID    int    `json:"vmid,omitempty"`
}
