package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/model"
)

// TestPVEService_LXCCloneSnapshotSuccess exercises the LXC clone and snapshot
// use cases through the PVEGateway mock.
func TestPVEService_LXCCloneSnapshotSuccess(t *testing.T) {
	t.Run("CloneLXC", func(t *testing.T) {
		gotReq := model.CloneLXCRequest{}
		gw := &mockPVEGateway{cloneLXC: func(ctx context.Context, node string, vmid int, req model.CloneLXCRequest) (*model.Task, error) {
			gotReq = req
			return &model.Task{UPID: "UPID:clone"}, nil
		}}
		req := model.CloneLXCRequest{NewID: 201, Hostname: "ct-clone", Full: true}
		task, err := (&PVEService{gw: gw}).CloneLXC(context.Background(), "pve1", 200, req)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.Equal(t, "UPID:clone", task.UPID)
		assert.Equal(t, req, gotReq)
	})

	t.Run("ListLXCSnapshots", func(t *testing.T) {
		gw := &mockPVEGateway{listLXCSnapshots: func(ctx context.Context, node string, vmid int) ([]model.Snapshot, error) {
			return []model.Snapshot{{Name: "snap1"}}, nil
		}}
		snaps, err := (&PVEService{gw: gw}).ListLXCSnapshots(context.Background(), "pve1", 200)
		require.NoError(t, err)
		require.Len(t, snaps, 1)
		assert.Equal(t, "snap1", snaps[0].Name)
	})

	t.Run("CreateLXCSnapshot", func(t *testing.T) {
		gotReq := model.SnapshotCreateRequest{}
		gw := &mockPVEGateway{createLXCSnapshot: func(ctx context.Context, node string, vmid int, req model.SnapshotCreateRequest) (*model.Task, error) {
			gotReq = req
			return &model.Task{UPID: "UPID:create"}, nil
		}}
		req := model.SnapshotCreateRequest{Snapname: "snap1", Description: "d"}
		task, err := (&PVEService{gw: gw}).CreateLXCSnapshot(context.Background(), "pve1", 200, req)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.Equal(t, "UPID:create", task.UPID)
		assert.Equal(t, req, gotReq)
	})

	t.Run("DeleteLXCSnapshot", func(t *testing.T) {
		gw := &mockPVEGateway{deleteLXCSnapshot: func(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
			return &model.Task{UPID: "UPID:delete"}, nil
		}}
		task, err := (&PVEService{gw: gw}).DeleteLXCSnapshot(context.Background(), "pve1", 200, "snap1")
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.Equal(t, "UPID:delete", task.UPID)
	})

	t.Run("RollbackLXCSnapshot", func(t *testing.T) {
		gw := &mockPVEGateway{rollbackLXCSnapshot: func(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
			return &model.Task{UPID: "UPID:rollback"}, nil
		}}
		task, err := (&PVEService{gw: gw}).RollbackLXCSnapshot(context.Background(), "pve1", 200, "snap1")
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.Equal(t, "UPID:rollback", task.UPID)
	})
}

// TestPVEService_LXCCloneSnapshotErrorPropagation verifies gateway errors
// propagate unchanged.
func TestPVEService_LXCCloneSnapshotErrorPropagation(t *testing.T) {
	sentinel := errors.New("pve boom")
	gw := &mockPVEGateway{}
	gw.cloneLXC = func(ctx context.Context, node string, vmid int, req model.CloneLXCRequest) (*model.Task, error) {
		return nil, sentinel
	}
	gw.listLXCSnapshots = func(ctx context.Context, node string, vmid int) ([]model.Snapshot, error) { return nil, sentinel }
	gw.createLXCSnapshot = func(ctx context.Context, node string, vmid int, req model.SnapshotCreateRequest) (*model.Task, error) {
		return nil, sentinel
	}
	gw.deleteLXCSnapshot = func(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
		return nil, sentinel
	}
	gw.rollbackLXCSnapshot = func(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
		return nil, sentinel
	}

	svc := &PVEService{gw: gw}
	cases := []struct {
		name string
		call func(*PVEService) error
	}{
		{"CloneLXC", func(s *PVEService) error {
			_, e := s.CloneLXC(context.Background(), "n", 1, model.CloneLXCRequest{})
			return e
		}},
		{"ListLXCSnapshots", func(s *PVEService) error { _, e := s.ListLXCSnapshots(context.Background(), "n", 1); return e }},
		{"CreateLXCSnapshot", func(s *PVEService) error {
			_, e := s.CreateLXCSnapshot(context.Background(), "n", 1, model.SnapshotCreateRequest{})
			return e
		}},
		{"DeleteLXCSnapshot", func(s *PVEService) error { _, e := s.DeleteLXCSnapshot(context.Background(), "n", 1, "s"); return e }},
		{"RollbackLXCSnapshot", func(s *PVEService) error { _, e := s.RollbackLXCSnapshot(context.Background(), "n", 1, "s"); return e }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.ErrorIs(t, tc.call(svc), sentinel)
		})
	}
}
