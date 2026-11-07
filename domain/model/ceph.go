package model

import "encoding/json"

// CephClusterStatus is the status of a PVE-managed Ceph cluster from
// GET /cluster/ceph/status. The deeply-nested maps (health, pgmap, osdmap, ...)
// are intentionally opaque: their shape is version-dependent, so they are
// exposed as arbitrary JSON objects rather than a brittle typed struct. The
// maps are guaranteed non-nil after decode (UnmarshalJSON initializes absent
// ones to empty maps) so callers can read into them without nil checks.
type CephClusterStatus struct {
	Status      string         `json:"status,omitempty"`
	Quorum      []int          `json:"quorum,omitempty"`
	QuorumNames []string       `json:"quorum_names,omitempty"`
	Health      map[string]any `json:"health,omitempty"`
	PGMap       map[string]any `json:"pgmap,omitempty"`
	OSDMap      map[string]any `json:"osdmap,omitempty"`
	MonMap      map[string]any `json:"monmap,omitempty"`
	FsMap       map[string]any `json:"fsmap,omitempty"`
	MgrMap      map[string]any `json:"mgrmap,omitempty"`
}

// UnmarshalJSON decodes the status payload and initializes any passthrough map
// that is absent from the payload to an empty (non-nil) map, so consumers never
// hit a nil map on the stable passthrough seam.
func (s *CephClusterStatus) UnmarshalJSON(b []byte) error {
	type alias CephClusterStatus
	var a alias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	a.Health = nonNilMap(a.Health)
	a.PGMap = nonNilMap(a.PGMap)
	a.OSDMap = nonNilMap(a.OSDMap)
	a.MonMap = nonNilMap(a.MonMap)
	a.FsMap = nonNilMap(a.FsMap)
	a.MgrMap = nonNilMap(a.MgrMap)
	*s = CephClusterStatus(a)

	return nil
}

// nonNilMap returns m, or an empty non-nil map when m is nil.
func nonNilMap(m map[string]any) map[string]any {
	if m == nil {
		return map[string]any{}
	}

	return m
}

// CephNodeStatus is the status of a single node's local Ceph daemons from
// GET /nodes/{node}/ceph/status. It has the same shape as CephClusterStatus,
// including the non-nil passthrough-map guarantee.
type CephNodeStatus struct {
	Status      string         `json:"status,omitempty"`
	Quorum      []int          `json:"quorum,omitempty"`
	QuorumNames []string       `json:"quorum_names,omitempty"`
	Health      map[string]any `json:"health,omitempty"`
	PGMap       map[string]any `json:"pgmap,omitempty"`
	OSDMap      map[string]any `json:"osdmap,omitempty"`
	MonMap      map[string]any `json:"monmap,omitempty"`
	FsMap       map[string]any `json:"fsmap,omitempty"`
	MgrMap      map[string]any `json:"mgrmap,omitempty"`
}

// UnmarshalJSON decodes the status payload and initializes any passthrough map
// that is absent from the payload to an empty (non-nil) map.
func (s *CephNodeStatus) UnmarshalJSON(b []byte) error {
	type alias CephNodeStatus
	var a alias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	a.Health = nonNilMap(a.Health)
	a.PGMap = nonNilMap(a.PGMap)
	a.OSDMap = nonNilMap(a.OSDMap)
	a.MonMap = nonNilMap(a.MonMap)
	a.FsMap = nonNilMap(a.FsMap)
	a.MgrMap = nonNilMap(a.MgrMap)
	*s = CephNodeStatus(a)

	return nil
}

// CephPool is one entry of GET /nodes/{node}/ceph/pool.
type CephPool struct {
	ID              int    `json:"id"`
	PoolName        string `json:"pool_name"`
	Type            string `json:"type,omitempty"`
	Size            int    `json:"size,omitempty"`
	MinSize         int    `json:"min_size,omitempty"`
	PGNum           int    `json:"pg_num,omitempty"`
	PGAutoscaleMode string `json:"pg_autoscale_mode,omitempty"`
}

// CephOSD is one entry of GET /nodes/{node}/ceph/osd.
type CephOSD struct {
	ID          int    `json:"id"`
	Type        string `json:"type,omitempty"`
	Node        string `json:"node,omitempty"`
	DeviceClass string `json:"device_class,omitempty"`
	Name        string `json:"name,omitempty"`
	Status      string `json:"status,omitempty"`
	State       string `json:"state,omitempty"`
}

// CephMon is one entry of GET /nodes/{node}/ceph/mon.
type CephMon struct {
	ID       int    `json:"id"`
	Type     string `json:"type,omitempty"`
	Name     string `json:"name,omitempty"`
	Node     string `json:"node,omitempty"`
	Addr     string `json:"addr,omitempty"`
	External int    `json:"external,omitempty"`
}

// CephMgr is one entry of GET /nodes/{node}/ceph/mgr.
type CephMgr struct {
	ID     int    `json:"id"`
	Type   string `json:"type,omitempty"`
	Name   string `json:"name,omitempty"`
	Node   string `json:"node,omitempty"`
	Status string `json:"status,omitempty"`
}

// CephCrushRule is one entry of GET /nodes/{node}/ceph/rules. The wire key for
// the rule name is `name` (the real PVE payload field).
type CephCrushRule struct {
	ID       int      `json:"id"`
	RuleName string   `json:"name"`
	Type     string   `json:"type,omitempty"`
	MinSize  int      `json:"min_size,omitempty"`
	MaxSize  int      `json:"max_size,omitempty"`
	Steps    []string `json:"steps,omitempty"`
}

// CephCrushMap is the CRUSH map of a node from GET /nodes/{node}/ceph/crush.
// Tunables is an object; Types and Buckets are arrays of {id,name}-style
// objects as returned by PVE.
type CephCrushMap struct {
	Rules    []CephCrushRule  `json:"rule,omitempty"`
	Tunables map[string]any   `json:"tunables,omitempty"`
	Types    []map[string]any `json:"types,omitempty"`
	Buckets  []map[string]any `json:"buckets,omitempty"`
}

// CephFlag is one entry of GET /nodes/{node}/ceph/flags. The Value is
// version-dependent, so it is exposed as any.
type CephFlag struct {
	ID      int    `json:"id"`
	Name    string `json:"name,omitempty"`
	Value   any    `json:"value,omitempty"`
	Enabled int    `json:"enabled,omitempty"`
}

// CephConfigEntry is one key/value config option from GET /cluster/ceph/config
// (or /nodes/{node}/ceph/config / configdb).
type CephConfigEntry struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

// CephLogEntry is one line of GET /cluster/ceph/log. The short keys mirror the
// real PVE payload (t = timestamp, n = level, m = message).
type CephLogEntry struct {
	Time    string `json:"t,omitempty"`
	Level   string `json:"n,omitempty"`
	Message string `json:"m,omitempty"`
}

// CephClusterMetadata is the cluster metadata from GET /cluster/ceph/metadata.
// The known fields (fsid, ceph_version) are captured explicitly; every other
// returned key is captured in Extra via a custom UnmarshalJSON. Extra is
// excluded from the wire representation (json:"-") so it never round-trips.
type CephClusterMetadata struct {
	Fsid        string         `json:"fsid,omitempty"`
	CephVersion string         `json:"ceph_version,omitempty"`
	Extra       map[string]any `json:"-"`
}

// UnmarshalJSON captures the known metadata fields (fsid, ceph_version) and
// routes every other returned key into Extra, so no payload field is lost.
func (m *CephClusterMetadata) UnmarshalJSON(b []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if v, ok := raw["fsid"].(string); ok {
		m.Fsid = v
	}
	if v, ok := raw["ceph_version"].(string); ok {
		m.CephVersion = v
	}
	extra := make(map[string]any, len(raw))
	for k, v := range raw {
		if k == "fsid" || k == "ceph_version" {
			continue
		}
		extra[k] = v
	}
	if len(extra) > 0 {
		m.Extra = extra
	}

	return nil
}

// CephNodeMetadata is the per-node metadata from GET /nodes/{node}/ceph/metadata.
// It is fully dynamic (the payload varies by node), so every returned key is
// captured in Extra via a custom UnmarshalJSON. Extra is excluded from the wire
// representation (json:"-") so it never round-trips.
type CephNodeMetadata struct {
	Extra map[string]any `json:"-"`
}

// UnmarshalJSON captures the entire dynamic node-metadata payload into Extra.
func (m *CephNodeMetadata) UnmarshalJSON(b []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if len(raw) > 0 {
		m.Extra = raw
	}

	return nil
}

// CephCrushRuleCreateRequest is the form body of POST /nodes/{node}/ceph/rules.
type CephCrushRuleCreateRequest struct {
	Name    string `json:"name,omitempty"`
	Type    string `json:"type,omitempty"`
	MinSize int    `json:"min_size,omitempty"`
	MaxSize int    `json:"max_size,omitempty"`
	Root    string `json:"root,omitempty"`
	OSD     string `json:"osd,omitempty"`
}

// CephPoolCreateRequest is the form body of POST /nodes/{node}/ceph/pool.
type CephPoolCreateRequest struct {
	Name            string `json:"name,omitempty"`
	Size            int    `json:"size,omitempty"`
	MinSize         int    `json:"min_size,omitempty"`
	PGNum           int    `json:"pg_num,omitempty"`
	PGAutoscaleMode string `json:"pg_autoscale_mode,omitempty"`
}

// CephPoolUpdateRequest is the form body of PUT /nodes/{node}/ceph/pool/{name}.
type CephPoolUpdateRequest struct {
	Size            int    `json:"size,omitempty"`
	MinSize         int    `json:"min_size,omitempty"`
	PGNum           int    `json:"pg_num,omitempty"`
	PGAutoscaleMode string `json:"pg_autoscale_mode,omitempty"`
}

// CephOSDCreateRequest is the form body of POST /nodes/{node}/ceph/osd.
type CephOSDCreateRequest struct {
	OSD string `json:"osd,omitempty"`
}

// CephMonCreateRequest is the form body of POST /nodes/{node}/ceph/mon.
type CephMonCreateRequest struct {
	MonAddr string `json:"mon_addr,omitempty"`
}

// CephMgrCreateRequest is the form body of POST /nodes/{node}/ceph/mgr.
type CephMgrCreateRequest struct {
	ID string `json:"id,omitempty"`
}

// CephFlagsSetRequest is the form body of PUT /nodes/{node}/ceph/flags. The
// flags are sent as individual form fields (one per key), carried in Values.
type CephFlagsSetRequest struct {
	Values map[string]string `json:"-"`
}

// CephConfigSetRequest is the form body of PUT /nodes/{node}/ceph/config.
type CephConfigSetRequest struct {
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

// CephInitRequest is the form body of POST /nodes/{node}/ceph/init.
type CephInitRequest struct {
	Network        string `json:"network,omitempty"`
	ClusterNetwork string `json:"cluster_network,omitempty"`
}

// CephDestroyRequest is the form body of DELETE /nodes/{node}/ceph/destroy.
type CephDestroyRequest struct {
	Force bool `json:"force,omitempty"`
}
