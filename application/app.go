// Package application implements the use cases (thin services) of mcp-proxmox
// over the domain ports, plus the App facade that aggregates them. It depends
// only on domain/* — never on the MCP SDK, net/http, logrus, envconfig, or any
// Proxmox client (enforced by go-arch-lint and the depguard rule).
package application

import (
	"github.com/teran/mcp-proxmox/domain/port"
)

// App is the facade of the application layer: it aggregates all services (use
// cases). The PVE and PBS services are optional — they are wired via the
// WithPVE / WithPBS options only when the corresponding backend is configured;
// otherwise they stay nil and adapter/mcp registers no tools for them.
type App struct {
	// PVE is non-nil only when the PVE backend is enabled (WithPVE applied).
	PVE *PVEService
	// PBS is non-nil only when the PBS backend is enabled (WithPBS applied).
	PBS *PBSService
	// System is always present.
	System *SystemService

	log port.AppLogger
}

// Option configures the App after construction.
type Option func(*App)

// New builds an App. The system service is always created; PVE/PBS are wired
// via the WithPVE / WithPBS options (applied before the returned App is used).
// version is the injected build version, reported by the system `status` tool.
func New(log port.AppLogger, version string, opts ...Option) *App {
	a := &App{
		System: &SystemService{log: port.NewCtxLogger(log), Version: version},
		log:    log,
	}
	for _, o := range opts {
		o(a)
	}
	// The system service reports which backends are enabled after the options
	// have been applied.
	a.System.pve = a.PVE
	a.System.pbs = a.PBS
	return a
}

// WithPVE wires a PVEGateway (adapter/pve) into the App. It must be called at
// startup only when the PVE backend is enabled, so the PVE tools are registered.
func WithPVE(gw port.PVEGateway) Option {
	return func(a *App) {
		a.PVE = &PVEService{gw: gw, log: port.NewCtxLogger(a.log)}
	}
}

// WithPBS wires a PBSGateway (adapter/pbs) into the App. It must be called at
// startup only when the PBS backend is enabled, so the PBS tools are registered.
func WithPBS(gw port.PBSGateway) Option {
	return func(a *App) {
		a.PBS = &PBSService{gw: gw, log: port.NewCtxLogger(a.log)}
	}
}
