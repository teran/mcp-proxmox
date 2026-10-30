package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/model"
)

// requireCloneSnapshotTask asserts a successful VM clone/snapshot mutation
// returned the expected task UPID.
func requireCloneSnapshotTask(t *testing.T, err error, task *model.Task, wantUPID string) {
	t.Helper()
	require.NoError(t, err)
	require.NotNil(t, task)
	assert.Equal(t, wantUPID, task.UPID)
}

// TestPVEService_CloneSnapshotSuccess exercises the QEMU VM clone and snapshot
// use cases (CloneVM/ListVMSnapshots/CreateVMSnapshot/DeleteVMSnapshot/
// RollbackVMSnapshot) through the PVEGateway mock.
func TestPVEService_CloneSnapshotSuccess(t *testing.T) {
	t.Run("CloneVM", func(t *testing.T) {
		gotReq := model.CloneVMRequest{}
		gateway := &mockPVEGateway{cloneVM: func(_ context.Context, _ string, _ int, req model.CloneVMRequest) (*model.Task, error) {
			gotReq = req

			return &model.Task{UPID: "UPID:clone"}, nil
		}}
		req := model.CloneVMRequest{NewID: 101, Name: "web-clone", Full: true}
		task, err := (&PVEService{gw: gateway}).CloneVM(context.Background(), "pve1", 100, req)
		requireCloneSnapshotTask(t, err, task, "UPID:clone")
		assert.Equal(t, req, gotReq)
	})

	t.Run("ListVMSnapshots", func(t *testing.T) {
		gateway := &mockPVEGateway{listVMSnapshots: func(_ context.Context, _ string, _ int) ([]model.Snapshot, error) {
			return []model.Snapshot{{Name: "snap1"}}, nil
		}}
		snaps, err := (&PVEService{gw: gateway}).ListVMSnapshots(context.Background(), "pve1", 100)
		require.NoError(t, err)
		require.Len(t, snaps, 1)
		assert.Equal(t, "snap1", snaps[0].Name)
	})

	t.Run("CreateVMSnapshot", func(t *testing.T) {
		gotReq := model.SnapshotCreateRequest{}
		gateway := &mockPVEGateway{createVMSnapshot: func(
			_ context.Context, _ string, _ int, req model.SnapshotCreateRequest,
		) (*model.Task, error) {
			gotReq = req

			return &model.Task{UPID: "UPID:create"}, nil
		}}
		req := model.SnapshotCreateRequest{Snapname: "snap1", VMState: true}
		task, err := (&PVEService{gw: gateway}).CreateVMSnapshot(context.Background(), "pve1", 100, req)
		requireCloneSnapshotTask(t, err, task, "UPID:create")
		assert.Equal(t, req, gotReq)
	})

	t.Run("DeleteVMSnapshot", func(t *testing.T) {
		gateway := &mockPVEGateway{deleteVMSnapshot: func(_ context.Context, _ string, _ int, _ string) (*model.Task, error) {
			return &model.Task{UPID: "UPID:delete"}, nil
		}}
		task, err := (&PVEService{gw: gateway}).DeleteVMSnapshot(context.Background(), "pve1", 100, "snap1")
		requireCloneSnapshotTask(t, err, task, "UPID:delete")
	})

	t.Run("RollbackVMSnapshot", func(t *testing.T) {
		gateway := &mockPVEGateway{rollbackVMSnapshot: func(_ context.Context, _ string, _ int, _ string) (*model.Task, error) {
			return &model.Task{UPID: "UPID:rollback"}, nil
		}}
		task, err := (&PVEService{gw: gateway}).RollbackVMSnapshot(context.Background(), "pve1", 100, "snap1")
		requireCloneSnapshotTask(t, err, task, "UPID:rollback")
	})
}

// TestPVEService_CloneSnapshotErrorPropagation verifies gateway errors propagate
// unchanged.
func TestPVEService_CloneSnapshotErrorPropagation(t *testing.T) {
	sentinel := errors.New("pve boom")
	gateway := &mockPVEGateway{}
	gateway.cloneVM = func(_ context.Context, _ string, _ int, _ model.CloneVMRequest) (*model.Task, error) {
		return nil, sentinel
	}
	gateway.listVMSnapshots = func(_ context.Context, _ string, _ int) ([]model.Snapshot, error) { return nil, sentinel }
	gateway.createVMSnapshot = func(_ context.Context, _ string, _ int, _ model.SnapshotCreateRequest) (*model.Task, error) {
		return nil, sentinel
	}
	gateway.deleteVMSnapshot = func(_ context.Context, _ string, _ int, _ string) (*model.Task, error) {
		return nil, sentinel
	}
	gateway.rollbackVMSnapshot = func(_ context.Context, _ string, _ int, _ string) (*model.Task, error) {
		return nil, sentinel
	}

	svc := &PVEService{gw: gateway}
	cases := []struct {
		name string
		call func(*PVEService) error
	}{
		{"CloneVM", func(s *PVEService) error {
			_, e := s.CloneVM(context.Background(), "n", 1, model.CloneVMRequest{})

			return e
		}},
		{"ListVMSnapshots", func(s *PVEService) error {
			_, e := s.ListVMSnapshots(context.Background(), "n", 1)

			return e
		}},
		{"CreateVMSnapshot", func(s *PVEService) error {
			_, e := s.CreateVMSnapshot(context.Background(), "n", 1, model.SnapshotCreateRequest{})

			return e
		}},
		{"DeleteVMSnapshot", func(s *PVEService) error {
			_, e := s.DeleteVMSnapshot(context.Background(), "n", 1, "s")

			return e
		}},
		{"RollbackVMSnapshot", func(s *PVEService) error {
			_, e := s.RollbackVMSnapshot(context.Background(), "n", 1, "s")

			return e
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.ErrorIs(t, testCase.call(svc), sentinel)
		})
	}
}
