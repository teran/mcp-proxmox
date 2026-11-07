package port

import (
	"context"

	"github.com/teran/mcp-proxmox/domain/model"
)

// CephGateway is the secondary port for PVE-managed Ceph, implemented by
// adapter/ceph. It models the Ceph subset of the Proxmox VE REST API
// (/api2/json): the read-only (query) surface plus the mutation surface
// (create/update/delete/init/destroy), which the MCP adapter registers only
// when the ENABLE_MUTATIONS gate is on (SPEC.md §2.5 / §3.4).
//
// Ceph rides the PVE API token (Authorization: PVEAPIToken=...), so it shares
// the PVE endpoint/token configuration and is enabled together with the PVE
// backend. It keeps its own port + adapter per the two-gateway shape (SPEC.md
// §2.3): Ceph has a distinct object model (pools, OSDs, mons, mgrs, CRUSH) and
// never shares an entity set with PVE or PBS.
type CephGateway interface {
	// Cluster-level reads.
	GetClusterStatus(ctx context.Context) (*model.CephClusterStatus, error)
	GetClusterConfig(ctx context.Context) ([]model.CephConfigEntry, error)
	GetClusterLog(ctx context.Context, limit int) ([]model.CephLogEntry, error)
	GetClusterMetadata(ctx context.Context) (*model.CephClusterMetadata, error)

	// Node-level reads.
	GetNodeStatus(ctx context.Context, node string) (*model.CephNodeStatus, error)
	GetNodeConfig(ctx context.Context, node string) ([]model.CephConfigEntry, error)
	GetNodeConfigDB(ctx context.Context, node string) ([]model.CephConfigEntry, error)
	GetNodeFlags(ctx context.Context, node string) ([]model.CephFlag, error)
	GetNodeMetadata(ctx context.Context, node string) (*model.CephNodeMetadata, error)
	GetNodeCrush(ctx context.Context, node string) (*model.CephCrushMap, error)
	GetNodeRules(ctx context.Context, node string) ([]model.CephCrushRule, error)
	ListPools(ctx context.Context, node string) ([]model.CephPool, error)
	ListOSDs(ctx context.Context, node string) ([]model.CephOSD, error)
	ListMons(ctx context.Context, node string) ([]model.CephMon, error)
	ListMgrs(ctx context.Context, node string) ([]model.CephMgr, error)

	// Mutations (gated by EnableMutations). These are never retried.
	CreateCrushRule(ctx context.Context, node string, req model.CephCrushRuleCreateRequest) (string, error)
	DeleteCrushRule(ctx context.Context, node, name string) error
	CreatePool(ctx context.Context, node string, req model.CephPoolCreateRequest) (string, error)
	UpdatePool(ctx context.Context, node, name string, req model.CephPoolUpdateRequest) error
	DeletePool(ctx context.Context, node, name string) error
	CreateOSD(ctx context.Context, node string, req model.CephOSDCreateRequest) (string, error)
	DeleteOSD(ctx context.Context, node string, osdid int) error
	CreateMon(ctx context.Context, node string, req model.CephMonCreateRequest) (string, error)
	DeleteMon(ctx context.Context, node, monid string) error
	CreateMgr(ctx context.Context, node string, req model.CephMgrCreateRequest) (string, error)
	SetFlags(ctx context.Context, node string, req model.CephFlagsSetRequest) error
	SetConfig(ctx context.Context, node string, req model.CephConfigSetRequest) error
	InitCeph(ctx context.Context, node string, req model.CephInitRequest) (string, error)
	DestroyCeph(ctx context.Context, node string, req model.CephDestroyRequest) error
}
