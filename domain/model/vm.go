package model

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"
)

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
//
// Real PVE emits the config as a FLAT object: scalar fields (vmid, name, cores,
// memory, onboot, ...) sit at the root alongside the device keys (scsi0, ide2,
// net0, ...). The device keys are aggregated into the Networks/Disks/Storage
// maps on decode and flattened back to the root on encode so the wire shape
// stays faithful to PVE (SPEC.md §9). Numeric config fields are decoded via
// FlexInt so both JSON numbers and JSON strings ("memory":"8192") are accepted.
type VMConfig struct {
	VMID        int               `json:"vmid"`
	Name        string            `json:"name,omitempty"`
	Cores       FlexInt           `json:"cores,omitempty"`
	Sockets     FlexInt           `json:"sockets,omitempty"`
	Memory      FlexInt           `json:"memory,omitempty"`
	Balloon     FlexInt           `json:"balloon,omitempty"`
	OSType      string            `json:"ostype,omitempty"`
	Boot        string            `json:"boot,omitempty"`
	Template    FlexInt           `json:"template,omitempty"`
	OnBoot      FlexInt           `json:"onboot"` // always serialized (0 must be preserved)
	Agent       string            `json:"agent,omitempty"`
	Networks    map[string]string `json:"net,omitempty"`  // net0, net1, ...
	Disks       map[string]string `json:"scsi,omitempty"` // scsi0, scsi1, ...
	Storage     map[string]string `json:"ide,omitempty"`  // ide0, ide2, ...
	Tags        string            `json:"tags,omitempty"`
	Description string            `json:"description,omitempty"`
}

// UnmarshalJSON decodes a VMConfig from the flat PVE config form. Scalar keys
// are mapped to their fields (numbers or strings, via FlexInt); flat device
// keys prefixed scsi/ide/net are aggregated into the Disks/Storage/Networks
// maps.
func (c *VMConfig) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	setScalar := func(key string, out any) {
		if v, ok := raw[key]; ok {
			_ = json.Unmarshal(v, out)
		}
	}
	setScalar("vmid", &c.VMID)
	setScalar("name", &c.Name)
	setScalar("cores", &c.Cores)
	setScalar("sockets", &c.Sockets)
	setScalar("memory", &c.Memory)
	setScalar("balloon", &c.Balloon)
	setScalar("ostype", &c.OSType)
	setScalar("boot", &c.Boot)
	setScalar("template", &c.Template)
	setScalar("onboot", &c.OnBoot)
	setScalar("agent", &c.Agent)
	setScalar("tags", &c.Tags)
	setScalar("description", &c.Description)

	for key, v := range raw {
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			// Not a string device value (e.g. a numeric scalar already handled).
			continue
		}
		switch {
		case strings.HasPrefix(key, "scsi"):
			if c.Disks == nil {
				c.Disks = map[string]string{}
			}
			c.Disks[key] = s
		case strings.HasPrefix(key, "ide"):
			if c.Storage == nil {
				c.Storage = map[string]string{}
			}
			c.Storage[key] = s
		case strings.HasPrefix(key, "net"):
			if c.Networks == nil {
				c.Networks = map[string]string{}
			}
			c.Networks[key] = s
		}
	}

	return nil
}

// MarshalJSON serializes a VMConfig back into the flat PVE form: the scalar
// fields plus the device keys (net0/scsi0/ide2) at the root, in the stable
// order PVE emits them.
func (c VMConfig) MarshalJSON() ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteByte('{')
	first := true

	scalars := []struct {
		key  string
		val  any
		omit bool
	}{
		{"vmid", c.VMID, false},
		{"name", c.Name, true},
		{"cores", c.Cores, true},
		{"sockets", c.Sockets, true},
		{"memory", c.Memory, true},
		{"balloon", c.Balloon, true},
		{"ostype", c.OSType, true},
		{"boot", c.Boot, true},
		{"template", c.Template, true},
		{"onboot", c.OnBoot, false},
		{"agent", c.Agent, true},
	}
	for _, f := range scalars {
		if err := writeVMConfigField(&buf, &first, f.key, f.val, f.omit); err != nil {
			return nil, err
		}
	}
	if err := writeSortedMap(&buf, &first, c.Networks); err != nil {
		return nil, err
	}
	if err := writeSortedMap(&buf, &first, c.Disks); err != nil {
		return nil, err
	}
	if err := writeSortedMap(&buf, &first, c.Storage); err != nil {
		return nil, err
	}
	if err := writeVMConfigField(&buf, &first, "tags", c.Tags, true); err != nil {
		return nil, err
	}
	if err := writeVMConfigField(&buf, &first, "description", c.Description, true); err != nil {
		return nil, err
	}

	buf.WriteByte('}')

	return buf.Bytes(), nil
}

// writeVMConfigField writes a single key/value pair to buf, honoring the
// omitempty semantics via `omit`, and managing the inter-field comma.
func writeVMConfigField(buf *bytes.Buffer, first *bool, key string, val any, omit bool) error {
	if omit && isEmptyValue(val) {
		return nil
	}
	if !*first {
		buf.WriteByte(',')
	}
	*first = false
	keyBytes, err := json.Marshal(key)
	if err != nil {
		return err
	}
	valBytes, err := json.Marshal(val)
	if err != nil {
		return err
	}
	buf.Write(keyBytes)
	buf.WriteByte(':')
	buf.Write(valBytes)

	return nil
}

// writeSortedMap emits the flat device keys (net0/scsi0/ide2) of a config map
// in sorted key order for deterministic output.
func writeSortedMap(buf *bytes.Buffer, first *bool, m map[string]string) error {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		if err := writeVMConfigField(buf, first, k, m[k], false); err != nil {
			return err
		}
	}

	return nil
}

// isEmptyValue reports whether v is a zero value, matching encoding/json's
// omitempty semantics for the VMConfig scalar fields.
func isEmptyValue(v any) bool {
	switch val := v.(type) {
	case string:
		return val == ""
	case FlexInt:
		return val == 0
	case int:
		return val == 0
	case bool:
		return !val
	}

	return false
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
