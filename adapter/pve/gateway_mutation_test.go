package pve

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/model"
	"github.com/teran/mcp-proxmox/domain/port"
)

// TestGateway_CreateVM_Success verifies POST /nodes/{node}/qemu sends the VM
// config as form parameters and decodes the returned VMID from `data`.
func TestGateway_CreateVM_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":100}`, rec)
	g, _ := newTestGateway(t, srv)

	vmid, err := g.CreateVM(context.Background(), "pve1", model.CreateVMRequest{
		VMID: 100, Name: "web", Cores: 2, Memory: 1024, Sockets: 1, Ostype: "l26",
		Net0: "virtio=AA:BB", Scsi0: "local:32", Ide2: "local:iso.iso",
		Boot: "order=scsi0", Balloon: 512, Description: "prod", Tags: "web",
		Agent: 1, OnBoot: 1, CPU: "host", Machine: "q35", VGA: "std",
	})
	require.NoError(t, err)
	assert.Equal(t, 100, vmid)

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api2/json/nodes/pve1/qemu", rec.path)
	assert.Equal(t, "PVEAPIToken=user@pam!tokenid=secret", rec.auth)
	assert.Equal(t, "100", rec.form.Get("vmid"))
	assert.Equal(t, "web", rec.form.Get("name"))
	assert.Equal(t, "2", rec.form.Get("cores"))
	assert.Equal(t, "1024", rec.form.Get("memory"))
	assert.Equal(t, "1", rec.form.Get("sockets"))
	assert.Equal(t, "l26", rec.form.Get("ostype"))
	assert.Equal(t, "virtio=AA:BB", rec.form.Get("net0"))
	assert.Equal(t, "local:32", rec.form.Get("scsi0"))
	assert.Equal(t, "local:iso.iso", rec.form.Get("ide2"))
	assert.Equal(t, "order=scsi0", rec.form.Get("boot"))
	assert.Equal(t, "512", rec.form.Get("balloon"))
	assert.Equal(t, "prod", rec.form.Get("description"))
	assert.Equal(t, "web", rec.form.Get("tags"))
	assert.Equal(t, "1", rec.form.Get("agent"))
	assert.Equal(t, "1", rec.form.Get("onboot"))
	assert.Equal(t, "host", rec.form.Get("cpu"))
	assert.Equal(t, "q35", rec.form.Get("machine"))
	assert.Equal(t, "std", rec.form.Get("vga"))
}

// TestGateway_CreateVM_ZeroVMID verifies a `{"data":0}` response decodes to a
// zero VMID (success).
func TestGateway_CreateVM_ZeroVMID(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":0}`, rec)
	g, _ := newTestGateway(t, srv)

	vmid, err := g.CreateVM(context.Background(), "pve1", model.CreateVMRequest{Name: "web"})
	require.NoError(t, err)
	assert.Equal(t, 0, vmid)
}

// TestGateway_ResizeVM_Success verifies PUT /nodes/{node}/qemu/{vmid}/resize
// with disk and a signed size ("+20G").
func TestGateway_ResizeVM_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{}`, rec)
	g, _ := newTestGateway(t, srv)

	err := g.ResizeVM(context.Background(), "pve1", 100, model.ResizeVMRequest{Disk: "scsi0", SizeGB: 20})
	require.NoError(t, err)

	assert.Equal(t, http.MethodPut, rec.method)
	assert.Equal(t, "/api2/json/nodes/pve1/qemu/100/resize", rec.path)
	assert.Equal(t, "scsi0", rec.form.Get("disk"))
	assert.Equal(t, "+20G", rec.form.Get("size"))
}

// TestGateway_MigrateVM_Success verifies POST .../migrate with target and the
// boolean flags serialized as 1 when set.
func TestGateway_MigrateVM_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{}`, rec)
	g, _ := newTestGateway(t, srv)

	err := g.MigrateVM(context.Background(), "pve1", 100, model.MigrateVMRequest{
		Target: "pve2", Online: true, WithLocalDisks: true,
	})
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api2/json/nodes/pve1/qemu/100/migrate", rec.path)
	assert.Equal(t, "pve2", rec.form.Get("target"))
	assert.Equal(t, "1", rec.form.Get("online"))
	assert.Equal(t, "1", rec.form.Get("with-local-disks"))
}

// TestGateway_AddHAResource_Success verifies POST /cluster/ha/resources with
// sid/type and the node list joined by commas.
func TestGateway_AddHAResource_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{}`, rec)
	g, _ := newTestGateway(t, srv)

	err := g.AddHAResource(context.Background(), model.HAResourceRequest{
		SID: "vm:100", Type: "vm", Nodes: []string{"pve1", "pve2"}, Comment: "primary",
	})
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api2/json/cluster/ha/resources", rec.path)
	assert.Equal(t, "vm:100", rec.form.Get("sid"))
	assert.Equal(t, "vm", rec.form.Get("type"))
	assert.Equal(t, "pve1,pve2", rec.form.Get("nodes"))
	assert.Equal(t, "primary", rec.form.Get("comment"))
}

// TestGateway_StartVMBackup_Success verifies POST /nodes/{node}/vzdump and that
// the returned Task wraps the UPID decoded from `data`.
func TestGateway_StartVMBackup_Success(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, 200, `{"data":"UPID:pve1:00001000:..."}`, rec)
	g, _ := newTestGateway(t, srv)

	task, err := g.StartVMBackup(context.Background(), "pve1", model.VMBackupRequest{
		VMID: 100, Storage: "backup", Mode: "snapshot",
		NotesTemplate: "{{guestname}}", Compress: "zstd",
	})
	require.NoError(t, err)
	require.NotNil(t, task)
	assert.Equal(t, "UPID:pve1:00001000:...", task.UPID)

	assert.Equal(t, http.MethodPost, rec.method)
	assert.Equal(t, "/api2/json/nodes/pve1/vzdump", rec.path)
	assert.Equal(t, "100", rec.form.Get("vmid"))
	assert.Equal(t, "backup", rec.form.Get("storage"))
	assert.Equal(t, "snapshot", rec.form.Get("mode"))
	assert.Equal(t, "{{guestname}}", rec.form.Get("notes-template"))
	assert.Equal(t, "zstd", rec.form.Get("compress"))
}

// TestGateway_MutationErrorMapping verifies that mutation failures map through
// the same error family as reads: 401 → ErrUnauthorized and a 5xx →
// UpstreamError without any retry (mutations are never replayed).
func TestGateway_MutationErrorMapping(t *testing.T) {
	t.Run("create vm unauthorized", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusUnauthorized, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.CreateVM(context.Background(), "pve1", model.CreateVMRequest{})
		require.Error(t, err)
		assert.ErrorIs(t, err, port.ErrUnauthorized)
		assert.Equal(t, 1, rec.count)
	})

	t.Run("resize upstream error no retry", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		err := g.ResizeVM(context.Background(), "pve1", 100, model.ResizeVMRequest{Disk: "scsi0", SizeGB: 5})
		require.Error(t, err)
		var ue *UpstreamError
		assert.ErrorAs(t, err, &ue)
		assert.Equal(t, 1, rec.count, "mutations must not be retried")
	})

	t.Run("migrate upstream error no retry", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		err := g.MigrateVM(context.Background(), "pve1", 100, model.MigrateVMRequest{Target: "pve2"})
		require.Error(t, err)
		assert.Equal(t, 1, rec.count)
	})

	t.Run("ha add upstream error no retry", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		err := g.AddHAResource(context.Background(), model.HAResourceRequest{SID: "vm:100", Type: "vm"})
		require.Error(t, err)
		assert.Equal(t, 1, rec.count)
	})

	t.Run("vm backup unauthorized", func(t *testing.T) {
		rec := &reqCapture{}
		srv := mockServer(t, http.StatusUnauthorized, `{"data":null}`, rec)
		g, _ := newTestGateway(t, srv)
		_, err := g.StartVMBackup(context.Background(), "pve1", model.VMBackupRequest{VMID: 100, Storage: "s", Mode: "snapshot"})
		require.Error(t, err)
		assert.ErrorIs(t, err, port.ErrUnauthorized)
		assert.Equal(t, 1, rec.count)
	})
}

// TestGateway_MutationErrorBranch covers the remaining mutation error return
// paths so the gateways' error branches are fully exercised.
func TestGateway_MutationErrorBranch(t *testing.T) {
	rec := &reqCapture{}
	srv := mockServer(t, http.StatusInternalServerError, `{"data":null}`, rec)
	g, _ := newTestGateway(t, srv)

	_, err := g.CreateVM(context.Background(), "pve1", model.CreateVMRequest{})
	require.Error(t, err)
	_, err = g.StartVMBackup(context.Background(), "pve1", model.VMBackupRequest{VMID: 1, Storage: "s", Mode: "snapshot"})
	require.Error(t, err)
}
