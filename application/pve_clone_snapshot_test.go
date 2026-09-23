package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/model"
)

// TestPVEService_CloneSnapshotSuccess exercises the QEMU VM clone and snapshot
// use cases (CloneVM/ListVMSnapshots/CreateVMSnapshot/DeleteVMSnapshot/
// RollbackVMSnapshot) through the PVEGateway mock.
func TestPVEService_CloneSnapshotSuccess(t *testing.T) {
	t.Run("CloneVM", func(t *testing.T) {
		gotReq := model.CloneVMRequest{}
		gw := &mockPVEGateway{cloneVM: func(ctx context.Context, node string, vmid int, req model.CloneVMRequest) (*model.Task, error) {
			gotReq = req
			return &model.Task{UPID: "UPID:clone"}, nil
		}}
		req := model.CloneVMRequest{NewID: 101, Name: "web-clone", Full: true}
		task, err := (&PVEService{gw: gw}).CloneVM(context.Background(), "pve1", 100, req)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.Equal(t, "UPID:clone", task.UPID)
		assert.Equal(t, req, gotReq)
	})

	t.Run("ListVMSnapshots", func(t *testing.T) {
		gw := &mockPVEGateway{listVMSnapshots: func(ctx context.Context, node string, vmid int) ([]model.Snapshot, error) {
			return []model.Snapshot{{Name: "snap1"}}, nil
		}}
		snaps, err := (&PVEService{gw: gw}).ListVMSnapshots(context.Background(), "pve1", 100)
		require.NoError(t, err)
		require.Len(t, snaps, 1)
		assert.Equal(t, "snap1", snaps[0].Name)
	})

	t.Run("CreateVMSnapshot", func(t *testing.T) {
		gotReq := model.SnapshotCreateRequest{}
		gw := &mockPVEGateway{createVMSnapshot: func(ctx context.Context, node string, vmid int, req model.SnapshotCreateRequest) (*model.Task, error) {
			gotReq = req
			return &model.Task{UPID: "UPID:create"}, nil
		}}
		req := model.SnapshotCreateRequest{Snapname: "snap1", VMState: true}
		task, err := (&PVEService{gw: gw}).CreateVMSnapshot(context.Background(), "pve1", 100, req)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.Equal(t, "UPID:create", task.UPID)
		assert.Equal(t, req, gotReq)
	})

	t.Run("DeleteVMSnapshot", func(t *testing.T) {
		gw := &mockPVEGateway{deleteVMSnapshot: func(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
			return &model.Task{UPID: "UPID:delete"}, nil
		}}
		task, err := (&PVEService{gw: gw}).DeleteVMSnapshot(context.Background(), "pve1", 100, "snap1")
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.Equal(t, "UPID:delete", task.UPID)
	})

	t.Run("RollbackVMSnapshot", func(t *testing.T) {
		gw := &mockPVEGateway{rollbackVMSnapshot: func(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
			return &model.Task{UPID: "UPID:rollback"}, nil
		}}
		task, err := (&PVEService{gw: gw}).RollbackVMSnapshot(context.Background(), "pve1", 100, "snap1")
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.Equal(t, "UPID:rollback", task.UPID)
	})
}

// TestPVEService_CloneSnapshotErrorPropagation verifies gateway errors propagate
// unchanged.
func TestPVEService_CloneSnapshotErrorPropagation(t *testing.T) {
	sentinel := errors.New("pve boom")
	gw := &mockPVEGateway{}
	gw.cloneVM = func(ctx context.Context, node string, vmid int, req model.CloneVMRequest) (*model.Task, error) {
		return nil, sentinel
	}
	gw.listVMSnapshots = func(ctx context.Context, node string, vmid int) ([]model.Snapshot, error) { return nil, sentinel }
	gw.createVMSnapshot = func(ctx context.Context, node string, vmid int, req model.SnapshotCreateRequest) (*model.Task, error) {
		return nil, sentinel
	}
	gw.deleteVMSnapshot = func(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
		return nil, sentinel
	}
	gw.rollbackVMSnapshot = func(ctx context.Context, node string, vmid int, snapname string) (*model.Task, error) {
		return nil, sentinel
	}

	svc := &PVEService{gw: gw}
	cases := []struct {
		name string
		call func(*PVEService) error
	}{
		{"CloneVM", func(s *PVEService) error { _, e := s.CloneVM(context.Background(), "n", 1, model.CloneVMRequest{}); return e }},
		{"ListVMSnapshots", func(s *PVEService) error { _, e := s.ListVMSnapshots(context.Background(), "n", 1); return e }},
		{"CreateVMSnapshot", func(s *PVEService) error {
			_, e := s.CreateVMSnapshot(context.Background(), "n", 1, model.SnapshotCreateRequest{})
			return e
		}},
		{"DeleteVMSnapshot", func(s *PVEService) error { _, e := s.DeleteVMSnapshot(context.Background(), "n", 1, "s"); return e }},
		{"RollbackVMSnapshot", func(s *PVEService) error { _, e := s.RollbackVMSnapshot(context.Background(), "n", 1, "s"); return e }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.ErrorIs(t, tc.call(svc), sentinel)
		})
	}
}
