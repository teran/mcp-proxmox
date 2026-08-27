package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/model"
)

func TestPBSService_Success(t *testing.T) {
	t.Run("ListDatastores", func(t *testing.T) {
		gw := &mockPBSGateway{listDatastores: func(ctx context.Context) ([]model.Datastore, error) {
			return []model.Datastore{{Name: "backup"}}, nil
		}}
		got, err := (&PBSService{gw: gw}).ListDatastores(context.Background())
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("GetDatastoreStatus", func(t *testing.T) {
		gw := &mockPBSGateway{getDatastoreStats: func(ctx context.Context, store string) (*model.DatastoreStatus, error) {
			return &model.DatastoreStatus{Store: store, Total: 1000}, nil
		}}
		got, err := (&PBSService{gw: gw}).GetDatastoreStatus(context.Background(), "backup")
		require.NoError(t, err)
		assert.Equal(t, int64(1000), got.Total)
	})

	t.Run("ListBackups", func(t *testing.T) {
		gw := &mockPBSGateway{listBackups: func(ctx context.Context, store string) ([]model.Backup, error) {
			return []model.Backup{{BackupID: "vm/100/..."}}, nil
		}}
		got, err := (&PBSService{gw: gw}).ListBackups(context.Background(), "backup")
		require.NoError(t, err)
		assert.Len(t, got, 1)
	})

	t.Run("GetBackup", func(t *testing.T) {
		gw := &mockPBSGateway{getBackup: func(ctx context.Context, store, snapshot string) (*model.Backup, error) {
			return &model.Backup{BackupID: snapshot}, nil
		}}
		got, err := (&PBSService{gw: gw}).GetBackup(context.Background(), "backup", "vm/100/...")
		require.NoError(t, err)
		assert.Equal(t, "vm/100/...", got.BackupID)
	})

	t.Run("GetBackupNotes", func(t *testing.T) {
		gw := &mockPBSGateway{getBackupNotes: func(ctx context.Context, store, snapshot string) (*model.BackupNotes, error) {
			return &model.BackupNotes{Snapshot: snapshot, Notes: "keep"}, nil
		}}
		got, err := (&PBSService{gw: gw}).GetBackupNotes(context.Background(), "backup", "vm/100/...")
		require.NoError(t, err)
		assert.Equal(t, "keep", got.Notes)
	})

	t.Run("GetVerifyStatus", func(t *testing.T) {
		gw := &mockPBSGateway{getVerifyStatus: func(ctx context.Context, store, upid string) (*model.VerifyStatus, error) {
			return &model.VerifyStatus{UPID: upid, Status: "ok"}, nil
		}}
		got, err := (&PBSService{gw: gw}).GetVerifyStatus(context.Background(), "backup", "UPID:...")
		require.NoError(t, err)
		assert.Equal(t, "ok", got.Status)
	})

	t.Run("GetPruneStatus", func(t *testing.T) {
		gw := &mockPBSGateway{getPruneStatus: func(ctx context.Context, store, upid string) (*model.PruneStatus, error) {
			return &model.PruneStatus{UPID: upid, Status: "running"}, nil
		}}
		got, err := (&PBSService{gw: gw}).GetPruneStatus(context.Background(), "backup", "UPID:...")
		require.NoError(t, err)
		assert.Equal(t, "running", got.Status)
	})

	t.Run("GetPBSVersion", func(t *testing.T) {
		gw := &mockPBSGateway{getPBSVersion: func(ctx context.Context) (*model.PBSVersion, error) {
			return &model.PBSVersion{Version: "3.2.3"}, nil
		}}
		got, err := (&PBSService{gw: gw}).GetPBSVersion(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "3.2.3", got.Version)
	})
}

func TestPBSService_ErrorPropagation(t *testing.T) {
	sentinel := errors.New("pbs boom")

	cases := []struct {
		name string
		call func(*PBSService) error
	}{
		{"ListDatastores", func(s *PBSService) error { _, e := s.ListDatastores(context.Background()); return e }},
		{"GetDatastoreStatus", func(s *PBSService) error { _, e := s.GetDatastoreStatus(context.Background(), "s"); return e }},
		{"ListBackups", func(s *PBSService) error { _, e := s.ListBackups(context.Background(), "s"); return e }},
		{"GetBackup", func(s *PBSService) error { _, e := s.GetBackup(context.Background(), "s", "snap"); return e }},
		{"GetBackupNotes", func(s *PBSService) error { _, e := s.GetBackupNotes(context.Background(), "s", "snap"); return e }},
		{"GetVerifyStatus", func(s *PBSService) error { _, e := s.GetVerifyStatus(context.Background(), "s", "u"); return e }},
		{"GetPruneStatus", func(s *PBSService) error { _, e := s.GetPruneStatus(context.Background(), "s", "u"); return e }},
		{"GetPBSVersion", func(s *PBSService) error { _, e := s.GetPBSVersion(context.Background()); return e }},
	}

	gw := &mockPBSGateway{}
	gw.listDatastores = func(ctx context.Context) ([]model.Datastore, error) { return nil, sentinel }
	gw.getDatastoreStats = func(ctx context.Context, store string) (*model.DatastoreStatus, error) { return nil, sentinel }
	gw.listBackups = func(ctx context.Context, store string) ([]model.Backup, error) { return nil, sentinel }
	gw.getBackup = func(ctx context.Context, store, snapshot string) (*model.Backup, error) { return nil, sentinel }
	gw.getBackupNotes = func(ctx context.Context, store, snapshot string) (*model.BackupNotes, error) { return nil, sentinel }
	gw.getVerifyStatus = func(ctx context.Context, store, upid string) (*model.VerifyStatus, error) { return nil, sentinel }
	gw.getPruneStatus = func(ctx context.Context, store, upid string) (*model.PruneStatus, error) { return nil, sentinel }
	gw.getPBSVersion = func(ctx context.Context) (*model.PBSVersion, error) { return nil, sentinel }

	svc := &PBSService{gw: gw}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.call(svc)
			assert.ErrorIs(t, err, sentinel)
		})
	}
}
