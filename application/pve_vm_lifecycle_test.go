package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/model"
)

// TestPVEService_VMLifecycleSuccess exercises the QEMU VM lifecycle use cases
// (StartVM/StopVM/ShutdownVM/RebootVM/ResetVM/SuspendVM/ResumeVM/DeleteVM)
// through the PVEGateway mock. Each asserts that the gateway is invoked with
// the exact arguments and that the returned Task is passed through unchanged.
func TestPVEService_VMLifecycleSuccess(t *testing.T) {
	t.Run("StartVM", func(t *testing.T) {
		gw := &mockPVEGateway{startVM: func(ctx context.Context, node string, vmid int) (*model.Task, error) {
			return &model.Task{UPID: "UPID:start"}, nil
		}}
		task, err := (&PVEService{gw: gw}).StartVM(context.Background(), "pve1", 100)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.Equal(t, "UPID:start", task.UPID)
	})

	t.Run("StopVM", func(t *testing.T) {
		gotSkip := false
		gw := &mockPVEGateway{stopVM: func(ctx context.Context, node string, vmid int, skiplock bool) (*model.Task, error) {
			gotSkip = skiplock
			return &model.Task{UPID: "UPID:stop"}, nil
		}}
		task, err := (&PVEService{gw: gw}).StopVM(context.Background(), "pve1", 100, true)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.True(t, gotSkip)
		assert.Equal(t, "UPID:stop", task.UPID)
	})

	t.Run("ShutdownVM", func(t *testing.T) {
		gotForce, gotTimeout := false, 0
		gw := &mockPVEGateway{shutdownVM: func(ctx context.Context, node string, vmid int, forceStop bool, timeout int) (*model.Task, error) {
			gotForce, gotTimeout = forceStop, timeout
			return &model.Task{UPID: "UPID:shutdown"}, nil
		}}
		task, err := (&PVEService{gw: gw}).ShutdownVM(context.Background(), "pve1", 100, true, 30)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.True(t, gotForce)
		assert.Equal(t, 30, gotTimeout)
		assert.Equal(t, "UPID:shutdown", task.UPID)
	})

	t.Run("RebootVM", func(t *testing.T) {
		gotTimeout := 0
		gw := &mockPVEGateway{rebootVM: func(ctx context.Context, node string, vmid int, timeout int) (*model.Task, error) {
			gotTimeout = timeout
			return &model.Task{UPID: "UPID:reboot"}, nil
		}}
		task, err := (&PVEService{gw: gw}).RebootVM(context.Background(), "pve1", 100, 60)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.Equal(t, 60, gotTimeout)
		assert.Equal(t, "UPID:reboot", task.UPID)
	})

	t.Run("ResetVM", func(t *testing.T) {
		gw := &mockPVEGateway{resetVM: func(ctx context.Context, node string, vmid int) (*model.Task, error) {
			return &model.Task{UPID: "UPID:reset"}, nil
		}}
		task, err := (&PVEService{gw: gw}).ResetVM(context.Background(), "pve1", 100)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.Equal(t, "UPID:reset", task.UPID)
	})

	t.Run("SuspendVM", func(t *testing.T) {
		gotToDisk := false
		gw := &mockPVEGateway{suspendVM: func(ctx context.Context, node string, vmid int, todisk bool) (*model.Task, error) {
			gotToDisk = todisk
			return &model.Task{UPID: "UPID:suspend"}, nil
		}}
		task, err := (&PVEService{gw: gw}).SuspendVM(context.Background(), "pve1", 100, true)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.True(t, gotToDisk)
		assert.Equal(t, "UPID:suspend", task.UPID)
	})

	t.Run("ResumeVM", func(t *testing.T) {
		gw := &mockPVEGateway{resumeVM: func(ctx context.Context, node string, vmid int) (*model.Task, error) {
			return &model.Task{UPID: "UPID:resume"}, nil
		}}
		task, err := (&PVEService{gw: gw}).ResumeVM(context.Background(), "pve1", 100)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.Equal(t, "UPID:resume", task.UPID)
	})

	t.Run("DeleteVM", func(t *testing.T) {
		gotPurge, gotDestroy := false, false
		gw := &mockPVEGateway{deleteVM: func(ctx context.Context, node string, vmid int, purge, destroyUnreferencedDisks bool) (*model.Task, error) {
			gotPurge, gotDestroy = purge, destroyUnreferencedDisks
			return &model.Task{UPID: "UPID:delete"}, nil
		}}
		task, err := (&PVEService{gw: gw}).DeleteVM(context.Background(), "pve1", 100, true, true)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.True(t, gotPurge)
		assert.True(t, gotDestroy)
		assert.Equal(t, "UPID:delete", task.UPID)
	})
}

// TestPVEService_VMLifecycleErrorPropagation verifies gateway errors propagate
// unchanged through the thin VM lifecycle service methods.
func TestPVEService_VMLifecycleErrorPropagation(t *testing.T) {
	sentinel := errors.New("pve boom")
	gw := &mockPVEGateway{}
	gw.startVM = func(ctx context.Context, node string, vmid int) (*model.Task, error) { return nil, sentinel }
	gw.stopVM = func(ctx context.Context, node string, vmid int, skiplock bool) (*model.Task, error) {
		return nil, sentinel
	}
	gw.shutdownVM = func(ctx context.Context, node string, vmid int, forceStop bool, timeout int) (*model.Task, error) {
		return nil, sentinel
	}
	gw.rebootVM = func(ctx context.Context, node string, vmid int, timeout int) (*model.Task, error) {
		return nil, sentinel
	}
	gw.resetVM = func(ctx context.Context, node string, vmid int) (*model.Task, error) { return nil, sentinel }
	gw.suspendVM = func(ctx context.Context, node string, vmid int, todisk bool) (*model.Task, error) {
		return nil, sentinel
	}
	gw.resumeVM = func(ctx context.Context, node string, vmid int) (*model.Task, error) { return nil, sentinel }
	gw.deleteVM = func(ctx context.Context, node string, vmid int, purge, destroyUnreferencedDisks bool) (*model.Task, error) {
		return nil, sentinel
	}

	svc := &PVEService{gw: gw}
	cases := []struct {
		name string
		call func(*PVEService) error
	}{
		{"StartVM", func(s *PVEService) error { _, e := s.StartVM(context.Background(), "n", 1); return e }},
		{"StopVM", func(s *PVEService) error { _, e := s.StopVM(context.Background(), "n", 1, false); return e }},
		{"ShutdownVM", func(s *PVEService) error { _, e := s.ShutdownVM(context.Background(), "n", 1, false, 0); return e }},
		{"RebootVM", func(s *PVEService) error { _, e := s.RebootVM(context.Background(), "n", 1, 0); return e }},
		{"ResetVM", func(s *PVEService) error { _, e := s.ResetVM(context.Background(), "n", 1); return e }},
		{"SuspendVM", func(s *PVEService) error { _, e := s.SuspendVM(context.Background(), "n", 1, false); return e }},
		{"ResumeVM", func(s *PVEService) error { _, e := s.ResumeVM(context.Background(), "n", 1); return e }},
		{"DeleteVM", func(s *PVEService) error { _, e := s.DeleteVM(context.Background(), "n", 1, false, false); return e }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.ErrorIs(t, tc.call(svc), sentinel)
		})
	}
}
