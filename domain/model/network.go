package model

import "encoding/json"

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

// UnmarshalJSON decodes a network interface, tolerating the `vlan-id` field
// whether PVE emits it as a JSON number (10) or a JSON string ("100"). The
// field itself stays an int, so consumers read the numeric VLAN ID directly.
func (n *NetworkInterface) UnmarshalJSON(data []byte) error {
	type alias NetworkInterface
	var aux struct {
		*alias

		VLANID json.RawMessage `json:"vlan-id"`
	}
	aux.alias = (*alias)(n)
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if len(aux.VLANID) == 0 || string(aux.VLANID) == "null" {
		return nil
	}
	var f FlexInt
	if err := json.Unmarshal(aux.VLANID, &f); err != nil {
		return err
	}
	n.VLANID = int(f)

	return nil
}
