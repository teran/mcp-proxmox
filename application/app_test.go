package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/model"
)

// nopAppLogger is a no-op AppLogger for the App facade tests.
type nopAppLogger struct{}

func (nopAppLogger) Debugf(string, ...any) {}
func (nopAppLogger) Infof(string, ...any)  {}
func (nopAppLogger) Warnf(string, ...any)  {}
func (nopAppLogger) Errorf(string, ...any) {}

// TestNewOptionalInjection verifies the App facade's optional-injection
// behavior: PVE/PBS services are nil unless their With* option is applied.
func TestNewOptionalInjection(t *testing.T) {
	t.Run("no backends", func(t *testing.T) {
		app := New(nopAppLogger{}, "v1.0.0")
		assert.Nil(t, app.PVE)
		assert.Nil(t, app.PBS)
		assert.NotNil(t, app.System)
	})

	t.Run("only PVE", func(t *testing.T) {
		app := New(nopAppLogger{}, "v1.0.0", WithPVE(&mockPVEGateway{}))
		assert.NotNil(t, app.PVE)
		assert.Nil(t, app.PBS)
	})

	t.Run("only PBS", func(t *testing.T) {
		app := New(nopAppLogger{}, "v1.0.0", WithPBS(&mockPBSGateway{}))
		assert.Nil(t, app.PVE)
		assert.NotNil(t, app.PBS)
	})

	t.Run("both", func(t *testing.T) {
		app := New(nopAppLogger{}, "v1.0.0", WithPVE(&mockPVEGateway{}), WithPBS(&mockPBSGateway{}))
		assert.NotNil(t, app.PVE)
		assert.NotNil(t, app.PBS)
	})
}

// TestAppServicesWired verifies the services wired via options actually
// delegate to the injected gateways.
func TestAppServicesWired(t *testing.T) {
	app := New(nopAppLogger{}, "v1.0.0",
		WithPVE(&mockPVEGateway{getNextID: func(ctx context.Context) (string, error) { return "42", nil }}),
		WithPBS(&mockPBSGateway{listDatastores: func(ctx context.Context) ([]model.Datastore, error) {
			return []model.Datastore{{Store: "backup"}}, nil
		}}),
	)

	id, err := app.PVE.GetNextID(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "42", id)

	ds, err := app.PBS.ListDatastores(context.Background())
	require.NoError(t, err)
	assert.Len(t, ds, 1)
}

func TestSystemService_EnabledBackends(t *testing.T) {
	t.Run("none", func(t *testing.T) {
		app := New(nopAppLogger{}, "v1.0.0")
		assert.Equal(t, map[string]bool{"pve": false, "pbs": false}, app.System.EnabledBackends())
		assert.Equal(t, map[string]bool{"pve": false, "pbs": false}, app.System.Ping(context.Background()))
	})

	t.Run("pve only", func(t *testing.T) {
		app := New(nopAppLogger{}, "v1.0.0", WithPVE(&mockPVEGateway{}))
		assert.Equal(t, map[string]bool{"pve": true, "pbs": false}, app.System.EnabledBackends())
	})

	t.Run("pbs only", func(t *testing.T) {
		app := New(nopAppLogger{}, "v1.0.0", WithPBS(&mockPBSGateway{}))
		assert.Equal(t, map[string]bool{"pve": false, "pbs": true}, app.System.EnabledBackends())
	})

	t.Run("both", func(t *testing.T) {
		app := New(nopAppLogger{}, "v1.0.0", WithPVE(&mockPVEGateway{}), WithPBS(&mockPBSGateway{}))
		assert.Equal(t, map[string]bool{"pve": true, "pbs": true}, app.System.EnabledBackends())
	})
}

func TestSystemService_Status(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		app := New(nopAppLogger{}, "v9.9.9")
		st := app.System.Status(context.Background())
		assert.Equal(t, model.ServerStatus{Version: "v9.9.9", Transport: "stdio", PVE: false, PBS: false}, st)
	})

	t.Run("both enabled", func(t *testing.T) {
		app := New(nopAppLogger{}, "v1.0.0", WithPVE(&mockPVEGateway{}), WithPBS(&mockPBSGateway{}))
		st := app.System.Status(context.Background())
		assert.True(t, st.PVE)
		assert.True(t, st.PBS)
		assert.Equal(t, "stdio", st.Transport)
	})
}
