package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/model"
)

// TestPBSService_RestoreSuccess exercises the PBS restore use cases
// (RestoreFile/RestoreVMBackup) through the PBSGateway mock.
func TestPBSService_RestoreSuccess(t *testing.T) {
	t.Run("RestoreFile", func(t *testing.T) {
		gotReq := model.PBSFileRestoreRequest{}
		var gotTime int64
		gateway := &mockPBSGateway{restoreFile: func(
			_ context.Context, _, _, _ string, backupTime int64, req model.PBSFileRestoreRequest,
		) error {
			gotReq = req
			gotTime = backupTime

			return nil
		}}
		req := model.PBSFileRestoreRequest{Path: "/a", Target: "/b"}
		err := (&PBSService{gw: gateway}).RestoreFile(context.Background(), "backup", "vm", "100", 1700000000, req)
		require.NoError(t, err)
		assert.Equal(t, req, gotReq)
		assert.Equal(t, int64(1700000000), gotTime)
	})

	t.Run("RestoreVMBackup", func(t *testing.T) {
		gotReq := model.PBSVMRestoreRequest{}
		var gotTime int64
		gateway := &mockPBSGateway{restoreVMBackup: func(
			_ context.Context, _, _, _ string, backupTime int64, req model.PBSVMRestoreRequest,
		) (string, error) {
			gotReq = req
			gotTime = backupTime

			return "UPID:restore", nil
		}}
		req := model.PBSVMRestoreRequest{Target: "local", VMID: "next"}
		upid, err := (&PBSService{gw: gateway}).RestoreVMBackup(context.Background(), "backup", "vm", "100", 1700000000, req)
		require.NoError(t, err)
		assert.Equal(t, "UPID:restore", upid)
		assert.Equal(t, req, gotReq)
		assert.Equal(t, int64(1700000000), gotTime)
	})
}

// TestPBSService_RestoreErrorPropagation verifies gateway errors propagate
// unchanged.
func TestPBSService_RestoreErrorPropagation(t *testing.T) {
	sentinel := errors.New("pbs boom")
	gateway := &mockPBSGateway{}
	gateway.restoreFile = func(_ context.Context, _, _, _ string, _ int64, _ model.PBSFileRestoreRequest) error {
		return sentinel
	}
	gateway.restoreVMBackup = func(_ context.Context, _, _, _ string, _ int64, _ model.PBSVMRestoreRequest) (string, error) {
		return "", sentinel
	}

	svc := &PBSService{gw: gateway}
	cases := []struct {
		name string
		call func(*PBSService) error
	}{
		{"RestoreFile", func(s *PBSService) error {
			return s.RestoreFile(context.Background(), "s", "vm", "100", 1700000000, model.PBSFileRestoreRequest{})
		}},
		{"RestoreVMBackup", func(s *PBSService) error {
			_, e := s.RestoreVMBackup(context.Background(), "s", "vm", "100", 1700000000, model.PBSVMRestoreRequest{})

			return e
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.ErrorIs(t, testCase.call(svc), sentinel)
		})
	}
}
