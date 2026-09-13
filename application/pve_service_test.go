package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/model"
	"github.com/teran/mcp-proxmox/domain/port"
)

func TestPVEService_Success(t *testing.T) {
	t.Run("ListNodes", func(t *testing.T) {
		gw := &mockPVEGateway{listNodes: func(ctx context.Context) ([]model.Node, error) {
			return []model.Node{{Node: "pve1", Status: "online"}}, nil
		}}
		got, err := (&PVEService{gw: gw}).ListNodes(context.Background())
		require.NoError(t, err)
		assert.Len(t, got, 1)
		assert.Equal(t, "pve1", got[0].Node)
	})

	t.Run("GetNodeStatus", func(t *testing.T) {
		gw := &mockPVEGateway{getNodeStatus: func(ctx context.Context, node string) (*model.NodeStatus, error) {
			return &model.NodeStatus{Node: node, Status: "online"}, nil
		}}
		got, err := (&PVEService{gw: gw}).GetNodeStatus(context.Background(), "pve1")
		require.NoError(t, err)
		assert.Equal(t, "online", got.Status)
	})

	t.Run("GetClusterStatus", func(t *testing.T) {
		gw := &mockPVEGateway{getClusterStatus: func(ctx context.Context) ([]model.ClusterStatus, error) {
			return []model.ClusterStatus{{Name: "cluster", Quorum: 1}}, nil
		}}
		got, err := (&PVEService{gw: gw}).GetClusterStatus(context.Background())
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("GetClusterResources", func(t *testing.T) {
		gw := &mockPVEGateway{getClusterRes: func(ctx context.Context) ([]model.ClusterResource, error) {
			return []model.ClusterResource{{ID: "qemu/100", Type: "qemu"}}, nil
		}}
		got, err := (&PVEService{gw: gw}).GetClusterResources(context.Background())
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("GetNextID", func(t *testing.T) {
		gw := &mockPVEGateway{getNextID: func(ctx context.Context) (string, error) { return "201", nil }}
		got, err := (&PVEService{gw: gw}).GetNextID(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "201", got)
	})

	t.Run("ListVMs", func(t *testing.T) {
		gw := &mockPVEGateway{listVMs: func(ctx context.Context, node string) ([]model.VM, error) {
			return []model.VM{{VMID: 100}}, nil
		}}
		got, err := (&PVEService{gw: gw}).ListVMs(context.Background(), "pve1")
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("GetVMConfig", func(t *testing.T) {
		gw := &mockPVEGateway{getVMConfig: func(ctx context.Context, node string, vmid int) (*model.VMConfig, error) {
			return &model.VMConfig{VMID: vmid, Name: "web"}, nil
		}}
		got, err := (&PVEService{gw: gw}).GetVMConfig(context.Background(), "pve1", 100)
		require.NoError(t, err)
		assert.Equal(t, "web", got.Name)
	})

	t.Run("GetVMStatus", func(t *testing.T) {
		gw := &mockPVEGateway{getVMStatus: func(ctx context.Context, node string, vmid int) (*model.VMStatus, error) {
			return &model.VMStatus{VMID: vmid, Status: "running"}, nil
		}}
		got, err := (&PVEService{gw: gw}).GetVMStatus(context.Background(), "pve1", 100)
		require.NoError(t, err)
		assert.Equal(t, "running", got.Status)
	})

	t.Run("ListLXCs", func(t *testing.T) {
		gw := &mockPVEGateway{listLXCs: func(ctx context.Context, node string) ([]model.LXC, error) {
			return []model.LXC{{VMID: 200}}, nil
		}}
		got, err := (&PVEService{gw: gw}).ListLXCs(context.Background(), "pve1")
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("GetLXCConfig", func(t *testing.T) {
		gw := &mockPVEGateway{getLXCConfig: func(ctx context.Context, node string, vmid int) (*model.LXCConfig, error) {
			return &model.LXCConfig{VMID: vmid, Hostname: "ct"}, nil
		}}
		got, err := (&PVEService{gw: gw}).GetLXCConfig(context.Background(), "pve1", 200)
		require.NoError(t, err)
		assert.Equal(t, "ct", got.Hostname)
	})

	t.Run("GetLXCStatus", func(t *testing.T) {
		gw := &mockPVEGateway{getLXCStatus: func(ctx context.Context, node string, vmid int) (*model.LXCStatus, error) {
			return &model.LXCStatus{VMID: vmid, Status: "running"}, nil
		}}
		got, err := (&PVEService{gw: gw}).GetLXCStatus(context.Background(), "pve1", 200)
		require.NoError(t, err)
		assert.Equal(t, "running", got.Status)
	})

	t.Run("ListStorage", func(t *testing.T) {
		gw := &mockPVEGateway{listStorage: func(ctx context.Context) ([]model.Storage, error) {
			return []model.Storage{{Storage: "local", Type: "dir"}}, nil
		}}
		got, err := (&PVEService{gw: gw}).ListStorage(context.Background())
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("GetStorageStatus", func(t *testing.T) {
		gw := &mockPVEGateway{getStorageStatus: func(ctx context.Context, node, storage string) (*model.StorageStatus, error) {
			return &model.StorageStatus{Storage: storage, Type: "dir"}, nil
		}}
		got, err := (&PVEService{gw: gw}).GetStorageStatus(context.Background(), "pve1", "local")
		require.NoError(t, err)
		assert.Equal(t, "local", got.Storage)
	})

	t.Run("ListNetwork", func(t *testing.T) {
		gw := &mockPVEGateway{listNetwork: func(ctx context.Context, node string) ([]model.NetworkInterface, error) {
			return []model.NetworkInterface{{Iface: "eth0"}}, nil
		}}
		got, err := (&PVEService{gw: gw}).ListNetwork(context.Background(), "pve1")
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("ListTasks", func(t *testing.T) {
		gotOpts := port.TaskListOptions{}
		gw := &mockPVEGateway{listTasks: func(ctx context.Context, node string, opts port.TaskListOptions) ([]model.Task, error) {
			gotOpts = opts
			return []model.Task{{UPID: "u"}}, nil
		}}
		opts := port.TaskListOptions{VMD: 100, TypeFilter: "qmstart", Limit: 5}
		got, err := (&PVEService{gw: gw}).ListTasks(context.Background(), "pve1", opts)
		require.NoError(t, err)
		assert.Len(t, got, 1)
		assert.Equal(t, opts, gotOpts)
	})

	t.Run("GetTaskStatus", func(t *testing.T) {
		gw := &mockPVEGateway{getTaskStatus: func(ctx context.Context, node, upid string) (*model.TaskStatus, error) {
			return &model.TaskStatus{UPID: upid, Status: "stopped"}, nil
		}}
		got, err := (&PVEService{gw: gw}).GetTaskStatus(context.Background(), "pve1", "UPID:...")
		require.NoError(t, err)
		assert.Equal(t, "stopped", got.Status)
	})

	t.Run("GetTaskLog", func(t *testing.T) {
		gotLimit := 0
		gw := &mockPVEGateway{getTaskLog: func(ctx context.Context, node, upid string, limit int) ([]model.TaskLogEntry, error) {
			gotLimit = limit
			return []model.TaskLogEntry{{LineNumber: 1, Text: "OK"}}, nil
		}}
		got, err := (&PVEService{gw: gw}).GetTaskLog(context.Background(), "pve1", "UPID:...", 50)
		require.NoError(t, err)
		assert.Len(t, got, 1)
		assert.Equal(t, 50, gotLimit)
	})

	t.Run("GetPVEVersion", func(t *testing.T) {
		gw := &mockPVEGateway{getPVEVersion: func(ctx context.Context) (*model.PVEVersion, error) {
			return &model.PVEVersion{Version: "8.2.2"}, nil
		}}
		got, err := (&PVEService{gw: gw}).GetPVEVersion(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "8.2.2", got.Version)
	})
}

// TestPVEService_MutationSuccess exercises the mutation use cases through the
// PVEGateway mock: CreateVM/ResizeVM/MigrateVM/AddHAResource/StartVMBackup.
func TestPVEService_MutationSuccess(t *testing.T) {
	t.Run("CreateVM", func(t *testing.T) {
		gotReq := model.CreateVMRequest{}
		gw := &mockPVEGateway{createVM: func(ctx context.Context, node string, req model.CreateVMRequest) (int, error) {
			gotReq = req
			return 100, nil
		}}
		req := model.CreateVMRequest{Name: "web", Cores: 2, Memory: 1024}
		vmid, err := (&PVEService{gw: gw}).CreateVM(context.Background(), "pve1", req)
		require.NoError(t, err)
		assert.Equal(t, 100, vmid)
		assert.Equal(t, req, gotReq)
	})

	t.Run("ResizeVM", func(t *testing.T) {
		gotReq := model.ResizeVMRequest{}
		gw := &mockPVEGateway{resizeVM: func(ctx context.Context, node string, vmid int, req model.ResizeVMRequest) error {
			gotReq = req
			return nil
		}}
		req := model.ResizeVMRequest{Disk: "scsi0", SizeGB: 20}
		err := (&PVEService{gw: gw}).ResizeVM(context.Background(), "pve1", 100, req)
		require.NoError(t, err)
		assert.Equal(t, req, gotReq)
	})

	t.Run("MigrateVM", func(t *testing.T) {
		gotReq := model.MigrateVMRequest{}
		gw := &mockPVEGateway{migrateVM: func(ctx context.Context, node string, vmid int, req model.MigrateVMRequest) error {
			gotReq = req
			return nil
		}}
		req := model.MigrateVMRequest{Target: "pve2", Online: true, WithLocalDisks: true}
		err := (&PVEService{gw: gw}).MigrateVM(context.Background(), "pve1", 100, req)
		require.NoError(t, err)
		assert.Equal(t, req, gotReq)
	})

	t.Run("AddHAResource", func(t *testing.T) {
		gotReq := model.HAResourceRequest{}
		gw := &mockPVEGateway{addHAResource: func(ctx context.Context, req model.HAResourceRequest) error {
			gotReq = req
			return nil
		}}
		req := model.HAResourceRequest{SID: "vm:100", Type: "vm", Nodes: []string{"pve1"}}
		err := (&PVEService{gw: gw}).AddHAResource(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, req, gotReq)
	})

	t.Run("StartVMBackup", func(t *testing.T) {
		gotReq := model.VMBackupRequest{}
		gw := &mockPVEGateway{startVMBackup: func(ctx context.Context, node string, req model.VMBackupRequest) (*model.Task, error) {
			gotReq = req
			return &model.Task{UPID: "UPID:..."}, nil
		}}
		req := model.VMBackupRequest{VMID: 100, Storage: "local", Mode: "snapshot"}
		task, err := (&PVEService{gw: gw}).StartVMBackup(context.Background(), "pve1", req)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.Equal(t, "UPID:...", task.UPID)
		assert.Equal(t, req, gotReq)
	})
}

// TestPVEService_ErrorPropagation verifies that gateway errors propagate
// unchanged through the thin service layer.
func TestPVEService_ErrorPropagation(t *testing.T) {
	sentinel := errors.New("boom")

	cases := []struct {
		name string
		call func(*PVEService) error
	}{
		{"ListNodes", func(s *PVEService) error { _, e := s.ListNodes(context.Background()); return e }},
		{"GetNodeStatus", func(s *PVEService) error { _, e := s.GetNodeStatus(context.Background(), "n"); return e }},
		{"GetClusterStatus", func(s *PVEService) error { _, e := s.GetClusterStatus(context.Background()); return e }},
		{"GetClusterResources", func(s *PVEService) error { _, e := s.GetClusterResources(context.Background()); return e }},
		{"GetNextID", func(s *PVEService) error { _, e := s.GetNextID(context.Background()); return e }},
		{"ListVMs", func(s *PVEService) error { _, e := s.ListVMs(context.Background(), "n"); return e }},
		{"GetVMConfig", func(s *PVEService) error { _, e := s.GetVMConfig(context.Background(), "n", 1); return e }},
		{"GetVMStatus", func(s *PVEService) error { _, e := s.GetVMStatus(context.Background(), "n", 1); return e }},
		{"ListLXCs", func(s *PVEService) error { _, e := s.ListLXCs(context.Background(), "n"); return e }},
		{"GetLXCConfig", func(s *PVEService) error { _, e := s.GetLXCConfig(context.Background(), "n", 1); return e }},
		{"GetLXCStatus", func(s *PVEService) error { _, e := s.GetLXCStatus(context.Background(), "n", 1); return e }},
		{"ListStorage", func(s *PVEService) error { _, e := s.ListStorage(context.Background()); return e }},
		{"GetStorageStatus", func(s *PVEService) error { _, e := s.GetStorageStatus(context.Background(), "n", "s"); return e }},
		{"ListNetwork", func(s *PVEService) error { _, e := s.ListNetwork(context.Background(), "n"); return e }},
		{"ListTasks", func(s *PVEService) error {
			_, e := s.ListTasks(context.Background(), "n", port.TaskListOptions{})
			return e
		}},
		{"GetTaskStatus", func(s *PVEService) error { _, e := s.GetTaskStatus(context.Background(), "n", "u"); return e }},
		{"GetTaskLog", func(s *PVEService) error { _, e := s.GetTaskLog(context.Background(), "n", "u", 0); return e }},
		{"GetPVEVersion", func(s *PVEService) error { _, e := s.GetPVEVersion(context.Background()); return e }},
		{"CreateVM", func(s *PVEService) error {
			_, e := s.CreateVM(context.Background(), "n", model.CreateVMRequest{})
			return e
		}},
		{"ResizeVM", func(s *PVEService) error { return s.ResizeVM(context.Background(), "n", 1, model.ResizeVMRequest{}) }},
		{"MigrateVM", func(s *PVEService) error { return s.MigrateVM(context.Background(), "n", 1, model.MigrateVMRequest{}) }},
		{"AddHAResource", func(s *PVEService) error { return s.AddHAResource(context.Background(), model.HAResourceRequest{}) }},
		{"StartVMBackup", func(s *PVEService) error {
			_, e := s.StartVMBackup(context.Background(), "n", model.VMBackupRequest{})
			return e
		}},
	}

	// Build a gateway where every method returns the sentinel error.
	gw := &mockPVEGateway{}
	gw.listNodes = func(ctx context.Context) ([]model.Node, error) { return nil, sentinel }
	gw.getNodeStatus = func(ctx context.Context, node string) (*model.NodeStatus, error) { return nil, sentinel }
	gw.getClusterStatus = func(ctx context.Context) ([]model.ClusterStatus, error) { return nil, sentinel }
	gw.getClusterRes = func(ctx context.Context) ([]model.ClusterResource, error) { return nil, sentinel }
	gw.getNextID = func(ctx context.Context) (string, error) { return "", sentinel }
	gw.listVMs = func(ctx context.Context, node string) ([]model.VM, error) { return nil, sentinel }
	gw.getVMConfig = func(ctx context.Context, node string, vmid int) (*model.VMConfig, error) { return nil, sentinel }
	gw.getVMStatus = func(ctx context.Context, node string, vmid int) (*model.VMStatus, error) { return nil, sentinel }
	gw.listLXCs = func(ctx context.Context, node string) ([]model.LXC, error) { return nil, sentinel }
	gw.getLXCConfig = func(ctx context.Context, node string, vmid int) (*model.LXCConfig, error) { return nil, sentinel }
	gw.getLXCStatus = func(ctx context.Context, node string, vmid int) (*model.LXCStatus, error) { return nil, sentinel }
	gw.listStorage = func(ctx context.Context) ([]model.Storage, error) { return nil, sentinel }
	gw.getStorageStatus = func(ctx context.Context, node, storage string) (*model.StorageStatus, error) { return nil, sentinel }
	gw.listNetwork = func(ctx context.Context, node string) ([]model.NetworkInterface, error) { return nil, sentinel }
	gw.listTasks = func(ctx context.Context, node string, opts port.TaskListOptions) ([]model.Task, error) {
		return nil, sentinel
	}
	gw.getTaskStatus = func(ctx context.Context, node, upid string) (*model.TaskStatus, error) { return nil, sentinel }
	gw.getTaskLog = func(ctx context.Context, node, upid string, limit int) ([]model.TaskLogEntry, error) {
		return nil, sentinel
	}
	gw.getPVEVersion = func(ctx context.Context) (*model.PVEVersion, error) { return nil, sentinel }
	gw.createVM = func(ctx context.Context, node string, req model.CreateVMRequest) (int, error) { return 0, sentinel }
	gw.resizeVM = func(ctx context.Context, node string, vmid int, req model.ResizeVMRequest) error { return sentinel }
	gw.migrateVM = func(ctx context.Context, node string, vmid int, req model.MigrateVMRequest) error { return sentinel }
	gw.addHAResource = func(ctx context.Context, req model.HAResourceRequest) error { return sentinel }
	gw.startVMBackup = func(ctx context.Context, node string, req model.VMBackupRequest) (*model.Task, error) {
		return nil, sentinel
	}

	svc := &PVEService{gw: gw}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call(svc)
			assert.ErrorIs(t, err, sentinel)
		})
	}
}
