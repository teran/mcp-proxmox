package model

// NetworkInterface is a network interface of a node from
// GET /nodes/{node}/network. The iface attribute carries the interface name.
type NetworkInterface struct {
	Iface       string `json:"iface"`
	Type        string `json:"type,omitempty"`
	Bridge      string `json:"bridge,omitempty"`
	BridgePorts string `json:"bridge_ports,omitempty"`
	Address     string `json:"address,omitempty"`
	Netmask     string `json:"netmask,omitempty"`
	Gateway     string `json:"gateway,omitempty"`
	Method      string `json:"method,omitempty"`
	VLANID      int    `json:"vlan-id,omitempty"`
	Active      int    `json:"active,omitempty"` // Proxmox returns active as 0/1
	Status      string `json:"status,omitempty"`
	Comments    string `json:"comments,omitempty"`
}
