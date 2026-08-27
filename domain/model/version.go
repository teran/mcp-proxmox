package model

// PVEVersion is the Proxmox VE version info from GET /version.
type PVEVersion struct {
	Version  string `json:"version"`
	Release  string `json:"release"`
	RepoID   string `json:"repoid,omitempty"`
	Keyboard string `json:"keyboard,omitempty"`
}

// PBSVersion is the Proxmox Backup Server version info from GET /version.
type PBSVersion struct {
	Version string `json:"version"`
	Release string `json:"release"`
	RepoID  string `json:"repoid,omitempty"`
}
