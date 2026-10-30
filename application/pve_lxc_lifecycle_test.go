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
		gateway := &mockPVEGateway{startLXC: func(_ context.Context, _ string, _ int) (*model.Task, error) {
			return &model.Task{UPID: "UPID:start"}, nil
		}}
		task, err := (&PVEService{gw: gateway}).StartLXC(context.Background(), "pve1", 200)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.Equal(t, "UPID:start", task.UPID)
	})

	t.Run("StopLXC", func(t *testing.T) {
		gotSkip, gotForce := false, false
		gateway := &mockPVEGateway{stopLXC: func(_ context.Context, _ string, _ int, skiplock, forceStop bool) (*model.Task, error) {
			gotSkip, gotForce = skiplock, forceStop

			return &model.Task{UPID: "UPID:stop"}, nil
		}}
		task, err := (&PVEService{gw: gateway}).StopLXC(context.Background(), "pve1", 200, true, true)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.True(t, gotSkip)
		assert.True(t, gotForce)
		assert.Equal(t, "UPID:stop", task.UPID)
	})

	t.Run("ShutdownLXC", func(t *testing.T) {
		gotForce, gotTimeout := false, 0
		gateway := &mockPVEGateway{shutdownLXC: func(
			_ context.Context, _ string, _ int, forceStop bool, timeout int,
		) (*model.Task, error) {
			gotForce, gotTimeout = forceStop, timeout

			return &model.Task{UPID: "UPID:shutdown"}, nil
		}}
		task, err := (&PVEService{gw: gateway}).ShutdownLXC(context.Background(), "pve1", 200, true, 45)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.True(t, gotForce)
		assert.Equal(t, 45, gotTimeout)
		assert.Equal(t, "UPID:shutdown", task.UPID)
	})
}

// TestPVEService_LXCLifecycleDeleteSuccess exercises the destructive LXC
// lifecycle use cases (RebootLXC/DeleteLXC).
func TestPVEService_LXCLifecycleDeleteSuccess(t *testing.T) {
	t.Run("RebootLXC", func(t *testing.T) {
		gotTimeout := 0
		gateway := &mockPVEGateway{rebootLXC: func(_ context.Context, _ string, _ int, timeout int) (*model.Task, error) {
			gotTimeout = timeout

			return &model.Task{UPID: "UPID:reboot"}, nil
		}}
		task, err := (&PVEService{gw: gateway}).RebootLXC(context.Background(), "pve1", 200, 30)
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.Equal(t, 30, gotTimeout)
		assert.Equal(t, "UPID:reboot", task.UPID)
	})

	t.Run("DeleteLXC", func(t *testing.T) {
		gotPurge, gotDestroy, gotForce := false, false, false
		gateway := &mockPVEGateway{deleteLXC: func(
			_ context.Context, _ string, _ int, purge, destroyUnreferencedDisks, force bool,
		) (*model.Task, error) {
			gotPurge, gotDestroy, gotForce = purge, destroyUnreferencedDisks, force

			return &model.Task{UPID: "UPID:delete"}, nil
		}}
		task, err := (&PVEService{gw: gateway}).DeleteLXC(context.Background(), "pve1", 200, true, true, true)
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
	gateway := &mockPVEGateway{}
	gateway.startLXC = func(_ context.Context, _ string, _ int) (*model.Task, error) { return nil, sentinel }
	gateway.stopLXC = func(_ context.Context, _ string, _ int, _, _ bool) (*model.Task, error) {
		return nil, sentinel
	}
	gateway.shutdownLXC = func(_ context.Context, _ string, _ int, _ bool, _ int) (*model.Task, error) {
		return nil, sentinel
	}
	gateway.rebootLXC = func(_ context.Context, _ string, _ int, _ int) (*model.Task, error) {
		return nil, sentinel
	}
	gateway.deleteLXC = func(_ context.Context, _ string, _ int, _, _, _ bool) (*model.Task, error) {
		return nil, sentinel
	}

	svc := &PVEService{gw: gateway}
	cases := []struct {
		name string
		call func(*PVEService) error
	}{
		{"StartLXC", func(s *PVEService) error {
			_, e := s.StartLXC(context.Background(), "n", 1)

			return e
		}},
		{"StopLXC", func(s *PVEService) error {
			_, e := s.StopLXC(context.Background(), "n", 1, false, false)

			return e
		}},
		{"ShutdownLXC", func(s *PVEService) error {
			_, e := s.ShutdownLXC(context.Background(), "n", 1, false, 0)

			return e
		}},
		{"RebootLXC", func(s *PVEService) error {
			_, e := s.RebootLXC(context.Background(), "n", 1, 0)

			return e
		}},
		{"DeleteLXC", func(s *PVEService) error {
			_, e := s.DeleteLXC(context.Background(), "n", 1, false, false, false)

			return e
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.ErrorIs(t, testCase.call(svc), sentinel)
		})
	}
}
