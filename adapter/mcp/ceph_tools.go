package mcp

import (
	"context"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/model"
)

// cephNodeIn identifies a node (ceph_node_status, ceph_pool_list, ...).
type cephNodeIn struct {
	Node string `json:"node" jsonschema:"PVE node name"`
}

// cephClusterLogIn caps the Ceph cluster log (ceph_cluster_log).
type cephClusterLogIn struct {
	Limit int `json:"limit,omitempty" jsonschema:"Maximum number of log lines"`
}

// cephRuleCreateIn is the input of ceph_rule_create.
type cephRuleCreateIn struct {
	Node    string `json:"node" jsonschema:"PVE node name"`
	Name    string `json:"name" jsonschema:"CRUSH rule name"`
	Type    string `json:"type" jsonschema:"Rule type: replicated | erasure"`
	MinSize int    `json:"min_size,omitempty" jsonschema:"Minimum replica count"`
	MaxSize int    `json:"max_size,omitempty" jsonschema:"Maximum replica count"`
	Root    string `json:"root,omitempty" jsonschema:"CRUSH root bucket"`
	OSD     string `json:"osd,omitempty" jsonschema:"OSD names (comma-separated)"`
}

// cephRuleDeleteIn is the input of ceph_rule_delete.
type cephRuleDeleteIn struct {
	Node string `json:"node" jsonschema:"PVE node name"`
	Name string `json:"name" jsonschema:"CRUSH rule name to delete"`
}

// cephPoolCreateIn is the input of ceph_pool_create.
type cephPoolCreateIn struct {
	Node            string `json:"node" jsonschema:"PVE node name"`
	Name            string `json:"name" jsonschema:"Pool name"`
	Size            int    `json:"size,omitempty" jsonschema:"Replication factor (size)"`
	MinSize         int    `json:"min_size,omitempty" jsonschema:"Minimum replica count for I/O"`
	PGNum           int    `json:"pg_num,omitempty" jsonschema:"Number of placement groups"`
	PGAutoscaleMode string `json:"pg_autoscale_mode,omitempty" jsonschema:"PG autoscaling mode: on | off | warn"`
}

// cephPoolUpdateIn is the input of ceph_pool_update.
type cephPoolUpdateIn struct {
	Node            string `json:"node" jsonschema:"PVE node name"`
	Name            string `json:"name" jsonschema:"Pool name to update"`
	Size            int    `json:"size,omitempty" jsonschema:"Replication factor (size)"`
	MinSize         int    `json:"min_size,omitempty" jsonschema:"Minimum replica count for I/O"`
	PGNum           int    `json:"pg_num,omitempty" jsonschema:"Number of placement groups"`
	PGAutoscaleMode string `json:"pg_autoscale_mode,omitempty" jsonschema:"PG autoscaling mode: on | off | warn"`
}

// cephPoolDeleteIn is the input of ceph_pool_delete.
type cephPoolDeleteIn struct {
	Node string `json:"node" jsonschema:"PVE node name"`
	Name string `json:"name" jsonschema:"Pool name to delete"`
}

// cephOSDCreateIn is the input of ceph_osd_create.
type cephOSDCreateIn struct {
	Node string `json:"node" jsonschema:"PVE node name"`
	OSD  string `json:"osd" jsonschema:"OSD specification (e.g. /dev/sdb)"`
}

// cephOSDDeleteIn is the input of ceph_osd_delete.
type cephOSDDeleteIn struct {
	Node  string `json:"node" jsonschema:"PVE node name"`
	OSDID int    `json:"osdid" jsonschema:"OSD numeric ID to delete"`
}

// cephMonCreateIn is the input of ceph_mon_create.
type cephMonCreateIn struct {
	Node    string `json:"node" jsonschema:"PVE node name"`
	MonAddr string `json:"mon_addr" jsonschema:"Monitor address (host:port)"`
}

// cephMonDeleteIn is the input of ceph_mon_delete.
type cephMonDeleteIn struct {
	Node  string `json:"node" jsonschema:"PVE node name"`
	MonID string `json:"monid" jsonschema:"Monitor ID to delete"`
}

// cephMgrCreateIn is the input of ceph_mgr_create.
type cephMgrCreateIn struct {
	Node string `json:"node" jsonschema:"PVE node name"`
	ID   string `json:"id" jsonschema:"Manager ID"`
}

// cephFlagsSetIn is the input of ceph_flags_set.
type cephFlagsSetIn struct {
	Node   string            `json:"node" jsonschema:"PVE node name"`
	Values map[string]string `json:"values" jsonschema:"Ceph flags to set (e.g. {\"noout\":\"true\"})"`
}

// cephConfigSetIn is the input of ceph_config_set.
type cephConfigSetIn struct {
	Node  string `json:"node" jsonschema:"PVE node name"`
	Name  string `json:"name" jsonschema:"Config option name"`
	Value string `json:"value" jsonschema:"Config option value"`
}

// cephInitIn is the input of ceph_init.
type cephInitIn struct {
	Node           string `json:"node" jsonschema:"PVE node name"`
	Network        string `json:"network,omitempty" jsonschema:"Ceph public network CIDR"`
	ClusterNetwork string `json:"cluster_network,omitempty" jsonschema:"Ceph cluster network CIDR"`
}

// cephDestroyIn is the input of ceph_destroy.
type cephDestroyIn struct {
	Node  string `json:"node" jsonschema:"PVE node name"`
	Force bool   `json:"force,omitempty" jsonschema:"Force destruction"`
}

// registerCephTools registers the PVE-managed Ceph tools. Read-only (query)
// tools are always registered for an enabled PVE/Ceph backend. Mutation tools
// are registered only when enableMutations is true (SPEC.md §2.5 / §3.4). It
// returns the ToolSurfaces of the registered tools (SPEC L10/L11); when s is
// nil it only computes them (the surface-only pass used by
// RegisteredToolSurface).
func registerCephTools(s *mcpSDK.Server, app *application.App, log toolLogger, enableMutations bool) []ToolSurface {
	var surface []ToolSurface
	surface = append(surface, registerCephQueryTools0(s, app, log)...)
	surface = append(surface, registerCephQueryTools1(s, app, log)...)
	surface = append(surface, registerCephQueryTools2(s, app, log)...)
	if enableMutations {
		surface = append(surface, registerCephMutationTools0(s, app, log)...)
		surface = append(surface, registerCephMutationTools1(s, app, log)...)
		surface = append(surface, registerCephMutationTools2(s, app, log)...)
		surface = append(surface, registerCephMutationTools3(s, app, log)...)
		surface = append(surface, registerCephMutationTools4(s, app, log)...)
	}

	return surface
}

func registerCephQueryTools0(s *mcpSDK.Server, app *application.App, log toolLogger) []ToolSurface {
	return []ToolSurface{
		roTool(s, "ceph_cluster_status", "Get Ceph cluster status",
			"Get the PVE-managed Ceph cluster status (quorum/health).",
			"No arguments. Returns the Ceph cluster status, quorum and health. Read-only and idempotent.",
			log, func(ctx context.Context, _ emptyIn) (any, error) {
				return app.Ceph.GetClusterStatus(ctx)
			}),
		roTool(s, "ceph_cluster_config", "Get Ceph cluster config",
			"Get the Ceph cluster configuration.",
			"No arguments. Returns the Ceph cluster config options. Read-only and idempotent.",
			log, func(ctx context.Context, _ emptyIn) (any, error) {
				return app.Ceph.GetClusterConfig(ctx)
			}),
		roTool(s, "ceph_cluster_log", "Get Ceph cluster log",
			"Get the Ceph cluster log.",
			"Optionally set a limit on the number of returned lines. Read-only and idempotent.",
			log, func(ctx context.Context, in cephClusterLogIn) (any, error) {
				return app.Ceph.GetClusterLog(ctx, in.Limit)
			}),
		roTool(s, "ceph_cluster_metadata", "Get Ceph cluster metadata",
			"Get the Ceph cluster metadata.",
			"No arguments. Returns the Ceph cluster metadata (fsid, version, extras). Read-only and idempotent.",
			log, func(ctx context.Context, _ emptyIn) (any, error) {
				return app.Ceph.GetClusterMetadata(ctx)
			}),
	}
}

func registerCephQueryTools1(s *mcpSDK.Server, app *application.App, log toolLogger) []ToolSurface {
	return []ToolSurface{
		roTool(s, "ceph_node_status", "Get Ceph node status",
			"Get the Ceph status of a single PVE node.",
			"Provide the node name. Returns the node's Ceph status. Read-only.",
			log, func(ctx context.Context, in cephNodeIn) (any, error) {
				return app.Ceph.GetNodeStatus(ctx, in.Node)
			}),
		roTool(s, "ceph_node_config", "Get Ceph node config",
			"Get the Ceph config of a single PVE node.",
			"Provide the node name. Returns the node's Ceph config options. Read-only.",
			log, func(ctx context.Context, in cephNodeIn) (any, error) {
				return app.Ceph.GetNodeConfig(ctx, in.Node)
			}),
		roTool(s, "ceph_node_configdb", "Get Ceph node config DB",
			"Get the Ceph config-db entries of a single PVE node.",
			"Provide the node name. Returns the node's Ceph config-db entries. Read-only.",
			log, func(ctx context.Context, in cephNodeIn) (any, error) {
				return app.Ceph.GetNodeConfigDB(ctx, in.Node)
			}),
		roTool(s, "ceph_node_flags", "Get Ceph node flags",
			"Get the Ceph flags of a single PVE node.",
			"Provide the node name. Returns the node's Ceph flags. Read-only.",
			log, func(ctx context.Context, in cephNodeIn) (any, error) {
				return app.Ceph.GetNodeFlags(ctx, in.Node)
			}),
		roTool(s, "ceph_node_metadata", "Get Ceph node metadata",
			"Get the Ceph metadata of a single PVE node.",
			"Provide the node name. Returns the node's Ceph metadata. Read-only.",
			log, func(ctx context.Context, in cephNodeIn) (any, error) {
				return app.Ceph.GetNodeMetadata(ctx, in.Node)
			}),
	}
}

func registerCephQueryTools2(s *mcpSDK.Server, app *application.App, log toolLogger) []ToolSurface {
	return []ToolSurface{
		roTool(s, "ceph_node_crush", "Get Ceph CRUSH map",
			"Get the Ceph CRUSH map of a single PVE node.",
			"Provide the node name. Returns the node's Ceph CRUSH map (rules, tunables, types, buckets). Read-only.",
			log, func(ctx context.Context, in cephNodeIn) (any, error) {
				return app.Ceph.GetNodeCrush(ctx, in.Node)
			}),
		roTool(s, "ceph_node_rules", "List Ceph CRUSH rules",
			"List the Ceph CRUSH rules of a single PVE node.",
			"Provide the node name. Returns the node's Ceph CRUSH rules. Read-only and idempotent.",
			log, func(ctx context.Context, in cephNodeIn) (any, error) {
				return app.Ceph.GetNodeRules(ctx, in.Node)
			}),
		roTool(s, "ceph_pool_list", "List Ceph pools",
			"List the Ceph pools on a PVE node.",
			"Provide the node name. Returns the node's Ceph pools. Read-only and idempotent.",
			log, func(ctx context.Context, in cephNodeIn) (any, error) {
				return app.Ceph.ListPools(ctx, in.Node)
			}),
		roTool(s, "ceph_osd_list", "List Ceph OSDs",
			"List the Ceph OSDs on a PVE node.",
			"Provide the node name. Returns the node's Ceph OSDs. Read-only and idempotent.",
			log, func(ctx context.Context, in cephNodeIn) (any, error) {
				return app.Ceph.ListOSDs(ctx, in.Node)
			}),
		roTool(s, "ceph_mon_list", "List Ceph monitors",
			"List the Ceph monitors on a PVE node.",
			"Provide the node name. Returns the node's Ceph monitors. Read-only and idempotent.",
			log, func(ctx context.Context, in cephNodeIn) (any, error) {
				return app.Ceph.ListMons(ctx, in.Node)
			}),
		roTool(s, "ceph_mgr_list", "List Ceph managers",
			"List the Ceph managers on a PVE node.",
			"Provide the node name. Returns the node's Ceph managers. Read-only and idempotent.",
			log, func(ctx context.Context, in cephNodeIn) (any, error) {
				return app.Ceph.ListMgrs(ctx, in.Node)
			}),
	}
}

func registerCephMutationTools0(s *mcpSDK.Server, app *application.App, log toolLogger) []ToolSurface {
	return []ToolSurface{
		mutTool(s, "ceph_rule_create", "Create Ceph CRUSH rule",
			"Create a Ceph CRUSH rule on a PVE node.",
			"Provide the node, rule name, type (replicated|erasure), optionally min_size, max_size,"+
				"root and osd. Returns the task UPID to poll with pve_task_status. Not idempotent: each"+
				"call creates a new rule.",
			log, false, func(ctx context.Context, in cephRuleCreateIn) (any, error) {
				upid, err := app.Ceph.CreateCrushRule(ctx, in.Node, model.CephCrushRuleCreateRequest{
					Name:    in.Name,
					Type:    in.Type,
					MinSize: in.MinSize,
					MaxSize: in.MaxSize,
					Root:    in.Root,
					OSD:     in.OSD,
				})
				if err != nil {
					return nil, err
				}

				return map[string]any{"upid": upid}, nil
			}),

		mutTool(s, "ceph_rule_delete", "Delete Ceph CRUSH rule",
			"Delete a Ceph CRUSH rule by name.",
			"Provide the node and rule name. Destructive: removes the CRUSH rule. Not idempotent.",
			log, false, func(ctx context.Context, in cephRuleDeleteIn) (any, error) {
				return map[string]any{"deleted": true}, app.Ceph.DeleteCrushRule(ctx, in.Node, in.Name)
			}, true),

		mutTool(s, "ceph_pool_create", "Create Ceph pool",
			"Create a Ceph pool on a PVE node.",
			"Provide the node and pool name, optionally size, min_size, pg_num and pg_autoscale_mode."+
				"Returns the task UPID to poll with pve_task_status. Not idempotent: each call creates a"+
				"new pool.",
			log, false, func(ctx context.Context, in cephPoolCreateIn) (any, error) {
				upid, err := app.Ceph.CreatePool(ctx, in.Node, model.CephPoolCreateRequest{
					Name:            in.Name,
					Size:            in.Size,
					MinSize:         in.MinSize,
					PGNum:           in.PGNum,
					PGAutoscaleMode: in.PGAutoscaleMode,
				})
				if err != nil {
					return nil, err
				}

				return map[string]any{"upid": upid}, nil
			}),
	}
}

func registerCephMutationTools1(s *mcpSDK.Server, app *application.App, log toolLogger) []ToolSurface {
	return []ToolSurface{
		mutTool(s, "ceph_pool_update", "Update Ceph pool",
			"Update a Ceph pool by name.",
			"Provide the node, pool name and the fields to change (size, min_size, pg_num,"+
				"pg_autoscale_mode). Idempotent: applying the same update twice converges to the same state.",
			log, true, func(ctx context.Context, in cephPoolUpdateIn) (any, error) {
				return nil, app.Ceph.UpdatePool(ctx, in.Node, in.Name, model.CephPoolUpdateRequest{
					Size:            in.Size,
					MinSize:         in.MinSize,
					PGNum:           in.PGNum,
					PGAutoscaleMode: in.PGAutoscaleMode,
				})
			}),

		mutTool(s, "ceph_pool_delete", "Delete Ceph pool",
			"Delete a Ceph pool by name.",
			"Provide the node and pool name. Destructive: removes the pool and its data. Not idempotent.",
			log, false, func(ctx context.Context, in cephPoolDeleteIn) (any, error) {
				return map[string]any{"deleted": true}, app.Ceph.DeletePool(ctx, in.Node, in.Name)
			}, true),

		mutTool(s, "ceph_osd_create", "Create Ceph OSD",
			"Create a Ceph OSD on a PVE node.",
			"Provide the node and an OSD specification (e.g. /dev/sdb). Returns the task UPID to poll"+
				"with pve_task_status. Not idempotent: each call creates a new OSD.",
			log, false, func(ctx context.Context, in cephOSDCreateIn) (any, error) {
				upid, err := app.Ceph.CreateOSD(ctx, in.Node, model.CephOSDCreateRequest{OSD: in.OSD})
				if err != nil {
					return nil, err
				}

				return map[string]any{"upid": upid}, nil
			}),
	}
}

func registerCephMutationTools2(s *mcpSDK.Server, app *application.App, log toolLogger) []ToolSurface {
	return []ToolSurface{
		mutTool(s, "ceph_osd_delete", "Delete Ceph OSD",
			"Delete a Ceph OSD by numeric ID.",
			"Provide the node and the OSD numeric ID. Destructive: removes the OSD from the cluster."+
				"Not idempotent.",
			log, false, func(ctx context.Context, in cephOSDDeleteIn) (any, error) {
				return map[string]any{"deleted": true}, app.Ceph.DeleteOSD(ctx, in.Node, in.OSDID)
			}, true),

		mutTool(s, "ceph_mon_create", "Create Ceph monitor",
			"Create a Ceph monitor on a PVE node.",
			"Provide the node and the monitor address (host:port). Returns the task UPID to poll with"+
				"pve_task_status. Not idempotent: each call creates a new monitor.",
			log, false, func(ctx context.Context, in cephMonCreateIn) (any, error) {
				upid, err := app.Ceph.CreateMon(ctx, in.Node, model.CephMonCreateRequest{MonAddr: in.MonAddr})
				if err != nil {
					return nil, err
				}

				return map[string]any{"upid": upid}, nil
			}),

		mutTool(s, "ceph_mon_delete", "Delete Ceph monitor",
			"Delete a Ceph monitor by ID.",
			"Provide the node and monitor ID. Destructive: removes the monitor from the cluster. Not"+
				"idempotent.",
			log, false, func(ctx context.Context, in cephMonDeleteIn) (any, error) {
				return map[string]any{"deleted": true}, app.Ceph.DeleteMon(ctx, in.Node, in.MonID)
			}, true),
	}
}

func registerCephMutationTools3(s *mcpSDK.Server, app *application.App, log toolLogger) []ToolSurface {
	return []ToolSurface{
		mutTool(s, "ceph_mgr_create", "Create Ceph manager",
			"Create a Ceph manager on a PVE node.",
			"Provide the node and a manager ID. Returns the task UPID to poll with pve_task_status."+
				"Not idempotent: each call creates a new manager.",
			log, false, func(ctx context.Context, in cephMgrCreateIn) (any, error) {
				upid, err := app.Ceph.CreateMgr(ctx, in.Node, model.CephMgrCreateRequest{ID: in.ID})
				if err != nil {
					return nil, err
				}

				return map[string]any{"upid": upid}, nil
			}),

		mutTool(s, "ceph_flags_set", "Set Ceph flags",
			"Set Ceph flags on a PVE node.",
			"Provide the node and a flags map (e.g. {\"noout\":\"true\"}). Idempotent: setting the same"+
				"flags twice converges to the same state.",
			log, true, func(ctx context.Context, in cephFlagsSetIn) (any, error) {
				return nil, app.Ceph.SetFlags(ctx, in.Node, model.CephFlagsSetRequest{Values: in.Values})
			}),

		mutTool(s, "ceph_config_set", "Set Ceph config option",
			"Set a Ceph config option on a PVE node.",
			"Provide the node, option name and value. Idempotent: setting the same option twice"+
				"converges to the same state.",
			log, true, func(ctx context.Context, in cephConfigSetIn) (any, error) {
				return nil, app.Ceph.SetConfig(ctx, in.Node, model.CephConfigSetRequest{Name: in.Name, Value: in.Value})
			}),
	}
}

func registerCephMutationTools4(s *mcpSDK.Server, app *application.App, log toolLogger) []ToolSurface {
	return []ToolSurface{
		mutTool(s, "ceph_init", "Initialize Ceph",
			"Initialize Ceph on a PVE node.",
			"Provide the node, optionally the Ceph public network (network) and cluster network"+
				"(cluster_network) CIDRs. Returns the task UPID to poll with pve_task_status. Not"+
				"idempotent: each call starts a new initialization.",
			log, false, func(ctx context.Context, in cephInitIn) (any, error) {
				upid, err := app.Ceph.InitCeph(ctx, in.Node, model.CephInitRequest{
					Network:        in.Network,
					ClusterNetwork: in.ClusterNetwork,
				})
				if err != nil {
					return nil, err
				}

				return map[string]any{"upid": upid}, nil
			}),

		mutTool(s, "ceph_destroy", "Destroy Ceph",
			"Destroy Ceph on a PVE node.",
			"Provide the node, optionally force. Destructive: tears down Ceph on the node. Not"+
				"idempotent.",
			log, false, func(ctx context.Context, in cephDestroyIn) (any, error) {
				return map[string]any{"deleted": true}, app.Ceph.DestroyCeph(ctx, in.Node, model.CephDestroyRequest{Force: in.Force})
			}, true),
	}
}
