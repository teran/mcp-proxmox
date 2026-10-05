package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/model"
)

// TestPVEService_RestoreVM_Success exercises the RestoreVM use case.
func TestPVEService_RestoreVM_Success(t *testing.T) {
	gotReq := model.PVERestoreRequest{}
	gw := &mockPVEGateway{restoreVM: func(ctx context.Context, node string, req model.PVERestoreRequest) (*model.Task, error) {
		gotReq = req
		return &model.Task{UPID: "UPID:restore"}, nil
	}}
	req := model.PVERestoreRequest{Archive: "/a.bak", VMID: 100, Force: true}
	task, err := (&PVEService{gw: gw}).RestoreVM(context.Background(), "pve1", req)
	require.NoError(t, err)
	require.NotNil(t, task)
	assert.Equal(t, "UPID:restore", task.UPID)
	assert.Equal(t, req, gotReq)
}

// TestPVEService_RestoreVM_ErrorPropagation verifies the gateway error
// propagates unchanged.
func TestPVEService_RestoreVM_ErrorPropagation(t *testing.T) {
	sentinel := errors.New("pve boom")
	gw := &mockPVEGateway{restoreVM: func(ctx context.Context, node string, req model.PVERestoreRequest) (*model.Task, error) {
		return nil, sentinel
	}}
	_, err := (&PVEService{gw: gw}).RestoreVM(context.Background(), "pve1", model.PVERestoreRequest{Archive: "/a.bak"})
	assert.ErrorIs(t, err, sentinel)
}
