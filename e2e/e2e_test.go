//go:build e2e

// Package e2e holds the hermetic end-to-end test suite. It is build-tagged
// `e2e` and therefore does NOT run in the default `go test ./...` pass. Run it
// with `make e2e` (requires Docker, see AGENTS.md §5 / SPEC.md §8.6).
//
// The suite:
//  1. builds the stdlib-only Proxmox emulator (e2e/emulator) for linux/amd64,
//  2. runs it in a Docker container via github.com/teran/go-docker-testsuite,
//  3. builds the real cmd/mcp-proxmox server for the host arch,
//  4. launches it as a subprocess over stdio pointed at the emulator, and
//  5. drives the MCP protocol to assert a real tool round-trip end-to-end
//     (including the Authorization header flowing to the emulator).
package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/go-docker-testsuite"
)

const (
	// emulatorListenPort is the in-container port the emulator binds. It is
	// NAT'ed to an ephemeral host port by the harness.
	emulatorListenPort = 8080

	// emulatorImage is the base image used for the emulator container. The
	// emulator is a static Go binary, so any minimal Linux image works; it can
	// be overridden for offline/alternative environments via the
	// MCP_PROXMOX_E2E_IMAGE env var.
	emulatorImage = "alpine:latest"
)

// TestPVEToolRoundTrip drives the real server over stdio against the emulator
// and asserts a real PVE tool returns the emulator's canned data.
func TestPVEToolRoundTrip(t *testing.T) {
	root := repoRoot(t)
	binDir := t.TempDir()

	// Build the emulator for linux/amd64 (the e2e runs in Docker, typically on
	// a linux CI runner) and the real server for the host arch.
	emulatorBytes := buildEmulator(t, root, binDir)
	serverBin := buildServer(t, root, binDir)

	// Start the emulator container. The emulator is the container's MAIN
	// command (not WithStartupCommand): the harness runs startup commands via
	// `docker exec`, which blocks until the command exits, so a long-running
	// server must be the container's entrypoint process instead.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	emulator := "emulator-" + filepath.Base(binDir)
	bindings := docker.NewPortBindings().PortDNAT(docker.ProtoTCP, emulatorListenPort)
	c, err := docker.NewContainerWithLifecycle(
		emulator,
		emulatorImage,
		[]string{"/usr/local/bin/emulator", "-addr", ":8080"},
		docker.NewEnvironment(),
		bindings,
		docker.WithFiles(
			docker.FileFromBytes("/usr/local/bin/emulator", emulatorBytes, 0o755, 0, 0),
		),
		// Cap the emulator so a runaway cannot exhaust the host/CI runner.
		// WithMemoryLimit/WithCPUs/WithPidsLimit are HostConfig (ContainerOption)
		// knobs, so they are grouped under WithHostConfig to be usable alongside
		// the LifecycleOptions above.
		docker.WithHostConfig(
			docker.WithMemoryLimit(128*1024*1024), // 128 MiB
			docker.WithCPUs(0.5),                  // half a vCPU
			docker.WithPidsLimit(256),
		),
	)
	require.NoError(t, err, "create emulator container")

	tc := docker.BindToT(t, c) // ties container lifecycle (teardown) to the test
	require.NoError(t, tc.Run(ctx), "run emulator container")

	// Resolve the host-side endpoint for the NAT'ed emulator port.
	hostPort, err := tc.URL(docker.ProtoTCP, emulatorListenPort)
	require.NoError(t, err, "resolve emulator host port")
	endpoint := "http://" + hostPort.String()

	// Launch the real server as a subprocess over stdio, pointed at the
	// emulator, with the PVE backend enabled. The CommandTransport starts the
	// process and connects to it over stdin/stdout.
	serverCmd := exec.Command(serverBin)
	serverCmd.Env = append(os.Environ(),
		"PVE_ENDPOINT="+endpoint,
		"PVE_TOKEN=test-pve-token",
	)

	client := mcpSDK.NewClient(&mcpSDK.Implementation{Name: "e2e-client", Version: "v0.0.0-test"}, nil)
	sess, err := client.Connect(ctx, &mcpSDK.CommandTransport{Command: serverCmd}, nil)
	require.NoError(t, err, "connect MCP client to server")
	defer sess.Close() // closes stdin and terminates the subprocess

	// Sanity: the read-only PVE tool we exercise is registered.
	tools, err := sess.ListTools(ctx, nil)
	require.NoError(t, err, "tools/list")
	names := make(map[string]bool, len(tools.Tools))
	for _, tl := range tools.Tools {
		names[tl.Name] = true
	}
	assert.True(t, names["pve_node_list"], "pve_node_list must be registered")

	// The real round-trip: call pve_node_list and assert it returns the
	// emulator's canned node.
	res, err := sess.CallTool(ctx, &mcpSDK.CallToolParams{Name: "pve_node_list", Arguments: map[string]any{}})
	require.NoError(t, err, "tools/call pve_node_list")
	require.False(t, res.IsError, "pve_node_list must succeed")

	sc, ok := res.StructuredContent.(map[string]any)
	require.True(t, ok, "structured content must be an object")
	items, ok := sc["items"].([]any)
	require.True(t, ok, "node list must be wrapped under items")
	require.Len(t, items, 1, "emulator returns exactly one node")

	node, ok := items[0].(map[string]any)
	require.True(t, ok, "node entry must be an object")
	assert.Equal(t, "pve", node["node"])
	assert.Equal(t, "online", node["status"])
}

// repoRoot walks up from this file until it finds the go.mod, returning the
// repository root. Build commands run from here so package paths are stable
// regardless of the test's working directory.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "locate e2e_test.go")

	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not locate repository root (go.mod)")
		}
		dir = parent
	}
}

// buildEmulator compiles the stdlib-only emulator for linux/amd64 and returns
// its bytes (ready to be copied into the container via WithFiles).
func buildEmulator(t *testing.T, root, binDir string) []byte {
	t.Helper()
	bin := filepath.Join(binDir, "emulator")
	cmd := exec.Command("go", "build", "-o", bin, "./e2e/emulator")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build emulator: %v\n%s", err, out)
	}
	data, err := os.ReadFile(bin)
	require.NoError(t, err, "read emulator binary")
	return data
}

// buildServer compiles the real cmd/mcp-proxmox server for the host arch and
// returns its path.
func buildServer(t *testing.T, root, binDir string) string {
	t.Helper()
	bin := filepath.Join(binDir, "mcp-proxmox")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/mcp-proxmox")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build server: %v\n%s", err, out)
	}
	return bin
}
