package model

// Storage is a cluster storage entry as returned by GET /storage.
type Storage struct {
	Storage string `json:"storage"`
	Type    string `json:"type"`
	Status  string `json:"status,omitempty"`
	Content string `json:"content,omitempty"`
	Nodes   string `json:"nodes,omitempty"`
	Shared  int    `json:"shared,omitempty"` // Proxmox returns shared as 0/1
	Active  int    `json:"active,omitempty"` // Proxmox returns active as 0/1
	Enabled bool   `json:"enabled,omitempty"`
	Path    string `json:"path,omitempty"`
}

// StorageStatus is the status of a single storage from
// GET /nodes/{node}/storage/{storage}/status.
type StorageStatus struct {
	Storage  string  `json:"storage"`
	Type     string  `json:"type"`
	Total    int64   `json:"total,omitempty"`
	Used     int64   `json:"used,omitempty"`
	Avail    int64   `json:"avail,omitempty"`
	UsedFrac float64 `json:"used_fraction,omitempty"`
	Enabled  bool    `json:"enabled,omitempty"`
	Active   int     `json:"active,omitempty"` // Proxmox returns active as 0/1
}
