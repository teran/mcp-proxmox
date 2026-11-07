package application

import (
	"context"

	"github.com/teran/mcp-proxmox/domain/model"
	"github.com/teran/mcp-proxmox/domain/port"
)

// CephService is the PVE-managed Ceph use cases. It is a thin orchestration
// layer over port.CephGateway; it exists so adapter/mcp talks to the
// application layer rather than the adapter directly, and so QA can test it
// against a CephGateway mock. See SPEC.md §3.4 / §6.
type CephService struct {
	gw  port.CephGateway
	log port.CtxLogger
}

// GetClusterStatus returns the Ceph cluster status.
func (s *CephService) GetClusterStatus(ctx context.Context) (*model.CephClusterStatus, error) {
	return s.gw.GetClusterStatus(ctx)
}

// GetClusterConfig returns the Ceph cluster config.
func (s *CephService) GetClusterConfig(ctx context.Context) ([]model.CephConfigEntry, error) {
	return s.gw.GetClusterConfig(ctx)
}

// GetClusterLog returns the Ceph cluster log, capped at limit lines.
func (s *CephService) GetClusterLog(ctx context.Context, limit int) ([]model.CephLogEntry, error) {
	return s.gw.GetClusterLog(ctx, limit)
}

// GetClusterMetadata returns the Ceph cluster metadata.
func (s *CephService) GetClusterMetadata(ctx context.Context) (*model.CephClusterMetadata, error) {
	return s.gw.GetClusterMetadata(ctx)
}

// GetNodeStatus returns the Ceph status of a node.
func (s *CephService) GetNodeStatus(ctx context.Context, node string) (*model.CephNodeStatus, error) {
	return s.gw.GetNodeStatus(ctx, node)
}

// GetNodeConfig returns the Ceph config of a node.
func (s *CephService) GetNodeConfig(ctx context.Context, node string) ([]model.CephConfigEntry, error) {
	return s.gw.GetNodeConfig(ctx, node)
}

// GetNodeConfigDB returns the Ceph config-db entries of a node.
func (s *CephService) GetNodeConfigDB(ctx context.Context, node string) ([]model.CephConfigEntry, error) {
	return s.gw.GetNodeConfigDB(ctx, node)
}

// GetNodeFlags returns the Ceph flags of a node.
func (s *CephService) GetNodeFlags(ctx context.Context, node string) ([]model.CephFlag, error) {
	return s.gw.GetNodeFlags(ctx, node)
}

// GetNodeMetadata returns the Ceph metadata of a node.
func (s *CephService) GetNodeMetadata(ctx context.Context, node string) (*model.CephNodeMetadata, error) {
	return s.gw.GetNodeMetadata(ctx, node)
}

// GetNodeCrush returns the Ceph CRUSH map of a node.
func (s *CephService) GetNodeCrush(ctx context.Context, node string) (*model.CephCrushMap, error) {
	return s.gw.GetNodeCrush(ctx, node)
}

// GetNodeRules returns the Ceph CRUSH rules of a node.
func (s *CephService) GetNodeRules(ctx context.Context, node string) ([]model.CephCrushRule, error) {
	return s.gw.GetNodeRules(ctx, node)
}

// ListPools returns the Ceph pools on a node.
func (s *CephService) ListPools(ctx context.Context, node string) ([]model.CephPool, error) {
	return s.gw.ListPools(ctx, node)
}

// ListOSDs returns the Ceph OSDs on a node.
func (s *CephService) ListOSDs(ctx context.Context, node string) ([]model.CephOSD, error) {
	return s.gw.ListOSDs(ctx, node)
}

// ListMons returns the Ceph monitors on a node.
func (s *CephService) ListMons(ctx context.Context, node string) ([]model.CephMon, error) {
	return s.gw.ListMons(ctx, node)
}

// ListMgrs returns the Ceph managers on a node.
func (s *CephService) ListMgrs(ctx context.Context, node string) ([]model.CephMgr, error) {
	return s.gw.ListMgrs(ctx, node)
}

// CreateCrushRule creates a Ceph CRUSH rule and returns the task UPID.
func (s *CephService) CreateCrushRule(ctx context.Context, node string, req model.CephCrushRuleCreateRequest) (string, error) {
	return s.gw.CreateCrushRule(ctx, node, req)
}

// DeleteCrushRule deletes a Ceph CRUSH rule by name.
func (s *CephService) DeleteCrushRule(ctx context.Context, node, name string) error {
	return s.gw.DeleteCrushRule(ctx, node, name)
}

// CreatePool creates a Ceph pool and returns the task UPID.
func (s *CephService) CreatePool(ctx context.Context, node string, req model.CephPoolCreateRequest) (string, error) {
	return s.gw.CreatePool(ctx, node, req)
}

// UpdatePool updates a Ceph pool by name.
func (s *CephService) UpdatePool(ctx context.Context, node, name string, req model.CephPoolUpdateRequest) error {
	return s.gw.UpdatePool(ctx, node, name, req)
}

// DeletePool deletes a Ceph pool by name.
func (s *CephService) DeletePool(ctx context.Context, node, name string) error {
	return s.gw.DeletePool(ctx, node, name)
}

// CreateOSD creates a Ceph OSD and returns the task UPID.
func (s *CephService) CreateOSD(ctx context.Context, node string, req model.CephOSDCreateRequest) (string, error) {
	return s.gw.CreateOSD(ctx, node, req)
}

// DeleteOSD deletes a Ceph OSD by numeric ID.
func (s *CephService) DeleteOSD(ctx context.Context, node string, osdid int) error {
	return s.gw.DeleteOSD(ctx, node, osdid)
}

// CreateMon creates a Ceph monitor and returns the task UPID.
func (s *CephService) CreateMon(ctx context.Context, node string, req model.CephMonCreateRequest) (string, error) {
	return s.gw.CreateMon(ctx, node, req)
}

// DeleteMon deletes a Ceph monitor by ID.
func (s *CephService) DeleteMon(ctx context.Context, node, monid string) error {
	return s.gw.DeleteMon(ctx, node, monid)
}

// CreateMgr creates a Ceph manager and returns the task UPID.
func (s *CephService) CreateMgr(ctx context.Context, node string, req model.CephMgrCreateRequest) (string, error) {
	return s.gw.CreateMgr(ctx, node, req)
}

// SetFlags sets Ceph flags on a node.
func (s *CephService) SetFlags(ctx context.Context, node string, req model.CephFlagsSetRequest) error {
	return s.gw.SetFlags(ctx, node, req)
}

// SetConfig sets a Ceph config option on a node.
func (s *CephService) SetConfig(ctx context.Context, node string, req model.CephConfigSetRequest) error {
	return s.gw.SetConfig(ctx, node, req)
}

// InitCeph initializes Ceph on a node and returns the task UPID.
func (s *CephService) InitCeph(ctx context.Context, node string, req model.CephInitRequest) (string, error) {
	return s.gw.InitCeph(ctx, node, req)
}

// DestroyCeph destroys Ceph on a node.
func (s *CephService) DestroyCeph(ctx context.Context, node string, req model.CephDestroyRequest) error {
	return s.gw.DestroyCeph(ctx, node, req)
}
