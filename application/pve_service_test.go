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

func TestPVEService_ReadNodes(t *testing.T) {
	t.Run("ListNodes", func(t *testing.T) {
		gateway := &mockPVEGateway{listNodes: func(_ context.Context) ([]model.Node, error) {
			return []model.Node{{Node: "pve1", Status: "online"}}, nil
		}}
		got, err := (&PVEService{gw: gateway}).ListNodes(context.Background())
		require.NoError(t, err)
		assert.Len(t, got, 1)
		assert.Equal(t, "pve1", got[0].Node)
	})

	t.Run("GetNodeStatus", func(t *testing.T) {
		gateway := &mockPVEGateway{getNodeStatus: func(_ context.Context, node string) (*model.NodeStatus, error) {
			return &model.NodeStatus{Node: node, Status: "online"}, nil
		}}
		got, err := (&PVEService{gw: gateway}).GetNodeStatus(context.Background(), "pve1")
		require.NoError(t, err)
		assert.Equal(t, "online", got.Status)
	})

	t.Run("GetClusterStatus", func(t *testing.T) {
		gateway := &mockPVEGateway{getClusterStatus: func(_ context.Context) ([]model.ClusterStatus, error) {
			return []model.ClusterStatus{{Name: "cluster", Quorum: 1}}, nil
		}}
		got, err := (&PVEService{gw: gateway}).GetClusterStatus(context.Background())
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("GetClusterResources", func(t *testing.T) {
		gateway := &mockPVEGateway{getClusterRes: func(_ context.Context) ([]model.ClusterResource, error) {
			return []model.ClusterResource{{ID: "qemu/100", Type: "qemu"}}, nil
		}}
		got, err := (&PVEService{gw: gateway}).GetClusterResources(context.Background())
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("ListBackupJobs", func(t *testing.T) {
		gateway := &mockPVEGateway{listBackupJobs: func(_ context.Context) ([]model.BackupJob, error) {
			return []model.BackupJob{{ID: "backup-100", Enabled: true, Storage: "local"}}, nil
		}}
		got, err := (&PVEService{gw: gateway}).ListBackupJobs(context.Background())
		require.NoError(t, err)
		assert.Len(t, got, 1)
		assert.Equal(t, "backup-100", got[0].ID)
	})

	t.Run("GetNextID", func(t *testing.T) {
		gateway := &mockPVEGateway{getNextID: func(_ context.Context) (string, error) { return "201", nil }}
		got, err := (&PVEService{gw: gateway}).GetNextID(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "201", got)
	})
}

// TestPVEService_ReadCompute exercises the VM/LXC read use cases.
func TestPVEService_ReadCompute(t *testing.T) {
	t.Run("ListVMs", func(t *testing.T) {
		gateway := &mockPVEGateway{listVMs: func(_ context.Context, _ string) ([]model.VM, error) {
			return []model.VM{{VMID: 100}}, nil
		}}
		got, err := (&PVEService{gw: gateway}).ListVMs(context.Background(), "pve1")
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("GetVMConfig", func(t *testing.T) {
		gateway := &mockPVEGateway{getVMConfig: func(_ context.Context, _ string, vmid int) (*model.VMConfig, error) {
			return &model.VMConfig{VMID: vmid, Name: "web"}, nil
		}}
		got, err := (&PVEService{gw: gateway}).GetVMConfig(context.Background(), "pve1", 100)
		require.NoError(t, err)
		assert.Equal(t, "web", got.Name)
	})

	t.Run("GetVMStatus", func(t *testing.T) {
		gateway := &mockPVEGateway{getVMStatus: func(_ context.Context, _ string, vmid int) (*model.VMStatus, error) {
			return &model.VMStatus{VMID: vmid, Status: "running"}, nil
		}}
		got, err := (&PVEService{gw: gateway}).GetVMStatus(context.Background(), "pve1", 100)
		require.NoError(t, err)
		assert.Equal(t, "running", got.Status)
	})

	t.Run("ListLXCs", func(t *testing.T) {
		gateway := &mockPVEGateway{listLXCs: func(_ context.Context, _ string) ([]model.LXC, error) {
			return []model.LXC{{VMID: 200}}, nil
		}}
		got, err := (&PVEService{gw: gateway}).ListLXCs(context.Background(), "pve1")
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("GetLXCConfig", func(t *testing.T) {
		gateway := &mockPVEGateway{getLXCConfig: func(_ context.Context, _ string, vmid int) (*model.LXCConfig, error) {
			return &model.LXCConfig{VMID: vmid, Hostname: "ct"}, nil
		}}
		got, err := (&PVEService{gw: gateway}).GetLXCConfig(context.Background(), "pve1", 200)
		require.NoError(t, err)
		assert.Equal(t, "ct", got.Hostname)
	})

	t.Run("GetLXCStatus", func(t *testing.T) {
		gateway := &mockPVEGateway{getLXCStatus: func(_ context.Context, _ string, vmid int) (*model.LXCStatus, error) {
			return &model.LXCStatus{VMID: vmid, Status: "running"}, nil
		}}
		got, err := (&PVEService{gw: gateway}).GetLXCStatus(context.Background(), "pve1", 200)
		require.NoError(t, err)
		assert.Equal(t, "running", got.Status)
	})
}

// TestPVEService_ReadStorageAndNetwork exercises the storage/network read use
// cases.
func TestPVEService_ReadStorageAndNetwork(t *testing.T) {
	t.Run("ListStorage", func(t *testing.T) {
		gateway := &mockPVEGateway{listStorage: func(_ context.Context) ([]model.Storage, error) {
			return []model.Storage{{Storage: "local", Type: "dir"}}, nil
		}}
		got, err := (&PVEService{gw: gateway}).ListStorage(context.Background())
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("GetStorageStatus", func(t *testing.T) {
		gateway := &mockPVEGateway{getStorageStatus: func(_ context.Context, _, storage string) (*model.StorageStatus, error) {
			return &model.StorageStatus{Storage: storage, Type: "dir"}, nil
		}}
		got, err := (&PVEService{gw: gateway}).GetStorageStatus(context.Background(), "pve1", "local")
		require.NoError(t, err)
		assert.Equal(t, "local", got.Storage)
	})

	t.Run("ListNetwork", func(t *testing.T) {
		gateway := &mockPVEGateway{listNetwork: func(_ context.Context, _ string) ([]model.NetworkInterface, error) {
			return []model.NetworkInterface{{Iface: "eth0"}}, nil
		}}
		got, err := (&PVEService{gw: gateway}).ListNetwork(context.Background(), "pve1")
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})
}

// TestPVEService_ReadTasksAndVersion exercises the task-log and version read
// use cases.
func TestPVEService_ReadTasksAndVersion(t *testing.T) {
	t.Run("ListTasks", func(t *testing.T) {
		gotOpts := port.TaskListOptions{}
		gateway := &mockPVEGateway{listTasks: func(_ context.Context, _ string, opts port.TaskListOptions) ([]model.Task, error) {
			gotOpts = opts

			return []model.Task{{UPID: "u"}}, nil
		}}
		opts := port.TaskListOptions{VMD: 100, TypeFilter: "qmstart", Limit: 5}
		got, err := (&PVEService{gw: gateway}).ListTasks(context.Background(), "pve1", opts)
		require.NoError(t, err)
		assert.Len(t, got, 1)
		assert.Equal(t, opts, gotOpts)
	})

	t.Run("GetTaskStatus", func(t *testing.T) {
		gateway := &mockPVEGateway{getTaskStatus: func(_ context.Context, _, upid string) (*model.TaskStatus, error) {
			return &model.TaskStatus{UPID: upid, Status: "stopped"}, nil
		}}
		got, err := (&PVEService{gw: gateway}).GetTaskStatus(context.Background(), "pve1", "UPID:...")
		require.NoError(t, err)
		assert.Equal(t, "stopped", got.Status)
	})

	t.Run("GetTaskLog", func(t *testing.T) {
		gotLimit := 0
		gateway := &mockPVEGateway{getTaskLog: func(_ context.Context, _, _ string, limit int) ([]model.TaskLogEntry, error) {
			gotLimit = limit

			return []model.TaskLogEntry{{LineNumber: 1, Text: "OK"}}, nil
		}}
		got, err := (&PVEService{gw: gateway}).GetTaskLog(context.Background(), "pve1", "UPID:...", 50)
		require.NoError(t, err)
		assert.Len(t, got, 1)
		assert.Equal(t, 50, gotLimit)
	})

	t.Run("GetPVEVersion", func(t *testing.T) {
		gateway := &mockPVEGateway{getPVEVersion: func(_ context.Context) (*model.PVEVersion, error) {
			return &model.PVEVersion{Version: "8.2.2"}, nil
		}}
		got, err := (&PVEService{gw: gateway}).GetPVEVersion(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "8.2.2", got.Version)
	})
}

// TestPVEService_MutationVM exercises the VM mutation use cases through the
// PVEGateway mock: CreateVM/ResizeVM/MigrateVM.
func TestPVEService_MutationVM(t *testing.T) {
	t.Run("CreateVM", func(t *testing.T) {
		gotReq := model.CreateVMRequest{}
		gateway := &mockPVEGateway{createVM: func(_ context.Context, _ string, req model.CreateVMRequest) (int, error) {
			gotReq = req

			return 100, nil
		}}
		req := model.CreateVMRequest{Name: "web", Cores: 2, Memory: 1024}
		vmid, err := (&PVEService{gw: gateway}).CreateVM(context.Background(), "pve1", req)
		require.NoError(t, err)
		assert.Equal(t, 100, vmid)
		assert.Equal(t, req, gotReq)
	})

	t.Run("ResizeVM", func(t *testing.T) {
		gotReq := model.ResizeVMRequest{}
		gateway := &mockPVEGateway{resizeVM: func(_ context.Context, _ string, _ int, req model.ResizeVMRequest) error {
			gotReq = req

			return nil
		}}
		req := model.ResizeVMRequest{Disk: "scsi0", SizeGB: 20}
		err := (&PVEService{gw: gateway}).ResizeVM(context.Background(), "pve1", 100, req)
		require.NoError(t, err)
		assert.Equal(t, req, gotReq)
	})

	t.Run("MigrateVM", func(t *testing.T) {
		gotReq := model.MigrateVMRequest{}
		gateway := &mockPVEGateway{migrateVM: func(_ context.Context, _ string, _ int, req model.MigrateVMRequest) error {
			gotReq = req

			return nil
		}}
		req := model.MigrateVMRequest{Target: "pve2", Online: true, WithLocalDisks: true}
		err := (&PVEService{gw: gateway}).MigrateVM(context.Background(), "pve1", 100, req)
		require.NoError(t, err)
		assert.Equal(t, req, gotReq)
	})
}

// TestPVEService_MutationResource exercises the HA-resource and VM-backup
// mutation use cases.
func TestPVEService_MutationResource(t *testing.T) {
	t.Run("AddHAResource", func(t *testing.T) {
		gotReq := model.HAResourceRequest{}
		gateway := &mockPVEGateway{addHAResource: func(_ context.Context, req model.HAResourceRequest) error {
			gotReq = req

			return nil
		}}
		req := model.HAResourceRequest{SID: "vm:100", Type: "vm", Nodes: []string{"pve1"}}
		err := (&PVEService{gw: gateway}).AddHAResource(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, req, gotReq)
	})

	t.Run("StartVMBackup", func(t *testing.T) {
		gotReq := model.VMBackupRequest{}
		gateway := &mockPVEGateway{startVMBackup: func(_ context.Context, _ string, req model.VMBackupRequest) (*model.Task, error) {
			gotReq = req

			return &model.Task{UPID: "UPID:..."}, nil
		}}
		req := model.VMBackupRequest{VMID: 100, Storage: "local", Mode: "snapshot"}
		task, err := (&PVEService{gw: gateway}).StartVMBackup(context.Background(), "pve1", req)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.Equal(t, "UPID:...", task.UPID)
		assert.Equal(t, req, gotReq)
	})
}

// TestPVEService_MutationConfig exercises the VM/LXC config mutation use cases.
func TestPVEService_MutationConfig(t *testing.T) {
	t.Run("SetVMConfig", func(t *testing.T) {
		gotNode := ""
		gotVMID := 0
		gotReq := model.SetVMConfigRequest{}
		gateway := &mockPVEGateway{setVMConfig: func(_ context.Context, node string, vmid int, req model.SetVMConfigRequest) error {
			gotNode = node
			gotVMID = vmid
			gotReq = req

			return nil
		}}
		req := model.SetVMConfigRequest{Config: map[string]string{"onboot": "0"}}
		err := (&PVEService{gw: gateway}).SetVMConfig(context.Background(), "pve1", 100, req)
		require.NoError(t, err)
		assert.Equal(t, "pve1", gotNode)
		assert.Equal(t, 100, gotVMID)
		assert.Equal(t, req, gotReq)
	})

	t.Run("SetLXCConfig", func(t *testing.T) {
		gotNode := ""
		gotVMID := 0
		gotReq := model.SetLXCConfigRequest{}
		gateway := &mockPVEGateway{setLXCConfig: func(_ context.Context, node string, vmid int, req model.SetLXCConfigRequest) error {
			gotNode = node
			gotVMID = vmid
			gotReq = req

			return nil
		}}
		req := model.SetLXCConfigRequest{Config: map[string]string{"onboot": "0"}}
		err := (&PVEService{gw: gateway}).SetLXCConfig(context.Background(), "pve1", 200, req)
		require.NoError(t, err)
		assert.Equal(t, "pve1", gotNode)
		assert.Equal(t, 200, gotVMID)
		assert.Equal(t, req, gotReq)
	})
}

// TestPVEService_MutationBackupJob exercises the backup-job mutation use cases.
func TestPVEService_MutationBackupJob(t *testing.T) {
	t.Run("CreateBackupJob", func(t *testing.T) {
		gotReq := model.CreateBackupJobRequest{}
		gateway := &mockPVEGateway{createBackupJob: func(_ context.Context, req model.CreateBackupJobRequest) (string, error) {
			gotReq = req

			return "backup-100", nil
		}}
		req := model.CreateBackupJobRequest{ID: "backup-100", Storage: "local", Schedule: "00 2 * * 1", Mode: "snapshot"}
		id, err := (&PVEService{gw: gateway}).CreateBackupJob(context.Background(), req)
		require.NoError(t, err)
		assert.Equal(t, "backup-100", id)
		assert.Equal(t, req, gotReq)
	})

	t.Run("UpdateBackupJob", func(t *testing.T) {
		gotID := ""
		gotReq := model.CreateBackupJobRequest{}
		gateway := &mockPVEGateway{updateBackupJob: func(
			_ context.Context, id string, req model.CreateBackupJobRequest,
		) (string, error) {
			gotID = id
			gotReq = req

			return "backup-100", nil
		}}
		req := model.CreateBackupJobRequest{Storage: "local", Schedule: "00 3 * * 1"}
		id, err := (&PVEService{gw: gateway}).UpdateBackupJob(context.Background(), "backup-100", req)
		require.NoError(t, err)
		assert.Equal(t, "backup-100", id)
		assert.Equal(t, "backup-100", gotID)
		assert.Equal(t, req, gotReq)
	})

	t.Run("DeleteBackupJob", func(t *testing.T) {
		gotID := ""
		gateway := &mockPVEGateway{deleteBackupJob: func(_ context.Context, id string) error {
			gotID = id

			return nil
		}}
		err := (&PVEService{gw: gateway}).DeleteBackupJob(context.Background(), "backup-100")
		require.NoError(t, err)
		assert.Equal(t, "backup-100", gotID)
	})
}

// pveErrorPropagationCases pairs each PVE service method with a call that
// invokes it through the thin service layer.
var pveErrorPropagationCases = []struct {
	name string
	call func(*PVEService) error
}{
	{"ListNodes", func(s *PVEService) error {
		_, e := s.ListNodes(context.Background())

		return e
	}},
	{"GetNodeStatus", func(s *PVEService) error {
		_, e := s.GetNodeStatus(context.Background(), "n")

		return e
	}},
	{"GetClusterStatus", func(s *PVEService) error {
		_, e := s.GetClusterStatus(context.Background())

		return e
	}},
	{"GetClusterResources", func(s *PVEService) error {
		_, e := s.GetClusterResources(context.Background())

		return e
	}},
	{"ListBackupJobs", func(s *PVEService) error {
		_, e := s.ListBackupJobs(context.Background())

		return e
	}},
	{"GetNextID", func(s *PVEService) error {
		_, e := s.GetNextID(context.Background())

		return e
	}},
	{"ListVMs", func(s *PVEService) error {
		_, e := s.ListVMs(context.Background(), "n")

		return e
	}},
	{"GetVMConfig", func(s *PVEService) error {
		_, e := s.GetVMConfig(context.Background(), "n", 1)

		return e
	}},
	{"GetVMStatus", func(s *PVEService) error {
		_, e := s.GetVMStatus(context.Background(), "n", 1)

		return e
	}},
	{"ListLXCs", func(s *PVEService) error {
		_, e := s.ListLXCs(context.Background(), "n")

		return e
	}},
	{"GetLXCConfig", func(s *PVEService) error {
		_, e := s.GetLXCConfig(context.Background(), "n", 1)

		return e
	}},
	{"GetLXCStatus", func(s *PVEService) error {
		_, e := s.GetLXCStatus(context.Background(), "n", 1)

		return e
	}},
	{"ListStorage", func(s *PVEService) error {
		_, e := s.ListStorage(context.Background())

		return e
	}},
	{"GetStorageStatus", func(s *PVEService) error {
		_, e := s.GetStorageStatus(context.Background(), "n", "s")

		return e
	}},
	{"ListNetwork", func(s *PVEService) error {
		_, e := s.ListNetwork(context.Background(), "n")

		return e
	}},
	{"ListTasks", func(s *PVEService) error {
		_, e := s.ListTasks(context.Background(), "n", port.TaskListOptions{})

		return e
	}},
	{"GetTaskStatus", func(s *PVEService) error {
		_, e := s.GetTaskStatus(context.Background(), "n", "u")

		return e
	}},
	{"GetTaskLog", func(s *PVEService) error {
		_, e := s.GetTaskLog(context.Background(), "n", "u", 0)

		return e
	}},
	{"GetPVEVersion", func(s *PVEService) error {
		_, e := s.GetPVEVersion(context.Background())

		return e
	}},
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
	{"SetVMConfig", func(s *PVEService) error {
		return s.SetVMConfig(context.Background(), "n", 1, model.SetVMConfigRequest{})
	}},
	{"SetLXCConfig", func(s *PVEService) error {
		return s.SetLXCConfig(context.Background(), "n", 1, model.SetLXCConfigRequest{})
	}},
	{"CreateBackupJob", func(s *PVEService) error {
		_, e := s.CreateBackupJob(context.Background(), model.CreateBackupJobRequest{})

		return e
	}},
	{"UpdateBackupJob", func(s *PVEService) error {
		_, e := s.UpdateBackupJob(context.Background(), "id", model.CreateBackupJobRequest{})

		return e
	}},
	{"DeleteBackupJob", func(s *PVEService) error {
		return s.DeleteBackupJob(context.Background(), "id")
	}},
}

// newFailingPVEGateway returns a gateway whose every method returns sentinel.
func newFailingPVEGateway(sentinel error) *mockPVEGateway {
	gateway := &mockPVEGateway{}
	setReadFailures(gateway, sentinel)
	setMutationFailures(gateway, sentinel)

	return gateway
}

func setReadFailures(gateway *mockPVEGateway, sentinel error) {
	gateway.listNodes = func(_ context.Context) ([]model.Node, error) { return nil, sentinel }
	gateway.getNodeStatus = func(_ context.Context, _ string) (*model.NodeStatus, error) { return nil, sentinel }
	gateway.getClusterStatus = func(_ context.Context) ([]model.ClusterStatus, error) { return nil, sentinel }
	gateway.getClusterRes = func(_ context.Context) ([]model.ClusterResource, error) { return nil, sentinel }
	gateway.getNextID = func(_ context.Context) (string, error) { return "", sentinel }
	gateway.listBackupJobs = func(_ context.Context) ([]model.BackupJob, error) { return nil, sentinel }
	gateway.listVMs = func(_ context.Context, _ string) ([]model.VM, error) { return nil, sentinel }
	gateway.getVMConfig = func(_ context.Context, _ string, _ int) (*model.VMConfig, error) { return nil, sentinel }
	gateway.getVMStatus = func(_ context.Context, _ string, _ int) (*model.VMStatus, error) { return nil, sentinel }
	gateway.listLXCs = func(_ context.Context, _ string) ([]model.LXC, error) { return nil, sentinel }
	gateway.getLXCConfig = func(_ context.Context, _ string, _ int) (*model.LXCConfig, error) { return nil, sentinel }
	gateway.getLXCStatus = func(_ context.Context, _ string, _ int) (*model.LXCStatus, error) { return nil, sentinel }
	gateway.listStorage = func(_ context.Context) ([]model.Storage, error) { return nil, sentinel }
	gateway.getStorageStatus = func(_ context.Context, _, _ string) (*model.StorageStatus, error) { return nil, sentinel }
	gateway.listNetwork = func(_ context.Context, _ string) ([]model.NetworkInterface, error) { return nil, sentinel }
	gateway.listTasks = func(_ context.Context, _ string, _ port.TaskListOptions) ([]model.Task, error) {
		return nil, sentinel
	}
	gateway.getTaskStatus = func(_ context.Context, _, _ string) (*model.TaskStatus, error) { return nil, sentinel }
	gateway.getTaskLog = func(_ context.Context, _, _ string, _ int) ([]model.TaskLogEntry, error) {
		return nil, sentinel
	}
	gateway.getPVEVersion = func(_ context.Context) (*model.PVEVersion, error) { return nil, sentinel }
}

func setMutationFailures(gateway *mockPVEGateway, sentinel error) {
	gateway.createVM = func(_ context.Context, _ string, _ model.CreateVMRequest) (int, error) { return 0, sentinel }
	gateway.resizeVM = func(_ context.Context, _ string, _ int, _ model.ResizeVMRequest) error { return sentinel }
	gateway.migrateVM = func(_ context.Context, _ string, _ int, _ model.MigrateVMRequest) error { return sentinel }
	gateway.addHAResource = func(_ context.Context, _ model.HAResourceRequest) error { return sentinel }
	gateway.startVMBackup = func(_ context.Context, _ string, _ model.VMBackupRequest) (*model.Task, error) {
		return nil, sentinel
	}
	gateway.setVMConfig = func(_ context.Context, _ string, _ int, _ model.SetVMConfigRequest) error {
		return sentinel
	}
	gateway.setLXCConfig = func(_ context.Context, _ string, _ int, _ model.SetLXCConfigRequest) error {
		return sentinel
	}
	gateway.createBackupJob = func(_ context.Context, _ model.CreateBackupJobRequest) (string, error) { return "", sentinel }
	gateway.updateBackupJob = func(_ context.Context, _ string, _ model.CreateBackupJobRequest) (string, error) {
		return "", sentinel
	}
	gateway.deleteBackupJob = func(_ context.Context, _ string) error { return sentinel }
}

// TestPVEService_ErrorPropagation verifies that gateway errors propagate
// unchanged through the thin service layer.
func TestPVEService_ErrorPropagation(t *testing.T) {
	sentinel := errors.New("boom")

	gateway := newFailingPVEGateway(sentinel)
	svc := &PVEService{gw: gateway}
	for _, testCase := range pveErrorPropagationCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.call(svc)
			assert.ErrorIs(t, err, sentinel)
		})
	}
}
