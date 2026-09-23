package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/model"
)

// TestPVEService_LXCLifecycleSuccess exercises the LXC lifecycle use cases
// (StartLXC/StopLXC/ShutdownLXC/RebootLXC/DeleteLXC) through the PVEGateway mock.
func TestPVEService_LXCLifecycleSuccess(t *testing.T) {
	t.Run("StartLXC", func(t *testing.T) {
		gw := &mockPVEGateway{startLXC: func(ctx context.Context, node string, vmid int) (*model.Task, error) {
			return &model.Task{UPID: "UPID:start"}, nil
		}}
		task, err := (&PVEService{gw: gw}).StartLXC(context.Background(), "pve1", 200)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.Equal(t, "UPID:start", task.UPID)
	})

	t.Run("StopLXC", func(t *testing.T) {
		gotSkip, gotForce := false, false
		gw := &mockPVEGateway{stopLXC: func(ctx context.Context, node string, vmid int, skiplock, forceStop bool) (*model.Task, error) {
			gotSkip, gotForce = skiplock, forceStop
			return &model.Task{UPID: "UPID:stop"}, nil
		}}
		task, err := (&PVEService{gw: gw}).StopLXC(context.Background(), "pve1", 200, true, true)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.True(t, gotSkip)
		assert.True(t, gotForce)
		assert.Equal(t, "UPID:stop", task.UPID)
	})

	t.Run("ShutdownLXC", func(t *testing.T) {
		gotForce, gotTimeout := false, 0
		gw := &mockPVEGateway{shutdownLXC: func(ctx context.Context, node string, vmid int, forceStop bool, timeout int) (*model.Task, error) {
			gotForce, gotTimeout = forceStop, timeout
			return &model.Task{UPID: "UPID:shutdown"}, nil
		}}
		task, err := (&PVEService{gw: gw}).ShutdownLXC(context.Background(), "pve1", 200, true, 45)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.True(t, gotForce)
		assert.Equal(t, 45, gotTimeout)
		assert.Equal(t, "UPID:shutdown", task.UPID)
	})

	t.Run("RebootLXC", func(t *testing.T) {
		gotTimeout := 0
		gw := &mockPVEGateway{rebootLXC: func(ctx context.Context, node string, vmid int, timeout int) (*model.Task, error) {
			gotTimeout = timeout
			return &model.Task{UPID: "UPID:reboot"}, nil
		}}
		task, err := (&PVEService{gw: gw}).RebootLXC(context.Background(), "pve1", 200, 30)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.Equal(t, 30, gotTimeout)
		assert.Equal(t, "UPID:reboot", task.UPID)
	})

	t.Run("DeleteLXC", func(t *testing.T) {
		gotPurge, gotDestroy, gotForce := false, false, false
		gw := &mockPVEGateway{deleteLXC: func(ctx context.Context, node string, vmid int, purge, destroyUnreferencedDisks, force bool) (*model.Task, error) {
			gotPurge, gotDestroy, gotForce = purge, destroyUnreferencedDisks, force
			return &model.Task{UPID: "UPID:delete"}, nil
		}}
		task, err := (&PVEService{gw: gw}).DeleteLXC(context.Background(), "pve1", 200, true, true, true)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.True(t, gotPurge)
		assert.True(t, gotDestroy)
		assert.True(t, gotForce)
		assert.Equal(t, "UPID:delete", task.UPID)
	})
}

// TestPVEService_LXCLifecycleErrorPropagation verifies gateway errors propagate
// unchanged.
func TestPVEService_LXCLifecycleErrorPropagation(t *testing.T) {
	sentinel := errors.New("pve boom")
	gw := &mockPVEGateway{}
	gw.startLXC = func(ctx context.Context, node string, vmid int) (*model.Task, error) { return nil, sentinel }
	gw.stopLXC = func(ctx context.Context, node string, vmid int, skiplock, forceStop bool) (*model.Task, error) {
		return nil, sentinel
	}
	gw.shutdownLXC = func(ctx context.Context, node string, vmid int, forceStop bool, timeout int) (*model.Task, error) {
		return nil, sentinel
	}
	gw.rebootLXC = func(ctx context.Context, node string, vmid int, timeout int) (*model.Task, error) { return nil, sentinel }
	gw.deleteLXC = func(ctx context.Context, node string, vmid int, purge, destroyUnreferencedDisks, force bool) (*model.Task, error) {
		return nil, sentinel
	}

	svc := &PVEService{gw: gw}
	cases := []struct {
		name string
		call func(*PVEService) error
	}{
		{"StartLXC", func(s *PVEService) error { _, e := s.StartLXC(context.Background(), "n", 1); return e }},
		{"StopLXC", func(s *PVEService) error { _, e := s.StopLXC(context.Background(), "n", 1, false, false); return e }},
		{"ShutdownLXC", func(s *PVEService) error { _, e := s.ShutdownLXC(context.Background(), "n", 1, false, 0); return e }},
		{"RebootLXC", func(s *PVEService) error { _, e := s.RebootLXC(context.Background(), "n", 1, 0); return e }},
		{"DeleteLXC", func(s *PVEService) error { _, e := s.DeleteLXC(context.Background(), "n", 1, false, false, false); return e }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.ErrorIs(t, tc.call(svc), sentinel)
		})
	}
}
