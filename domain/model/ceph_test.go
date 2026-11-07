package model

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// cephEntityRoundTripCases holds the fully-typed (stable-scalar) Ceph entities
// and their populated values. Each is marshalled then unmarshalled and compared
// with reflect.DeepEqual — a broken json tag, a dropped field, or a wrong type
// breaks the round trip (survives mutation). Entities that carry
// version-dependent passthrough map[string]any fields are covered separately by
// the decode tests below (their exact inner shape must never be asserted).
var cephEntityRoundTripCases = []struct {
	name string
	in   any
}{
	{
		name: "pool",
		in: CephPool{
			ID: 1, PoolName: "rbd", Type: "replicated",
			Size: 3, MinSize: 2, PGNum: 128, PGAutoscaleMode: "on",
		},
	},
	{
		name: "osd",
		in: CephOSD{
			ID: 0, Type: "osd", Node: "pve1", DeviceClass: "hdd",
			Name: "osd.0", Status: "up", State: "active",
		},
	},
	{
		name: "mon",
		in: CephMon{
			ID: 1, Type: "mon", Name: "pve1", Node: "pve1",
			Addr: "10.0.0.1:6789", External: 0,
		},
	},
	{
		name: "mgr",
		in:   CephMgr{ID: 1, Type: "mgr", Name: "pve1", Node: "pve1", Status: "active"},
	},
	{
		name: "crush-rule",
		in: CephCrushRule{
			ID: 0, RuleName: "replicated_rule", Type: "replicated",
			MinSize: 1, MaxSize: 10, Steps: []string{"take default", "chooseleaf firstn 0 type host"},
		},
	},
	{
		name: "config-entry",
		in:   CephConfigEntry{Name: "mon_allow_pool_delete", Value: "true"},
	},
	{
		name: "log-entry",
		in:   CephLogEntry{Time: "2026-01-01T00:00:00Z", Level: "info", Message: "boot"},
	},
	{
		name: "flag",
		in:   CephFlag{ID: 1, Name: "noout", Value: "0", Enabled: 0},
	},
}

func TestCephEntityRoundTrip(t *testing.T) {
	for _, tt := range cephEntityRoundTripCases {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.in)
			require.NoError(t, err)

			typ := reflect.TypeOf(tt.in)
			rv := reflect.New(typ)
			require.NoError(t, json.Unmarshal(b, rv.Interface()))
			require.True(t, reflect.DeepEqual(rv.Elem().Interface(), tt.in),
				"round-trip mismatch\n got: %#v\nwant: %#v", rv.Elem().Interface(), tt.in)
		})
	}
}

// TestCephLogEntryJSONTags pins the exact wire keys of CephLogEntry: PVE emits
// log entries with the short t/n/m keys (SPEC contract), and a regression to the
// verbose names would silently drop the fields on decode.
func TestCephLogEntryJSONTags(t *testing.T) {
	in := CephLogEntry{Time: "t1", Level: "l", Message: "m"}
	b, err := json.Marshal(in)
	require.NoError(t, err)
	assert.JSONEq(t, `{"t":"t1","n":"l","m":"m"}`, string(b))
}

// TestCephStatusPassthroughMaps decodes real-shaped PVE Ceph API payloads and
// asserts the STABLE top-level scalars decode to their typed fields while the
// version-dependent nested blobs (pgmap/osdmap/monmap/fsmap/mgrmap) survive as
// non-nil map[string]any passthroughs. Their exact inner shape is deliberately
// NOT asserted (SPEC contract) — only non-nil presence and a spot-check key.
func TestCephStatusPassthroughMaps(t *testing.T) {
	t.Run("cluster-status", func(t *testing.T) {
		payload := `{"data":{
			"status":"HEALTH_OK",
			"quorum":[1,2,3],
			"quorum_names":["pve1","pve2","pve3"],
			"pgmap":{"num_pgs":100,"bytes_total":1000},
			"osdmap":{"num_osds":6},
			"monmap":{"mons":[]},
			"fsmap":{"by_rank":[]},
			"mgrmap":{"active_name":"pve1"}
		}}`
		var env struct {
			Data CephClusterStatus `json:"data"`
		}
		require.NoError(t, json.Unmarshal([]byte(payload), &env))

		s := env.Data
		assert.Equal(t, "HEALTH_OK", s.Status)
		assert.Equal(t, []int{1, 2, 3}, s.Quorum)
		assert.Equal(t, []string{"pve1", "pve2", "pve3"}, s.QuorumNames)
		// passthrough maps present, never exact inner shape.
		assert.NotNil(t, s.Health)
		assert.NotNil(t, s.PGMap)
		assert.NotNil(t, s.OSDMap)
		assert.NotNil(t, s.MonMap)
		assert.NotNil(t, s.FsMap)
		assert.NotNil(t, s.MgrMap)
		assert.Equal(t, float64(100), s.PGMap["num_pgs"])
	})

	t.Run("node-status", func(t *testing.T) {
		payload := `{"data":{
			"status":"HEALTH_OK",
			"quorum":[1],
			"quorum_names":["pve1"],
			"pgmap":{"num_pgs":64},
			"osdmap":{"num_osds":3},
			"monmap":{"mons":[]},
			"fsmap":{"by_rank":[]},
			"mgrmap":{"active_name":"pve1"}
		}}`
		var env struct {
			Data CephNodeStatus `json:"data"`
		}
		require.NoError(t, json.Unmarshal([]byte(payload), &env))

		s := env.Data
		assert.Equal(t, "HEALTH_OK", s.Status)
		assert.Equal(t, []int{1}, s.Quorum)
		assert.NotNil(t, s.PGMap)
		assert.NotNil(t, s.OSDMap)
		assert.NotNil(t, s.MonMap)
		assert.NotNil(t, s.FsMap)
		assert.NotNil(t, s.MgrMap)
	})
}

// TestCephCrushMapDecode verifies CephCrushMap: typed Rules plus passthrough
// Tunables/Types/Buckets survive as non-nil; Rules are bound to the "rule" json
// key.
func TestCephCrushMapDecode(t *testing.T) {
	payload := `{"data":{
		"rule":[
			{"id":0,"name":"replicated_rule","type":"replicated","min_size":1,"max_size":10,"steps":["take default"]}
		],
		"tunables":{"choose_local_tries":50},
		"types":[{"id":1,"name":"host"}],
		"buckets":[{"id":1,"name":"default"}]
	}}`
	var env struct {
		Data CephCrushMap `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(payload), &env))

	crushMap := env.Data
	require.Len(t, crushMap.Rules, 1)
	assert.Equal(t, 0, crushMap.Rules[0].ID)
	assert.Equal(t, "replicated_rule", crushMap.Rules[0].RuleName)
	assert.Equal(t, []string{"take default"}, crushMap.Rules[0].Steps)
	assert.NotNil(t, crushMap.Tunables)
	assert.NotNil(t, crushMap.Types)
	assert.NotNil(t, crushMap.Buckets)
}

// TestCephMetadataExtraJSONTag pins that the version-dependent Extra map on the
// metadata entities is excluded from JSON (json:"-") so only the stable
// scalars are serialized; a developer regression that includes Extra would
// break this and leak unstable fields.
func TestCephMetadataExtraJSONTag(t *testing.T) {
	cluster := CephClusterMetadata{
		Fsid:        "0123",
		CephVersion: "17.2.7",
		Extra:       map[string]any{"version": "ceph version 17.2.7"},
	}
	b, err := json.Marshal(cluster)
	require.NoError(t, err)
	assert.JSONEq(t, `{"fsid":"0123","ceph_version":"17.2.7"}`, string(b))

	node := CephNodeMetadata{Extra: map[string]any{"arch": "amd64"}}
	b, err = json.Marshal(node)
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(b))
}

// TestCephMetadataDecode verifies the stable scalars of the metadata entities
// decode from the real PVE payload shape while Extra is ignored on decode (it
// is the version-dependent passthrough seam the gateway fills).
func TestCephMetadataDecode(t *testing.T) {
	var cluster struct {
		Data CephClusterMetadata `json:"data"`
	}
	require.NoError(t, json.Unmarshal(
		[]byte(`{"data":{"fsid":"0123","ceph_version":"17.2.7"}}`), &cluster))
	assert.Equal(t, "0123", cluster.Data.Fsid)
	assert.Equal(t, "17.2.7", cluster.Data.CephVersion)
}

// TestCephClusterMetadataUnmarshalEdge covers the branches of the custom
// CephClusterMetadata.UnmarshalJSON that a happy-path decode does not reach:
// non-string known keys (ignored), a payload of only unknown keys (routed to
// Extra), a payload of only known keys (Extra stays nil), and invalid JSON.
func TestCephClusterMetadataUnmarshalEdge(t *testing.T) {
	t.Run("non-string known keys are ignored", func(t *testing.T) {
		var m CephClusterMetadata
		require.NoError(t, json.Unmarshal(
			[]byte(`{"fsid":123,"ceph_version":{"major":17}}`), &m))
		assert.Empty(t, m.Fsid)
		assert.Empty(t, m.CephVersion)
		// The non-string known keys must NOT leak into Extra.
		assert.Nil(t, m.Extra)
	})

	t.Run("unknown keys routed to Extra", func(t *testing.T) {
		var m CephClusterMetadata
		require.NoError(t, json.Unmarshal(
			[]byte(`{"fsid":"abc","kernel_version":"5.15","arch":"amd64"}`), &m))
		assert.Equal(t, "abc", m.Fsid)
		require.NotNil(t, m.Extra)
		assert.Equal(t, "5.15", m.Extra["kernel_version"])
		assert.Equal(t, "amd64", m.Extra["arch"])
		assert.NotContains(t, m.Extra, "fsid")
	})

	t.Run("only known keys leaves Extra nil", func(t *testing.T) {
		var m CephClusterMetadata
		require.NoError(t, json.Unmarshal(
			[]byte(`{"fsid":"abc","ceph_version":"17.2.7"}`), &m))
		assert.Equal(t, "abc", m.Fsid)
		assert.Equal(t, "17.2.7", m.CephVersion)
		assert.Nil(t, m.Extra)
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		var m CephClusterMetadata
		require.Error(t, json.Unmarshal([]byte(`not json`), &m))
	})
}

// TestCephNodeMetadataUnmarshal covers the fully-dynamic per-node metadata
// decode: the entire payload is captured into Extra, an empty payload leaves
// Extra nil, and invalid JSON returns an error.
func TestCephNodeMetadataUnmarshal(t *testing.T) {
	t.Run("payload captured into Extra", func(t *testing.T) {
		var m CephNodeMetadata
		require.NoError(t, json.Unmarshal(
			[]byte(`{"arch":"amd64","version":"17.2.7","os":"pve"}`), &m))
		require.NotNil(t, m.Extra)
		assert.Equal(t, "amd64", m.Extra["arch"])
		assert.Equal(t, "17.2.7", m.Extra["version"])
	})

	t.Run("empty payload leaves Extra nil", func(t *testing.T) {
		var m CephNodeMetadata
		require.NoError(t, json.Unmarshal([]byte(`{}`), &m))
		assert.Nil(t, m.Extra)
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		var m CephNodeMetadata
		require.Error(t, json.Unmarshal([]byte(`{`), &m))
	})
}

// TestCephStatusUnmarshalEdge covers the error/absent branches of the custom
// CephClusterStatus / CephNodeStatus UnmarshalJSON: a payload with no
// passthrough maps still yields non-nil maps, and invalid JSON propagates the
// decode error.
func TestCephStatusUnmarshalEdge(t *testing.T) {
	t.Run("absent maps become non-nil", func(t *testing.T) {
		payload := `{"data":{"status":"HEALTH_WARN"}}`
		var env struct {
			Data CephClusterStatus `json:"data"`
		}
		require.NoError(t, json.Unmarshal([]byte(payload), &env))
		assert.Equal(t, "HEALTH_WARN", env.Data.Status)
		assert.NotNil(t, env.Data.Health)
		assert.NotNil(t, env.Data.PGMap)
		assert.NotNil(t, env.Data.OSDMap)
		assert.NotNil(t, env.Data.MonMap)
		assert.NotNil(t, env.Data.FsMap)
		assert.NotNil(t, env.Data.MgrMap)
	})

	t.Run("invalid JSON returns error", func(t *testing.T) {
		var s CephClusterStatus
		require.Error(t, json.Unmarshal([]byte(`{`), &s))
		var ns CephNodeStatus
		require.Error(t, json.Unmarshal([]byte(`{`), &ns))
	})
}

// TestNonNilMap pins the nonNilMap helper's passthrough-vs-replacement behavior.
func TestNonNilMap(t *testing.T) {
	assert.NotNil(t, nonNilMap(nil))
	// A non-nil map is passed through unchanged (never replaced by an empty map).
	out := nonNilMap(map[string]any{"k": "v"})
	assert.Equal(t, map[string]any{"k": "v"}, out)
}
