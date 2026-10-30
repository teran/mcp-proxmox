package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/model"
)

// TestPBSService_ReadOnlySuccess exercises the PBS read-only use cases
// (ListBackupFiles/GetTaskStatus/GetTaskLog) through the PBSGateway mock.
func TestPBSService_ReadOnlySuccess(t *testing.T) {
	t.Run("ListBackupFiles", func(t *testing.T) {
		gotPath := ""
		gateway := &mockPBSGateway{listBackupFiles: func(_ context.Context, _, _, _, _, path string) ([]model.PBSFile, error) {
			gotPath = path

			return []model.PBSFile{{Filename: "f"}}, nil
		}}
		files, err := (&PBSService{gw: gateway}).ListBackupFiles(context.Background(), "backup", "vm", "100", "snap", "/etc")
		require.NoError(t, err)
		require.Len(t, files, 1)
		assert.Equal(t, "f", files[0].Filename)
		assert.Equal(t, "/etc", gotPath)
	})

	t.Run("GetTaskStatus", func(t *testing.T) {
		gateway := &mockPBSGateway{getTaskStatus: func(_ context.Context, upid string) (*model.PBSTask, error) {
			return &model.PBSTask{UPID: upid, Status: "running"}, nil
		}}
		task, err := (&PBSService{gw: gateway}).GetTaskStatus(context.Background(), "upid1")
		require.NoError(t, err)
		require.NotNil(t, task)
		assert.Equal(t, "running", task.Status)
	})

	t.Run("GetTaskLog", func(t *testing.T) {
		gotLimit := 0
		gateway := &mockPBSGateway{getTaskLog: func(_ context.Context, _ string, limit int) ([]model.TaskLogEntry, error) {
			gotLimit = limit

			return []model.TaskLogEntry{{LineNumber: 1, Text: "OK"}}, nil
		}}
		entries, err := (&PBSService{gw: gateway}).GetTaskLog(context.Background(), "upid1", 50)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		assert.Equal(t, 50, gotLimit)
	})
}

// TestPBSService_ReadOnlyErrorPropagation verifies gateway errors propagate
// unchanged.
func TestPBSService_ReadOnlyErrorPropagation(t *testing.T) {
	sentinel := errors.New("pbs boom")
	gateway := &mockPBSGateway{}
	gateway.listBackupFiles = func(_ context.Context, _, _, _, _, _ string) ([]model.PBSFile, error) {
		return nil, sentinel
	}
	gateway.getTaskStatus = func(_ context.Context, _ string) (*model.PBSTask, error) { return nil, sentinel }
	gateway.getTaskLog = func(_ context.Context, _ string, _ int) ([]model.TaskLogEntry, error) {
		return nil, sentinel
	}

	svc := &PBSService{gw: gateway}
	cases := []struct {
		name string
		call func(*PBSService) error
	}{
		{"ListBackupFiles", func(s *PBSService) error {
			_, e := s.ListBackupFiles(context.Background(), "s", "vm", "100", "snap", "")

			return e
		}},
		{"GetTaskStatus", func(s *PBSService) error {
			_, e := s.GetTaskStatus(context.Background(), "u")

			return e
		}},
		{"GetTaskLog", func(s *PBSService) error {
			_, e := s.GetTaskLog(context.Background(), "u", 0)

			return e
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.ErrorIs(t, testCase.call(svc), sentinel)
		})
	}
}
