package model

// ServerStatus is the result of the system `status` tool. It describes the
// server itself, not a Proxmox entity: version, transport, and which backends
// are enabled. See SPEC.md §6.1.
type ServerStatus struct {
	Version   string `json:"version"`
	Transport string `json:"transport"`
	PVE       bool   `json:"pve"`
	PBS       bool   `json:"pbs"`
}
