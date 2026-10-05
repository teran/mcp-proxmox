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
		gw := &mockPBSGateway{restoreFile: func(ctx context.Context, store, backupType, backupID, snapshot string, req model.PBSFileRestoreRequest) error {
			gotReq = req
			return nil
		}}
		req := model.PBSFileRestoreRequest{Path: "/a", Target: "/b"}
		err := (&PBSService{gw: gw}).RestoreFile(context.Background(), "backup", "vm", "100", "snap1", req)
		require.NoError(t, err)
		assert.Equal(t, req, gotReq)
	})

	t.Run("RestoreVMBackup", func(t *testing.T) {
		gotReq := model.PBSVMRestoreRequest{}
		gw := &mockPBSGateway{restoreVMBackup: func(ctx context.Context, store, backupType, backupID, snapshot string, req model.PBSVMRestoreRequest) (string, error) {
			gotReq = req
			return "UPID:restore", nil
		}}
		req := model.PBSVMRestoreRequest{Target: "local", VMID: "next"}
		upid, err := (&PBSService{gw: gw}).RestoreVMBackup(context.Background(), "backup", "vm", "100", "snap1", req)
		require.NoError(t, err)
		assert.Equal(t, "UPID:restore", upid)
		assert.Equal(t, req, gotReq)
	})
}

// TestPBSService_RestoreErrorPropagation verifies gateway errors propagate
// unchanged.
func TestPBSService_RestoreErrorPropagation(t *testing.T) {
	sentinel := errors.New("pbs boom")
	gw := &mockPBSGateway{}
	gw.restoreFile = func(ctx context.Context, store, backupType, backupID, snapshot string, req model.PBSFileRestoreRequest) error {
		return sentinel
	}
	gw.restoreVMBackup = func(ctx context.Context, store, backupType, backupID, snapshot string, req model.PBSVMRestoreRequest) (string, error) {
		return "", sentinel
	}

	svc := &PBSService{gw: gw}
	cases := []struct {
		name string
		call func(*PBSService) error
	}{
		{"RestoreFile", func(s *PBSService) error {
			return s.RestoreFile(context.Background(), "s", "vm", "100", "snap", model.PBSFileRestoreRequest{})
		}},
		{"RestoreVMBackup", func(s *PBSService) error {
			_, e := s.RestoreVMBackup(context.Background(), "s", "vm", "100", "snap", model.PBSVMRestoreRequest{})
			return e
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.ErrorIs(t, tc.call(svc), sentinel)
		})
	}
}
