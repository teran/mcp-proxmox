package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-proxmox/application"
)

// This file tests the RegisteredToolSurface contract (L10/L11 conformance to
// mcp-server-spec):
//
//	type ToolSurface struct {
//	    Name          string
//	    Title         string
//	    AlwaysPresent bool
//	}
//	func RegisteredToolSurface(app *application.App, enableMutations bool) []ToolSurface
//
// The surface must mirror exactly the set (and order) of tools that
// RegisterTools registers on a real server (invariant: surface == tools/list),
// with the AlwaysPresent flag set to true for every tool. This is the single
// source of truth the composition root (cmd/mcp-proxmox) uses to emit the
// startup tool-surface log.

// orderedToolNames returns the registered tool names in the order tools/list
// reports them (registration order), for the invariant check.
func orderedToolNames(t *testing.T, sess *mcpSDK.ClientSession) []string {
	t.Helper()
	res, err := sess.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	names := make([]string, 0, len(res.Tools))
	for _, tl := range res.Tools {
		names = append(names, tl.Name)
	}

	return names
}

// TestRegisteredToolSurface_Composition covers the full matrix of backend
// enablement (PVE/PBS) × mutation gate, asserting:
//   - system tools ping/status are always present;
//   - a backend's read-only tools are present iff that backend is enabled;
//   - a backend's mutation tools are present iff the backend is enabled AND
//     enableMutations is true;
//   - every entry reports AlwaysPresent == true (L11);
//   - the surface contains exactly the expected tools — no extras, no missing.
func TestRegisteredToolSurface_Composition(t *testing.T) {
	tests := []struct {
		name            string
		pve             bool
		ceph            bool
		pbs             bool
		enableMutations bool
	}{
		{"no backends, mutations off", false, false, false, false},
		{"no backends, mutations on", false, false, false, true},
		{"pve only, mutations off", true, true, false, false},
		{"pve only, mutations on", true, true, false, true},
		{"pbs only, mutations off", false, false, true, false},
		{"pbs only, mutations on", false, false, true, true},
		{"both, mutations off", true, true, true, false},
		{"both, mutations on", true, true, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts []application.Option
			if tt.pve {
				opts = append(opts, application.WithPVE(&stubPVEGateway{}))
			}
			if tt.ceph {
				opts = append(opts, application.WithCeph(&stubCephGateway{}))
			}
			if tt.pbs {
				opts = append(opts, application.WithPBS(&stubPBSGateway{}))
			}
			app := application.New(nopLogger{}, "v1.0.0", opts...)

			assertSurfaceComposition(t, RegisteredToolSurface(app, tt.enableMutations),
				tt.pve, tt.ceph, tt.pbs, tt.enableMutations)
		})
	}
}

// assertSurfaceComposition asserts the per-case invariants of the surface
// composition matrix: system tools always present, backend tools iff enabled,
// mutation tools iff enabled AND gated on, every entry AlwaysPresent, and
// exactly the expected number of tools (no extras, no missing).
func assertSurfaceComposition(t *testing.T, surface []ToolSurface, pve, ceph, pbs, enableMutations bool) {
	t.Helper()

	names := make(map[string]bool, len(surface))
	for _, ts := range surface {
		names[ts.Name] = true
		require.NotEmpty(t, ts.Title,
			"L10/L05GO: every surface entry must carry a title; %s had none", ts.Name)
		require.True(t, ts.AlwaysPresent,
			"L11: every surface entry must be AlwaysPresent; %s was not", ts.Name)
	}

	// System tools are always present, regardless of backends/gate.
	assert.True(t, names["ping"], "ping must always be present")
	assert.True(t, names["status"], "status must always be present")

	// Read-only tools appear exactly when their backend is enabled.
	for _, n := range pveROTools {
		assert.Equal(t, pve, names[n], "pve read-only tool %s presence", n)
	}
	for _, n := range cephROTools {
		assert.Equal(t, ceph, names[n], "ceph read-only tool %s presence", n)
	}
	for _, n := range pbsROTools {
		assert.Equal(t, pbs, names[n], "pbs read-only tool %s presence", n)
	}

	// Mutation tools appear only when the backend is enabled AND the gate is on.
	for _, n := range pveMutationTools {
		assert.Equal(t, pve && enableMutations, names[n], "pve mutation tool %s presence", n)
	}
	for _, n := range cephMutationTools {
		assert.Equal(t, ceph && enableMutations, names[n], "ceph mutation tool %s presence", n)
	}
	for _, n := range pbsMutationTools {
		assert.Equal(t, pbs && enableMutations, names[n], "pbs mutation tool %s presence", n)
	}

	// No extras and no missing: the surface size must be exactly the union of
	// system + enabled read-only + gated mutations.
	want := 2 + len(pveROTools)*boolInt(pve) + len(cephROTools)*boolInt(ceph) + len(pbsROTools)*boolInt(pbs)
	if pve && enableMutations {
		want += len(pveMutationTools)
	}
	if ceph && enableMutations {
		want += len(cephMutationTools)
	}
	if pbs && enableMutations {
		want += len(pbsMutationTools)
	}
	assert.Len(t, surface, want, "surface must contain exactly the expected tools")
}

func boolInt(b bool) int {
	if b {
		return 1
	}

	return 0
}

// TestRegisteredToolSurface_MatchesRegisteredTools asserts the core L10/L11
// invariant: the set returned by RegisteredToolSurface for a given
// App/gate must equal, name-for-name and in order, the tools actually
// registered on a real server (surface == tools/list). Both backends with
// mutations enabled exercise the fullest surface.
func TestRegisteredToolSurface_MatchesRegisteredTools(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0",
		application.WithPVE(&stubPVEGateway{}), application.WithCeph(&stubCephGateway{}),
		application.WithPBS(&stubPBSGateway{}))
	_, sess := newTestServer(t, app, true)

	surface := RegisteredToolSurface(app, true)
	surfaceNames := make([]string, len(surface))
	for i, ts := range surface {
		surfaceNames[i] = ts.Name
	}

	actual := orderedToolNames(t, sess)
	assert.Equal(t, surfaceNames, actual,
		"RegisteredToolSurface must mirror the tools actually registered (surface == tools/list)")
}

// TestRegisteredToolSurface_Order pins the stable order of the startup surface
// log: tools are sorted by name, exactly as tools/list returns them (the go-sdk
// featureSet yields features sorted by unique ID), so the startup log is a true
// mirror of what a client sees over the protocol.
func TestRegisteredToolSurface_Order(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0",
		application.WithPVE(&stubPVEGateway{}), application.WithCeph(&stubCephGateway{}),
		application.WithPBS(&stubPBSGateway{}))
	surface := RegisteredToolSurface(app, true)

	require.GreaterOrEqual(t, len(surface), 2)
	prev := surface[0].Name
	for _, ts := range surface[1:] {
		assert.True(t, prev < ts.Name,
			"surface must be sorted by name (tools/list order); %q after %q", ts.Name, prev)
		prev = ts.Name
	}
}

// TestToolSurface_EveryEntryHasTitle is the focused L10/L05GO conformance check
// over the full surface (both backends, mutations on): every entry must carry a
// non-empty Name and human-readable Title, and report AlwaysPresent == true, so
// the startup tool-surface log can emit a meaningful `title` for every tool.
func TestToolSurface_EveryEntryHasTitle(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0",
		application.WithPVE(&stubPVEGateway{}), application.WithCeph(&stubCephGateway{}),
		application.WithPBS(&stubPBSGateway{}))
	surface := RegisteredToolSurface(app, true)

	require.NotEmpty(t, surface, "full surface must not be empty")
	for _, ts := range surface {
		assert.NotEmpty(t, ts.Name, "every surface entry must carry a name")
		assert.NotEmpty(t, ts.Title,
			"L10/L05GO: every surface entry must carry a title; %s had none", ts.Name)
		assert.True(t, ts.AlwaysPresent,
			"L11: every surface entry must be AlwaysPresent; %s was not", ts.Name)
	}
}
