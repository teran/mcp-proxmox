package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/adapter/config"
	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/port"
)

func TestVersionString(t *testing.T) {
	// versionString uses the package-level `version` var (default v0.0.0-dev,
	// overridable via -ldflags -X main.version=...).
	assert.Equal(t, "mcp-proxmox "+version, versionString())
}

// noopLogger is a port.AppLogger no-op test double. buildApp must never touch
// the network and only uses the logger for lifecycle lines, so a silent stub
// keeps the test hermetic.
type noopLogger struct{}

func (noopLogger) Tracef(string, ...any) {}
func (noopLogger) Debugf(string, ...any) {}
func (noopLogger) Infof(string, ...any)  {}
func (noopLogger) Warnf(string, ...any)  {}
func (noopLogger) Errorf(string, ...any) {}

// TestBuildAppWiresBothBackends verifies that buildApp collects the options for
// every enabled backend into a single App, so that when BOTH PVE and PBS are
// enabled the returned App retains both (it must NOT drop the PVE wiring like
// the current sequential New(...) calls in main do).
func TestBuildAppWiresBackends(t *testing.T) {
	const version = "v0.0.0-test"

	tests := []struct {
		name    string
		cfg     config.Config
		wantPVE bool
		wantPBS bool
	}{
		{
			name:    "both backends enabled",
			cfg:     config.Config{PVEEndpoint: "https://pve:8006", PVEToken: "u@realm!id=secret", PBSEndpoint: "https://pbs:8007", PBSToken: "u@pbs!id=secret"},
			wantPVE: true,
			wantPBS: true,
		},
		{
			name:    "only PVE enabled",
			cfg:     config.Config{PVEEndpoint: "https://pve:8006", PVEToken: "u@realm!id=secret"},
			wantPVE: true,
			wantPBS: false,
		},
		{
			name:    "only PBS enabled",
			cfg:     config.Config{PBSEndpoint: "https://pbs:8007", PBSToken: "u@pbs!id=secret"},
			wantPVE: false,
			wantPBS: true,
		},
		{
			name:    "no backend enabled",
			cfg:     config.Config{},
			wantPVE: false,
			wantPBS: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var app *application.App
			app = buildApp(tt.cfg, noopLogger{}, version)
			require.NotNil(t, app, "buildApp must always return a non-nil App")
			assert.Equal(t, tt.wantPVE, app.PVE != nil, "PVE presence mismatch")
			assert.Equal(t, tt.wantPBS, app.PBS != nil, "PBS presence mismatch")
		})
	}
}

// TestBuildAppBothBackendsKeepsBoth explicitly guards the composition-root bug:
// building an App with PVE and PBS enabled must wire BOTH into the same App.
func TestBuildAppBothBackendsKeepsBoth(t *testing.T) {
	app := buildApp(config.Config{
		PVEEndpoint: "https://pve:8006",
		PVEToken:    "u@realm!id=secret",
		PBSEndpoint: "https://pbs:8007",
		PBSToken:    "u@pbs!id=secret",
	}, noopLogger{}, "v0.0.0-test")

	require.NotNil(t, app, "buildApp must return a non-nil App")
	require.NotNil(t, app.PVE, "PVE backend must be wired when PVE is enabled")
	require.NotNil(t, app.PBS, "PBS backend must be wired when PBS is enabled")
}
