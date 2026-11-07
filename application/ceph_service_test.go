package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/model"
)

// TestCephService_ReadSuccess exercises every read use case of CephService
// against the CephGateway mock, asserting the thin pass-through returns the
// gateway's value unchanged (SPEC contract: CephService mirrors the gateway
// 1:1).
func TestCephService_ReadSuccess_Cluster(t *testing.T) {
	t.Run("GetClusterStatus", func(t *testing.T) {
		gateway := &mockCephGateway{getClusterStatus: func(_ context.Context) (*model.CephClusterStatus, error) {
			return &model.CephClusterStatus{Status: "HEALTH_OK", Quorum: []int{1, 2}}, nil
		}}
		got, err := (&CephService{gw: gateway}).GetClusterStatus(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "HEALTH_OK", got.Status)
		assert.Equal(t, []int{1, 2}, got.Quorum)
	})

	t.Run("GetClusterConfig", func(t *testing.T) {
		gateway := &mockCephGateway{getClusterConfig: func(_ context.Context) ([]model.CephConfigEntry, error) {
			return []model.CephConfigEntry{{Name: "mon_allow_pool_delete", Value: "true"}}, nil
		}}
		got, err := (&CephService{gw: gateway}).GetClusterConfig(context.Background())
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "mon_allow_pool_delete", got[0].Name)
	})

	t.Run("GetClusterLog", func(t *testing.T) {
		var gotLimit int
		gateway := &mockCephGateway{getClusterLog: func(_ context.Context, limit int) ([]model.CephLogEntry, error) {
			gotLimit = limit

			return []model.CephLogEntry{{Message: "boot"}}, nil
		}}
		got, err := (&CephService{gw: gateway}).GetClusterLog(context.Background(), 100)
		require.NoError(t, err)
		assert.Equal(t, 100, gotLimit)
		require.Len(t, got, 1)
		assert.Equal(t, "boot", got[0].Message)
	})

	t.Run("GetClusterMetadata", func(t *testing.T) {
		gateway := &mockCephGateway{getClusterMeta: func(_ context.Context) (*model.CephClusterMetadata, error) {
			return &model.CephClusterMetadata{Fsid: "abc", CephVersion: "17.2.7"}, nil
		}}
		got, err := (&CephService{gw: gateway}).GetClusterMetadata(context.Background())
		require.NoError(t, err)
		assert.Equal(t, "abc", got.Fsid)
		assert.Equal(t, "17.2.7", got.CephVersion)
	})
}

func TestCephService_ReadSuccess_NodeState(t *testing.T) {
	t.Run("GetNodeStatus", func(t *testing.T) {
		var gotNode string
		gateway := &mockCephGateway{getNodeStatus: func(_ context.Context, node string) (*model.CephNodeStatus, error) {
			gotNode = node

			return &model.CephNodeStatus{Status: "HEALTH_WARN"}, nil
		}}
		got, err := (&CephService{gw: gateway}).GetNodeStatus(context.Background(), "pve1")
		require.NoError(t, err)
		assert.Equal(t, "pve1", gotNode)
		assert.Equal(t, "HEALTH_WARN", got.Status)
	})

	t.Run("GetNodeConfig", func(t *testing.T) {
		var gotNode string
		gateway := &mockCephGateway{getNodeConfig: func(_ context.Context, node string) ([]model.CephConfigEntry, error) {
			gotNode = node

			return []model.CephConfigEntry{{Name: "n", Value: "v"}}, nil
		}}
		got, err := (&CephService{gw: gateway}).GetNodeConfig(context.Background(), "pve1")
		require.NoError(t, err)
		assert.Equal(t, "pve1", gotNode)
		require.Len(t, got, 1)
		assert.Equal(t, "n", got[0].Name)
	})

	t.Run("GetNodeConfigDB", func(t *testing.T) {
		gateway := &mockCephGateway{getNodeConfigDB: func(_ context.Context, _ string) ([]model.CephConfigEntry, error) {
			return []model.CephConfigEntry{{Name: "db", Value: "1"}}, nil
		}}
		got, err := (&CephService{gw: gateway}).GetNodeConfigDB(context.Background(), "pve1")
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "db", got[0].Name)
	})

	t.Run("GetNodeFlags", func(t *testing.T) {
		gateway := &mockCephGateway{getNodeFlags: func(_ context.Context, _ string) ([]model.CephFlag, error) {
			return []model.CephFlag{{Name: "noout", Enabled: 1}}, nil
		}}
		got, err := (&CephService{gw: gateway}).GetNodeFlags(context.Background(), "pve1")
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "noout", got[0].Name)
		assert.Equal(t, 1, got[0].Enabled)
	})
}

func TestCephService_ReadSuccess_NodeMeta(t *testing.T) {
	t.Run("GetNodeMetadata", func(t *testing.T) {
		gateway := &mockCephGateway{getNodeMeta: func(_ context.Context, _ string) (*model.CephNodeMetadata, error) {
			return &model.CephNodeMetadata{}, nil
		}}
		got, err := (&CephService{gw: gateway}).GetNodeMetadata(context.Background(), "pve1")
		require.NoError(t, err)
		assert.NotNil(t, got)
	})

	t.Run("GetNodeCrush", func(t *testing.T) {
		gateway := &mockCephGateway{getNodeCrush: func(_ context.Context, _ string) (*model.CephCrushMap, error) {
			return &model.CephCrushMap{Rules: []model.CephCrushRule{{ID: 0, RuleName: "r"}}}, nil
		}}
		got, err := (&CephService{gw: gateway}).GetNodeCrush(context.Background(), "pve1")
		require.NoError(t, err)
		require.Len(t, got.Rules, 1)
		assert.Equal(t, "r", got.Rules[0].RuleName)
	})

	t.Run("GetNodeRules", func(t *testing.T) {
		gateway := &mockCephGateway{getNodeRules: func(_ context.Context, _ string) ([]model.CephCrushRule, error) {
			return []model.CephCrushRule{{ID: 0, RuleName: "replicated_rule"}}, nil
		}}
		got, err := (&CephService{gw: gateway}).GetNodeRules(context.Background(), "pve1")
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "replicated_rule", got[0].RuleName)
	})
}

func TestCephService_ReadSuccess_List(t *testing.T) {
	t.Run("ListPools", func(t *testing.T) {
		gateway := &mockCephGateway{listPools: func(_ context.Context, _ string) ([]model.CephPool, error) {
			return []model.CephPool{{ID: 1, PoolName: "rbd"}}, nil
		}}
		got, err := (&CephService{gw: gateway}).ListPools(context.Background(), "pve1")
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "rbd", got[0].PoolName)
	})

	t.Run("ListOSDs", func(t *testing.T) {
		gateway := &mockCephGateway{listOSDs: func(_ context.Context, _ string) ([]model.CephOSD, error) {
			return []model.CephOSD{{ID: 0, Name: "osd.0", Status: "up"}}, nil
		}}
		got, err := (&CephService{gw: gateway}).ListOSDs(context.Background(), "pve1")
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "up", got[0].Status)
	})

	t.Run("ListMons", func(t *testing.T) {
		gateway := &mockCephGateway{listMons: func(_ context.Context, _ string) ([]model.CephMon, error) {
			return []model.CephMon{{ID: 1, Name: "pve1"}}, nil
		}}
		got, err := (&CephService{gw: gateway}).ListMons(context.Background(), "pve1")
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "pve1", got[0].Name)
	})

	t.Run("ListMgrs", func(t *testing.T) {
		gateway := &mockCephGateway{listMgrs: func(_ context.Context, _ string) ([]model.CephMgr, error) {
			return []model.CephMgr{{ID: 1, Name: "pve1", Status: "active"}}, nil
		}}
		got, err := (&CephService{gw: gateway}).ListMgrs(context.Background(), "pve1")
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "active", got[0].Status)
	})
}

// TestCephService_MutationSuccess exercises every mutation use case, asserting
// the request struct and identifiers are passed through to the gateway 1:1.
func TestCephService_MutationSuccess_Crush(t *testing.T) {
	t.Run("CreateCrushRule", func(t *testing.T) {
		var gotNode string
		var gotReq model.CephCrushRuleCreateRequest
		gateway := &mockCephGateway{createCrushRule: func(
			_ context.Context, node string, req model.CephCrushRuleCreateRequest,
		) (string, error) {
			gotNode, gotReq = node, req

			return "replicated_rule", nil
		}}
		req := model.CephCrushRuleCreateRequest{Name: "rule1", Type: "replicated", MinSize: 1, MaxSize: 10}
		out, err := (&CephService{gw: gateway}).CreateCrushRule(context.Background(), "pve1", req)
		require.NoError(t, err)
		assert.Equal(t, "replicated_rule", out)
		assert.Equal(t, "pve1", gotNode)
		assert.Equal(t, req, gotReq)
	})

	t.Run("DeleteCrushRule", func(t *testing.T) {
		var gotNode, gotName string
		gateway := &mockCephGateway{deleteCrushRule: func(_ context.Context, node, name string) error {
			gotNode, gotName = node, name

			return nil
		}}
		require.NoError(t, (&CephService{gw: gateway}).DeleteCrushRule(context.Background(), "pve1", "rule1"))
		assert.Equal(t, "pve1", gotNode)
		assert.Equal(t, "rule1", gotName)
	})
}

func TestCephService_MutationSuccess_Pool(t *testing.T) {
	t.Run("CreatePool", func(t *testing.T) {
		var gotReq model.CephPoolCreateRequest
		gateway := &mockCephGateway{createPool: func(
			_ context.Context, _ string, req model.CephPoolCreateRequest,
		) (string, error) {
			gotReq = req

			return "pool1", nil
		}}
		req := model.CephPoolCreateRequest{Name: "pool1", Size: 3, PGNum: 128}
		out, err := (&CephService{gw: gateway}).CreatePool(context.Background(), "pve1", req)
		require.NoError(t, err)
		assert.Equal(t, "pool1", out)
		assert.Equal(t, req, gotReq)
	})

	t.Run("UpdatePool", func(t *testing.T) {
		var gotName string
		var gotReq model.CephPoolUpdateRequest
		gateway := &mockCephGateway{updatePool: func(
			_ context.Context, _ string, name string, req model.CephPoolUpdateRequest,
		) error {
			gotName, gotReq = name, req

			return nil
		}}
		req := model.CephPoolUpdateRequest{Size: 3, MinSize: 2}
		require.NoError(t, (&CephService{gw: gateway}).UpdatePool(context.Background(), "pve1", "pool1", req))
		assert.Equal(t, "pool1", gotName)
		assert.Equal(t, req, gotReq)
	})

	t.Run("DeletePool", func(t *testing.T) {
		var gotNode, gotName string
		gateway := &mockCephGateway{deletePool: func(_ context.Context, node, name string) error {
			gotNode, gotName = node, name

			return nil
		}}
		require.NoError(t, (&CephService{gw: gateway}).DeletePool(context.Background(), "pve1", "pool1"))
		assert.Equal(t, "pve1", gotNode)
		assert.Equal(t, "pool1", gotName)
	})
}

func TestCephService_MutationSuccess_OSD(t *testing.T) {
	t.Run("CreateOSD", func(t *testing.T) {
		var gotReq model.CephOSDCreateRequest
		gateway := &mockCephGateway{createOSD: func(
			_ context.Context, _ string, req model.CephOSDCreateRequest,
		) (string, error) {
			gotReq = req

			return "osd.0", nil
		}}
		req := model.CephOSDCreateRequest{OSD: "0"}
		out, err := (&CephService{gw: gateway}).CreateOSD(context.Background(), "pve1", req)
		require.NoError(t, err)
		assert.Equal(t, "osd.0", out)
		assert.Equal(t, req, gotReq)
	})

	t.Run("DeleteOSD", func(t *testing.T) {
		var gotNode string
		var gotID int
		gateway := &mockCephGateway{deleteOSD: func(_ context.Context, node string, osdid int) error {
			gotNode, gotID = node, osdid

			return nil
		}}
		require.NoError(t, (&CephService{gw: gateway}).DeleteOSD(context.Background(), "pve1", 3))
		assert.Equal(t, "pve1", gotNode)
		assert.Equal(t, 3, gotID)
	})
}

func TestCephService_MutationSuccess_MonMgr(t *testing.T) {
	t.Run("CreateMon", func(t *testing.T) {
		var gotReq model.CephMonCreateRequest
		gateway := &mockCephGateway{createMon: func(
			_ context.Context, _ string, req model.CephMonCreateRequest,
		) (string, error) {
			gotReq = req

			return "pve2", nil
		}}
		req := model.CephMonCreateRequest{MonAddr: "10.0.0.2:6789"}
		out, err := (&CephService{gw: gateway}).CreateMon(context.Background(), "pve1", req)
		require.NoError(t, err)
		assert.Equal(t, "pve2", out)
		assert.Equal(t, req, gotReq)
	})

	t.Run("DeleteMon", func(t *testing.T) {
		var gotNode, gotMonID string
		gateway := &mockCephGateway{deleteMon: func(_ context.Context, node, monid string) error {
			gotNode, gotMonID = node, monid

			return nil
		}}
		require.NoError(t, (&CephService{gw: gateway}).DeleteMon(context.Background(), "pve1", "pve2"))
		assert.Equal(t, "pve1", gotNode)
		assert.Equal(t, "pve2", gotMonID)
	})

	t.Run("CreateMgr", func(t *testing.T) {
		var gotReq model.CephMgrCreateRequest
		gateway := &mockCephGateway{createMgr: func(
			_ context.Context, _ string, req model.CephMgrCreateRequest,
		) (string, error) {
			gotReq = req

			return "pve2", nil
		}}
		req := model.CephMgrCreateRequest{ID: "pve2"}
		out, err := (&CephService{gw: gateway}).CreateMgr(context.Background(), "pve1", req)
		require.NoError(t, err)
		assert.Equal(t, "pve2", out)
		assert.Equal(t, req, gotReq)
	})
}

func TestCephService_MutationSuccess_Config(t *testing.T) {
	t.Run("SetFlags", func(t *testing.T) {
		var gotReq model.CephFlagsSetRequest
		gateway := &mockCephGateway{setFlags: func(_ context.Context, _ string, req model.CephFlagsSetRequest) error {
			gotReq = req

			return nil
		}}
		req := model.CephFlagsSetRequest{Values: map[string]string{"noout": "true"}}
		require.NoError(t, (&CephService{gw: gateway}).SetFlags(context.Background(), "pve1", req))
		assert.Equal(t, req, gotReq)
	})

	t.Run("SetConfig", func(t *testing.T) {
		var gotReq model.CephConfigSetRequest
		gateway := &mockCephGateway{setConfig: func(_ context.Context, _ string, req model.CephConfigSetRequest) error {
			gotReq = req

			return nil
		}}
		req := model.CephConfigSetRequest{Name: "mon_allow_pool_delete", Value: "true"}
		require.NoError(t, (&CephService{gw: gateway}).SetConfig(context.Background(), "pve1", req))
		assert.Equal(t, req, gotReq)
	})
}

func TestCephService_MutationSuccess_Cluster(t *testing.T) {
	t.Run("InitCeph", func(t *testing.T) {
		var gotReq model.CephInitRequest
		gateway := &mockCephGateway{initCeph: func(
			_ context.Context, _ string, req model.CephInitRequest,
		) (string, error) {
			gotReq = req

			return "UPID:init", nil
		}}
		req := model.CephInitRequest{Network: "10.0.0.0/24", ClusterNetwork: "10.0.1.0/24"}
		out, err := (&CephService{gw: gateway}).InitCeph(context.Background(), "pve1", req)
		require.NoError(t, err)
		assert.Equal(t, "UPID:init", out)
		assert.Equal(t, req, gotReq)
	})

	t.Run("DestroyCeph", func(t *testing.T) {
		var gotReq model.CephDestroyRequest
		gateway := &mockCephGateway{destroyCeph: func(_ context.Context, _ string, req model.CephDestroyRequest) error {
			gotReq = req

			return nil
		}}
		req := model.CephDestroyRequest{Force: true}
		require.NoError(t, (&CephService{gw: gateway}).DestroyCeph(context.Background(), "pve1", req))
		assert.Equal(t, req, gotReq)
	})
}

// cephErrorPropagationCases pairs each CephService method with a call that
// invokes it through the thin service layer, to assert error pass-through.
var cephErrorPropagationCases = []struct {
	name string
	call func(*CephService) error
}{
	{"GetClusterStatus", func(s *CephService) error {
		_, e := s.GetClusterStatus(context.Background())

		return e
	}},
	{"GetClusterConfig", func(s *CephService) error {
		_, e := s.GetClusterConfig(context.Background())

		return e
	}},
	{"GetClusterLog", func(s *CephService) error {
		_, e := s.GetClusterLog(context.Background(), 10)

		return e
	}},
	{"GetClusterMetadata", func(s *CephService) error {
		_, e := s.GetClusterMetadata(context.Background())

		return e
	}},
	{"GetNodeStatus", func(s *CephService) error {
		_, e := s.GetNodeStatus(context.Background(), "n")

		return e
	}},
	{"GetNodeConfig", func(s *CephService) error {
		_, e := s.GetNodeConfig(context.Background(), "n")

		return e
	}},
	{"GetNodeConfigDB", func(s *CephService) error {
		_, e := s.GetNodeConfigDB(context.Background(), "n")

		return e
	}},
	{"GetNodeFlags", func(s *CephService) error {
		_, e := s.GetNodeFlags(context.Background(), "n")

		return e
	}},
	{"GetNodeMetadata", func(s *CephService) error {
		_, e := s.GetNodeMetadata(context.Background(), "n")

		return e
	}},
	{"GetNodeCrush", func(s *CephService) error {
		_, e := s.GetNodeCrush(context.Background(), "n")

		return e
	}},
	{"GetNodeRules", func(s *CephService) error {
		_, e := s.GetNodeRules(context.Background(), "n")

		return e
	}},
	{"ListPools", func(s *CephService) error {
		_, e := s.ListPools(context.Background(), "n")

		return e
	}},
	{"ListOSDs", func(s *CephService) error {
		_, e := s.ListOSDs(context.Background(), "n")

		return e
	}},
	{"ListMons", func(s *CephService) error {
		_, e := s.ListMons(context.Background(), "n")

		return e
	}},
	{"ListMgrs", func(s *CephService) error {
		_, e := s.ListMgrs(context.Background(), "n")

		return e
	}},
	{"CreateCrushRule", func(s *CephService) error {
		_, e := s.CreateCrushRule(context.Background(), "n", model.CephCrushRuleCreateRequest{})

		return e
	}},
	{"DeleteCrushRule", func(s *CephService) error { return s.DeleteCrushRule(context.Background(), "n", "r") }},
	{"CreatePool", func(s *CephService) error {
		_, e := s.CreatePool(context.Background(), "n", model.CephPoolCreateRequest{})

		return e
	}},
	{"UpdatePool", func(s *CephService) error {
		return s.UpdatePool(context.Background(), "n", "p", model.CephPoolUpdateRequest{})
	}},
	{"DeletePool", func(s *CephService) error { return s.DeletePool(context.Background(), "n", "p") }},
	{"CreateOSD", func(s *CephService) error {
		_, e := s.CreateOSD(context.Background(), "n", model.CephOSDCreateRequest{})

		return e
	}},
	{"DeleteOSD", func(s *CephService) error { return s.DeleteOSD(context.Background(), "n", 1) }},
	{"CreateMon", func(s *CephService) error {
		_, e := s.CreateMon(context.Background(), "n", model.CephMonCreateRequest{})

		return e
	}},
	{"DeleteMon", func(s *CephService) error { return s.DeleteMon(context.Background(), "n", "m") }},
	{"CreateMgr", func(s *CephService) error {
		_, e := s.CreateMgr(context.Background(), "n", model.CephMgrCreateRequest{})

		return e
	}},
	{"SetFlags", func(s *CephService) error {
		return s.SetFlags(context.Background(), "n", model.CephFlagsSetRequest{})
	}},
	{"SetConfig", func(s *CephService) error {
		return s.SetConfig(context.Background(), "n", model.CephConfigSetRequest{})
	}},
	{"InitCeph", func(s *CephService) error {
		_, e := s.InitCeph(context.Background(), "n", model.CephInitRequest{})

		return e
	}},
	{"DestroyCeph", func(s *CephService) error {
		return s.DestroyCeph(context.Background(), "n", model.CephDestroyRequest{})
	}},
}

// newFailingCephGateway returns a gateway whose every method returns sentinel.
func newFailingCephGateway(sentinel error) *mockCephGateway {
	g := failingCephReadGateway(sentinel)
	failingCephWriteGateway(g, sentinel)

	return g
}

func failingCephReadGateway(sentinel error) *mockCephGateway {
	g := &mockCephGateway{}
	g.getClusterStatus = func(_ context.Context) (*model.CephClusterStatus, error) { return nil, sentinel }
	g.getClusterConfig = func(_ context.Context) ([]model.CephConfigEntry, error) { return nil, sentinel }
	g.getClusterLog = func(_ context.Context, _ int) ([]model.CephLogEntry, error) { return nil, sentinel }
	g.getClusterMeta = func(_ context.Context) (*model.CephClusterMetadata, error) { return nil, sentinel }
	g.getNodeStatus = func(_ context.Context, _ string) (*model.CephNodeStatus, error) { return nil, sentinel }
	g.getNodeConfig = func(_ context.Context, _ string) ([]model.CephConfigEntry, error) { return nil, sentinel }
	g.getNodeConfigDB = func(_ context.Context, _ string) ([]model.CephConfigEntry, error) { return nil, sentinel }
	g.getNodeFlags = func(_ context.Context, _ string) ([]model.CephFlag, error) { return nil, sentinel }
	g.getNodeMeta = func(_ context.Context, _ string) (*model.CephNodeMetadata, error) { return nil, sentinel }
	g.getNodeCrush = func(_ context.Context, _ string) (*model.CephCrushMap, error) { return nil, sentinel }
	g.getNodeRules = func(_ context.Context, _ string) ([]model.CephCrushRule, error) { return nil, sentinel }
	g.listPools = func(_ context.Context, _ string) ([]model.CephPool, error) { return nil, sentinel }
	g.listOSDs = func(_ context.Context, _ string) ([]model.CephOSD, error) { return nil, sentinel }
	g.listMons = func(_ context.Context, _ string) ([]model.CephMon, error) { return nil, sentinel }
	g.listMgrs = func(_ context.Context, _ string) ([]model.CephMgr, error) { return nil, sentinel }

	return g
}

func failingCephWriteGateway(g *mockCephGateway, sentinel error) {
	g.createCrushRule = func(_ context.Context, _ string, _ model.CephCrushRuleCreateRequest) (string, error) {
		return "", sentinel
	}
	g.deleteCrushRule = func(_ context.Context, _, _ string) error { return sentinel }
	g.createPool = func(_ context.Context, _ string, _ model.CephPoolCreateRequest) (string, error) {
		return "", sentinel
	}
	g.updatePool = func(_ context.Context, _, _ string, _ model.CephPoolUpdateRequest) error { return sentinel }
	g.deletePool = func(_ context.Context, _, _ string) error { return sentinel }
	g.createOSD = func(_ context.Context, _ string, _ model.CephOSDCreateRequest) (string, error) {
		return "", sentinel
	}
	g.deleteOSD = func(_ context.Context, _ string, _ int) error { return sentinel }
	g.createMon = func(_ context.Context, _ string, _ model.CephMonCreateRequest) (string, error) {
		return "", sentinel
	}
	g.deleteMon = func(_ context.Context, _, _ string) error { return sentinel }
	g.createMgr = func(_ context.Context, _ string, _ model.CephMgrCreateRequest) (string, error) {
		return "", sentinel
	}
	g.setFlags = func(_ context.Context, _ string, _ model.CephFlagsSetRequest) error { return sentinel }
	g.setConfig = func(_ context.Context, _ string, _ model.CephConfigSetRequest) error { return sentinel }
	g.initCeph = func(_ context.Context, _ string, _ model.CephInitRequest) (string, error) { return "", sentinel }
	g.destroyCeph = func(_ context.Context, _ string, _ model.CephDestroyRequest) error { return sentinel }
}

func TestCephService_ErrorPropagation(t *testing.T) {
	sentinel := errors.New("ceph boom")

	svc := &CephService{gw: newFailingCephGateway(sentinel)}
	for _, testCase := range cephErrorPropagationCases {
		t.Run(testCase.name, func(t *testing.T) {
			err := testCase.call(svc)
			assert.ErrorIs(t, err, sentinel)
		})
	}
}
