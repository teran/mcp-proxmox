package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/teran/mcp-proxmox/application"
	"github.com/teran/mcp-proxmox/domain/model"
)

// These regression tests pin the MCP tool-layer behavior for two of the live
// integration defects. The actual decode fixes live in the gateway/model layers
// (see adapter/pve/gateway_regression_test.go and
// domain/model/regression_test.go); these tests guard the tool INPUT/OUTPUT
// contract that the defective gateway path broke.

// TestMutationTool_HAAdd_AcceptsNodesList guards Defect 4 (pve_ha_add): the tool
// input must accept `nodes` as a list of preferred nodes (the MCP schema
// declares it as an array of strings), and calling with the live
// ["nuc01","ms01"] payload must not error. The gateway then serializes it as a
// comma-joined string (asserted in the pve gateway regression test).
func TestMutationTool_HAAdd_AcceptsNodesList(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServer(t, app, true)

	res := callTool(t, sess, "pve_ha_add", map[string]any{
		"sid": "vm:100", "type": "vm", "nodes": []string{"nuc01", "ms01"}, "comment": "primary",
	})
	assert.False(t, res.IsError, "pve_ha_add must accept a nodes list")
}

// TestMutationTool_HAAdd_AcceptsCommaSeparatedNodes is the RED reproducer for
// the Defect 4 tool-decoding gap: PVE documents `nodes` as a comma-separated
// STRING ("nuc01,ms01"), and an LLM client following that convention passes the
// value as a single string. The go-sdk input schema currently declares `nodes`
// as an array only, so the call is rejected with
// `validating /properties/nodes: type: nuc01,ms01 has type "string", want one
// of "null, array"`. The tool must accept the comma-joined string form too
// (then the gateway already serializes it as-is).
func TestMutationTool_HAAdd_AcceptsCommaSeparatedNodes(t *testing.T) {
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(&stubPVEGateway{}))
	_, sess := newTestServer(t, app, true)

	res := callTool(t, sess, "pve_ha_add", map[string]any{
		"sid": "vm:100", "type": "vm", "nodes": "nuc01,ms01",
	})
	assert.False(t, res.IsError, "pve_ha_add must accept nodes as a comma-joined string")
}

// TestMutationTool_BackupJobCreate_EmptyIDSuccess guards Defect 5
// (pve_backup_job_create): when the created backup job returns no ID (PVE
// returns empty/{} data on success), the tool must still report success rather
// than IsError. The RED reproducer of the underlying gateway decode defect lives
// in adapter/pve (TestGateway_CreateBackupJob_EmptyDataSuccess).
func TestMutationTool_BackupJobCreate_EmptyIDSuccess(t *testing.T) {
	gateway := &stubPVEGateway{
		createBackupJob: func(_ context.Context, _ model.CreateBackupJobRequest) (string, error) {
			return "", nil // PVE created the job but returned empty data
		},
	}
	app := application.New(nopLogger{}, "v1.0.0", application.WithPVE(gateway))
	_, sess := newTestServer(t, app, true)

	res := callTool(t, sess, "pve_backup_job_create", map[string]any{
		"storage": "local", "schedule": "00 2 * * 1",
	})
	assert.False(t, res.IsError, "pve_backup_job_create must not flag IsError when the job is created with empty data")
}
