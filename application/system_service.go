package application

import (
	"context"

	"github.com/teran/mcp-proxmox/domain/model"
	"github.com/teran/mcp-proxmox/domain/port"
)

// SystemService implements the system use cases: ping and status. It always
// exists; the PVE/PBS services may be nil when the corresponding backend is
// disabled, and the system tools report which backends are active.
type SystemService struct {
	// pve/pbs are set by application.New after options are applied. They may be
	// nil when a backend is disabled.
	pve     *PVEService
	pbs     *PBSService
	log     port.CtxLogger
	Version string
}

// EnabledBackends returns a map describing which backends are enabled
// ("pve" / "pbs"). It backs both the ping and status tools.
func (s *SystemService) EnabledBackends() map[string]bool {
	return map[string]bool{"pve": s.pve != nil, "pbs": s.pbs != nil}
}

// Ping is the liveness check. It is a pure liveness probe: it returns the
// enabled backends without touching the network (the actual API reachability is
// exercised by the backend tools themselves).
func (s *SystemService) Ping(ctx context.Context) map[string]bool {
	return s.EnabledBackends()
}

// Status returns the server status: version, transport, and enabled backends.
func (s *SystemService) Status(ctx context.Context) model.ServerStatus {
	return model.ServerStatus{
		Version:   s.Version,
		Transport: "stdio",
		PVE:       s.pve != nil,
		PBS:       s.pbs != nil,
	}
}
