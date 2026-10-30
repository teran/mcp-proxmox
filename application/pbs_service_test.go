package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/model"
)

func TestPBSService_ReadDatastores(t *testing.T) {
	t.Run("ListDatastores", func(t *testing.T) {
		gateway := &mockPBSGateway{listDatastores: func(_ context.Context) ([]model.Datastore, error) {
			return []model.Datastore{{Store: "backup"}}, nil
		}}
		got, err := (&PBSService{gw: gateway}).ListDatastores(context.Background())
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("GetDatastoreStatus", func(t *testing.T) {
		gateway := &mockPBSGateway{getDatastoreStats: func(_ context.Context, store string) (*model.DatastoreStatus, error) {
			return &model.DatastoreStatus{Store: store, Total: 1000}, nil
		}}
		got, err := (&PBSService{gw: gateway}).GetDatastoreStatus(context.Background(), "backup")
		require.NoError(t, err)
		assert.Equal(t, int64(1000), got.Total)
	})

	t.Run("ListBackups", func(t *testing.T) {
		gateway := &mockPBSGateway{listBackups: func(_ context.Context, _ string) ([]model.Backup, error) {
			return []model.Backup{{BackupID: "vm/100/..."}}, nil
		}}
		got, err := (&PBSService{gw: gateway}).ListBackups(context.Background(), "backup")
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("GetBackup", func(t *testing.T) {
		gateway := &mockPBSGateway{getBackup: func(_ context.Context, _, backupID string) ([]model.Backup, error) {
			return []model.Backup{{BackupID: backupID}}, nil
		}}
		got, err := (&PBSService{gw: gateway}).GetBackup(context.Background(), "backup", "vm/100/...")
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "vm/100/...", got[0].BackupID)
	})
}

// TestPBSService_ReadStatusAndVersion exercises the status/version read use
// cases.
func TestPBSService_ReadStatusAndVersion(t *testing.T) {
	t.Run("GetBackupNotes", func(t *testing.T) {
		gateway := &mockPBSGateway{getBackupNotes: func(_ context.Context, _, _, _ string) (*model.BackupNotes, error) {
			return &model.BackupNotes{Notes: "keep forever"}, nil
		}}
		got, err := (&PBSService{gw: gateway}).GetBackupNotes(context.Background(), "backup", "vm/100/...", "vm")
		require.NoError(t, err)
		assert.Equal(t, "keep forever", got.Notes)
	})

	t.Run("GetVerifyStatus", func(t *testing.T) {
		gateway := &mockPBSGateway{getVerifyStatus: func(_ context.Context, _, upid string) (*model.VerifyStatus, error) {
			return &model.VerifyStatus{UPID: upid, Status: "ok"}, nil
		}}
		got, err := (&PBSService{gw: gateway}).GetVerifyStatus(context.Background(), "backup", "UPID:...")
		require.NoError(t, err)
		assert.Equal(t, "ok", got.Status)
	})

	t.Run("GetPruneStatus", func(t *testing.T) {
		gateway := &mockPBSGateway{getPruneStatus: func(_ context.Context, _, upid string) (*model.PruneStatus, error) {
			return &model.PruneStatus{UPID: upid, Status: "running"}, nil
		}}
		got, err := (&PBSService{gw: gateway}).GetPruneStatus(context.Background(), "backup", "UPID:...")
		require.NoError(t, err)
		assert.Equal(t, "running", got.Status)
	})

	t.Run("GetPBSVersion", func(t *testing.T) {
		gateway := &mockPBSGateway{getPBSVersion: func(_ context.Context) (*model.PBSVersion, error) {
			return &model.PBSVersion{Version: "3.2.3"}, nil
		}}
		got, err := (&PBSService{gw: gateway}).GetPBSVersion(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "3.2.3", got.Version)
	})
}

// TestPBSService_MutationSuccess exercises the PBS mutation use cases
// (StartVerify/StartGC/StartPrune/StartSync) through the PBSGateway mock.
func TestPBSService_MutationSuccess(t *testing.T) {
	t.Run("StartVerify", func(t *testing.T) {
		gateway := &mockPBSGateway{startVerify: func(_ context.Context, _ string) (string, error) {
			return "UPID:verify", nil
		}}
		upid, err := (&PBSService{gw: gateway}).StartVerify(context.Background(), "backup")
		require.NoError(t, err)
		assert.Equal(t, "UPID:verify", upid)
	})

	t.Run("StartGC", func(t *testing.T) {
		gateway := &mockPBSGateway{startGC: func(_ context.Context, _ string) (string, error) {
			return "UPID:gc", nil
		}}
		upid, err := (&PBSService{gw: gateway}).StartGC(context.Background(), "backup")
		require.NoError(t, err)
		assert.Equal(t, "UPID:gc", upid)
	})

	t.Run("StartPrune", func(t *testing.T) {
		gateway := &mockPBSGateway{startPrune: func(_ context.Context, _ string) (string, error) {
			return "UPID:prune", nil
		}}
		upid, err := (&PBSService{gw: gateway}).StartPrune(context.Background(), "backup")
		require.NoError(t, err)
		assert.Equal(t, "UPID:prune", upid)
	})

	t.Run("StartSync", func(t *testing.T) {
		gotReq := model.PBSSyncRequest{}
		gateway := &mockPBSGateway{startSync: func(_ context.Context, _ string, req model.PBSSyncRequest) (string, error) {
			gotReq = req

			return "UPID:sync", nil
		}}
		req := model.PBSSyncRequest{Remote: "remote1", RemoteStore: "backup"}
		upid, err := (&PBSService{gw: gateway}).StartSync(context.Background(), "backup", req)
		require.NoError(t, err)
		assert.Equal(t, "UPID:sync", upid)
		assert.Equal(t, req, gotReq)
	})
}

// pbsErrorPropagationCases pairs each PBS service method with a call that
// invokes it through the thin service layer.
var pbsErrorPropagationCases = []struct {
	name string
	call func(*PBSService) error
}{
	{"ListDatastores", func(s *PBSService) error {
		_, e := s.ListDatastores(context.Background())

		return e
	}},
	{"GetDatastoreStatus", func(s *PBSService) error {
		_, e := s.GetDatastoreStatus(context.Background(), "s")

		return e
	}},
	{"ListBackups", func(s *PBSService) error {
		_, e := s.ListBackups(context.Background(), "s")

		return e
	}},
	{"GetBackup", func(s *PBSService) error {
		_, e := s.GetBackup(context.Background(), "s", "id")

		return e
	}},
	{"GetBackupNotes", func(s *PBSService) error {
		_, e := s.GetBackupNotes(context.Background(), "s", "id", "vm")

		return e
	}},
	{"GetVerifyStatus", func(s *PBSService) error {
		_, e := s.GetVerifyStatus(context.Background(), "s", "u")

		return e
	}},
	{"GetPruneStatus", func(s *PBSService) error {
		_, e := s.GetPruneStatus(context.Background(), "s", "u")

		return e
	}},
	{"GetPBSVersion", func(s *PBSService) error {
		_, e := s.GetPBSVersion(context.Background())

		return e
	}},
	{"StartVerify", func(s *PBSService) error {
		_, e := s.StartVerify(context.Background(), "s")

		return e
	}},
	{"StartGC", func(s *PBSService) error {
		_, e := s.StartGC(context.Background(), "s")

		return e
	}},
	{"StartPrune", func(s *PBSService) error {
		_, e := s.StartPrune(context.Background(), "s")

		return e
	}},
	{"StartSync", func(s *PBSService) error {
		_, e := s.StartSync(context.Background(), "s", model.PBSSyncRequest{})

		return e
	}},
}

// newFailingPBSGateway returns a gateway whose every method returns sentinel.
func newFailingPBSGateway(sentinel error) *mockPBSGateway {
	gateway := &mockPBSGateway{}
	gateway.listDatastores = func(_ context.Context) ([]model.Datastore, error) { return nil, sentinel }
	gateway.getDatastoreStats = func(_ context.Context, _ string) (*model.DatastoreStatus, error) { return nil, sentinel }
	gateway.listBackups = func(_ context.Context, _ string) ([]model.Backup, error) { return nil, sentinel }
	gateway.getBackup = func(_ context.Context, _, _ string) ([]model.Backup, error) { return nil, sentinel }
	gateway.getBackupNotes = func(_ context.Context, _, _, _ string) (*model.BackupNotes, error) {
		return nil, sentinel
	}
	gateway.getVerifyStatus = func(_ context.Context, _, _ string) (*model.VerifyStatus, error) { return nil, sentinel }
	gateway.getPruneStatus = func(_ context.Context, _, _ string) (*model.PruneStatus, error) { return nil, sentinel }
	gateway.getPBSVersion = func(_ context.Context) (*model.PBSVersion, error) { return nil, sentinel }
	gateway.startVerify = func(_ context.Context, _ string) (string, error) { return "", sentinel }
	gateway.startGC = func(_ context.Context, _ string) (string, error) { return "", sentinel }
	gateway.startPrune = func(_ context.Context, _ string) (string, error) { return "", sentinel }
	gateway.startSync = func(_ context.Context, _ string, _ model.PBSSyncRequest) (string, error) {
		return "", sentinel
	}

	return gateway
}

func TestPBSService_ErrorPropagation(t *testing.T) {
	sentinel := errors.New("pbs boom")

	svc := &PBSService{gw: newFailingPBSGateway(sentinel)}
	for _, testCase := range pbsErrorPropagationCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.call(svc)
			assert.ErrorIs(t, err, sentinel)
		})
	}
}
