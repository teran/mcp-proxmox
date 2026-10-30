package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/model"
)

// requireLXCCloneSnapshotTask asserts a successful LXC clone/snapshot mutation
// returned the expected task UPID.
func requireLXCCloneSnapshotTask(t *testing.T, err error, task *model.Task, wantUPID string) {
	t.Helper()
	require.NoError(t, err)
	require.NotNil(t, task)
	assert.Equal(t, wantUPID, task.UPID)
}

// TestPVEService_LXCCloneSnapshotSuccess exercises the LXC clone and snapshot
// use cases through the PVEGateway mock.
func TestPVEService_LXCCloneSnapshotSuccess(t *testing.T) {
	t.Run("CloneLXC", func(t *testing.T) {
		gotReq := model.CloneLXCRequest{}
		gateway := &mockPVEGateway{cloneLXC: func(_ context.Context, _ string, _ int, req model.CloneLXCRequest) (*model.Task, error) {
			gotReq = req

			return &model.Task{UPID: "UPID:clone"}, nil
		}}
		req := model.CloneLXCRequest{NewID: 201, Hostname: "ct-clone", Full: true}
		task, err := (&PVEService{gw: gateway}).CloneLXC(context.Background(), "pve1", 200, req)
		requireLXCCloneSnapshotTask(t, err, task, "UPID:clone")
		assert.Equal(t, req, gotReq)
	})

	t.Run("ListLXCSnapshots", func(t *testing.T) {
		gateway := &mockPVEGateway{listLXCSnapshots: func(_ context.Context, _ string, _ int) ([]model.Snapshot, error) {
			return []model.Snapshot{{Name: "snap1"}}, nil
		}}
		snaps, err := (&PVEService{gw: gateway}).ListLXCSnapshots(context.Background(), "pve1", 200)
		require.NoError(t, err)
		require.Len(t, snaps, 1)
		assert.Equal(t, "snap1", snaps[0].Name)
	})

	t.Run("CreateLXCSnapshot", func(t *testing.T) {
		gotReq := model.SnapshotCreateRequest{}
		gateway := &mockPVEGateway{createLXCSnapshot: func(
			_ context.Context, _ string, _ int, req model.SnapshotCreateRequest,
		) (*model.Task, error) {
			gotReq = req

			return &model.Task{UPID: "UPID:create"}, nil
		}}
		req := model.SnapshotCreateRequest{Snapname: "snap1", Description: "d"}
		task, err := (&PVEService{gw: gateway}).CreateLXCSnapshot(context.Background(), "pve1", 200, req)
		requireLXCCloneSnapshotTask(t, err, task, "UPID:create")
		assert.Equal(t, req, gotReq)
	})

	t.Run("DeleteLXCSnapshot", func(t *testing.T) {
		gateway := &mockPVEGateway{deleteLXCSnapshot: func(_ context.Context, _ string, _ int, _ string) (*model.Task, error) {
			return &model.Task{UPID: "UPID:delete"}, nil
		}}
		task, err := (&PVEService{gw: gateway}).DeleteLXCSnapshot(context.Background(), "pve1", 200, "snap1")
		requireLXCCloneSnapshotTask(t, err, task, "UPID:delete")
	})

	t.Run("RollbackLXCSnapshot", func(t *testing.T) {
		gateway := &mockPVEGateway{rollbackLXCSnapshot: func(_ context.Context, _ string, _ int, _ string) (*model.Task, error) {
			return &model.Task{UPID: "UPID:rollback"}, nil
		}}
		task, err := (&PVEService{gw: gateway}).RollbackLXCSnapshot(context.Background(), "pve1", 200, "snap1")
		requireLXCCloneSnapshotTask(t, err, task, "UPID:rollback")
	})
}

// TestPVEService_LXCCloneSnapshotErrorPropagation verifies gateway errors
// propagate unchanged.
func TestPVEService_LXCCloneSnapshotErrorPropagation(t *testing.T) {
	sentinel := errors.New("pve boom")
	gateway := &mockPVEGateway{}
	gateway.cloneLXC = func(_ context.Context, _ string, _ int, _ model.CloneLXCRequest) (*model.Task, error) {
		return nil, sentinel
	}
	gateway.listLXCSnapshots = func(_ context.Context, _ string, _ int) ([]model.Snapshot, error) { return nil, sentinel }
	gateway.createLXCSnapshot = func(_ context.Context, _ string, _ int, _ model.SnapshotCreateRequest) (*model.Task, error) {
		return nil, sentinel
	}
	gateway.deleteLXCSnapshot = func(_ context.Context, _ string, _ int, _ string) (*model.Task, error) {
		return nil, sentinel
	}
	gateway.rollbackLXCSnapshot = func(_ context.Context, _ string, _ int, _ string) (*model.Task, error) {
		return nil, sentinel
	}

	svc := &PVEService{gw: gateway}
	cases := []struct {
		name string
		call func(*PVEService) error
	}{
		{"CloneLXC", func(s *PVEService) error {
			_, e := s.CloneLXC(context.Background(), "n", 1, model.CloneLXCRequest{})

			return e
		}},
		{"ListLXCSnapshots", func(s *PVEService) error {
			_, e := s.ListLXCSnapshots(context.Background(), "n", 1)

			return e
		}},
		{"CreateLXCSnapshot", func(s *PVEService) error {
			_, e := s.CreateLXCSnapshot(context.Background(), "n", 1, model.SnapshotCreateRequest{})

			return e
		}},
		{"DeleteLXCSnapshot", func(s *PVEService) error {
			_, e := s.DeleteLXCSnapshot(context.Background(), "n", 1, "s")

			return e
		}},
		{"RollbackLXCSnapshot", func(s *PVEService) error {
			_, e := s.RollbackLXCSnapshot(context.Background(), "n", 1, "s")

			return e
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.ErrorIs(t, testCase.call(svc), sentinel)
		})
	}
}
