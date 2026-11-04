package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNodeStatusUnmarshalRealResponse is a regression test for the live bug
// where GET /nodes/{node}/status returns a completely different shape (PVE
// 9.2) than the old model.NodeStatus. It decodes a REAL response element and
// pins the new nested types: LoadAvg (string numbers), MemoryInfo, RootFSInfo,
// BootInfo, CPUInfo, CurrentKernel and KSMInfo.
func TestNodeStatusUnmarshalRealResponse(t *testing.T) {
	const body = `{"data":{"loadavg":["0.36","0.60","2.73"],"cpu":0,` +
		`"memory":{"used":18149171200,"free":42468319232,"available":46922694656,"total":65071865856},` +
		`"wait":0,"uptime":3000927,"pveversion":"pve-manager/9.2.10/43df2e01f27a1a19","idle":0,` +
		`"kversion":"Linux 7.0.14-11-pve","swap":{"used":0,"free":0,"total":0},` +
		`"rootfs":{"avail":29257637888,"free":29257637888,"used":13045903360,"total":42303541248},` +
		`"cpuinfo":{"mhz":"1114.960","cores":8,"cpus":16,"sockets":1,"vendor":"AuthenticAMD",` +
		`"model":"AMD Ryzen 7 255","family":"25","flags":"fpu"},` +
		`"boot-info":{"secureboot":1,"mode":"efi"},` +
		`"current-kernel":{"machine":"x86_64","release":"7.0.14-11-pve","sysname":"Linux","version":"#1 SMP"},` +
		`"ksm":{"shared":0}}}`

	var resp struct {
		Data NodeStatus `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &resp))

	status := resp.Data
	require.Equal(t, LoadAvg{0.36, 0.60, 2.73}, status.LoadAvg)
	assert.Equal(t, int64(65071865856), status.Memory.Total)
	assert.Equal(t, int64(29257637888), status.RootFS.Avail)
	assert.Equal(t, "efi", status.BootInfo.Mode)
	assert.Equal(t, FlexInt(1), status.BootInfo.SecureBoot)
	assert.Equal(t, 8, status.CPUInfo.Cores)
	assert.Equal(t, "7.0.14-11-pve", status.CurrentKernel.Release)
	assert.Equal(t, int64(0), status.KSM.Shared)
	assert.Equal(t, "pve-manager/9.2.10/43df2e01f27a1a19", status.PVEVersion)
}

// TestNodeStatusUnmarshalLoadAvgNumbers verifies that the loadavg field also
// unmarshals from a plain JSON array of numbers (not just strings).
func TestNodeStatusUnmarshalLoadAvgNumbers(t *testing.T) {
	var status NodeStatus
	require.NoError(t, json.Unmarshal([]byte(`{"loadavg":[0.1,0.2,0.3]}`), &status))
	require.Equal(t, LoadAvg{0.1, 0.2, 0.3}, status.LoadAvg)
}
