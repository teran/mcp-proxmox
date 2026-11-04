package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBackupJobUnmarshalIntegerBooleans is a regression test for the live bug
// where PVE /cluster/backup returns boolean fields as integers 0/1
// (e.g. "enabled":1), which a plain Go bool could not decode. It decodes a
// REAL response element (PVE 9.2) with nested fleecing/prune-backups objects
// and the notification-mode field, verifying integer booleans map to the
// correct FlexBool state and nested structs decode into their new types.
func TestBackupJobUnmarshalIntegerBooleans(t *testing.T) {
	const body = `{"notification-mode":"notification-system","schedule":"*/1:00","enabled":1,` +
		`"id":"backup-90bd778c-53b5","next-run":1793757600,"storage":"pbs01","mode":"snapshot",` +
		`"node":"ms01","fleecing":{"enabled":0},"prune-backups":{"keep-all":1},` +
		`"notes-template":"{{guestname}}","type":"vzdump","all":1}`

	var job BackupJob
	require.NoError(t, json.Unmarshal([]byte(body), &job))
	assert.Equal(t, "backup-90bd778c-53b5", job.ID)
	assert.Equal(t, FlexBool(true), job.Enabled)
	assert.Equal(t, FlexBool(true), job.All)
	assert.Equal(t, FlexBool(false), job.Fleecing.Enabled)
	assert.Equal(t, FlexInt(1), job.PruneBackups.KeepAll)
	assert.Equal(t, "notification-system", job.MailNotification)
	assert.Equal(t, int64(1793757600), job.NextRun)
}

// TestBackupJobUnmarshalFleecingEnabled verifies the nested fleecing object
// with "enabled":1 maps to Fleecing.Enabled == FlexBool(true).
func TestBackupJobUnmarshalFleecingEnabled(t *testing.T) {
	const body = `{"id":"backup-100","enabled":1,"schedule":"daily","mode":"snapshot",` +
		`"storage":"local","all":1,"fleecing":{"enabled":1}}`

	var job BackupJob
	require.NoError(t, json.Unmarshal([]byte(body), &job))
	assert.Equal(t, FlexBool(true), job.Fleecing.Enabled)
	assert.Equal(t, FlexBool(true), job.Enabled)
}

// TestBackupJobUnmarshalAllFalse verifies the "all":0 case maps to
// FlexBool(false).
func TestBackupJobUnmarshalAllFalse(t *testing.T) {
	const body = `{"id":"backup-101","enabled":1,"schedule":"00 2 * * 1","mode":"snapshot",` +
		`"storage":"local","all":0,"vmid":"100"}`

	var job BackupJob
	require.NoError(t, json.Unmarshal([]byte(body), &job))
	assert.Equal(t, FlexBool(false), job.All)
	assert.Equal(t, FlexBool(true), job.Enabled)
}

// TestCreateBackupJobRequestMarshalMinimal pins the exact JSON for a minimal
// create request: only the required Storage and Schedule fields appear, and
// every optional field (ID, All, VMID, Mode, Compress, Enabled, NotesTemplate,
// Comment, Node) is omitted.
func TestCreateBackupJobRequestMarshalMinimal(t *testing.T) {
	b, err := json.Marshal(CreateBackupJobRequest{Storage: "local", Schedule: "00 2 * * 1"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"storage":"local","schedule":"00 2 * * 1"}`, string(b))
}

// TestCreateBackupJobRequestMarshalFull verifies every field marshals with the
// correct JSON key (including the dashed notes-template) when populated.
func TestCreateBackupJobRequestMarshalFull(t *testing.T) {
	req := CreateBackupJobRequest{
		ID:            "backup-100",
		All:           true,
		VMID:          "100,101",
		Storage:       "local",
		Schedule:      "00 2 * * 1",
		Mode:          "snapshot",
		Compress:      "zstd",
		Enabled:       new(true),
		NotesTemplate: "{{guestname}}",
		Comment:       "prod cluster",
		Node:          "pve1",
	}
	b, err := json.Marshal(req)
	require.NoError(t, err)

	want := `{
		"id":"backup-100",
		"all":true,
		"vmid":"100,101",
		"storage":"local",
		"schedule":"00 2 * * 1",
		"mode":"snapshot",
		"compress":"zstd",
		"enabled":true,
		"notes-template":"{{guestname}}",
		"comment":"prod cluster",
		"node":"pve1"
	}`
	assert.JSONEq(t, want, string(b))
}

// TestCreateBackupJobRequestMarshalEnabledFalse verifies that an explicit
// Enabled=false is NOT omitted (a non-nil *bool is serialized even when false),
// so the adapter can send "enabled":"0" to disable a job.
func TestCreateBackupJobRequestMarshalEnabledFalse(t *testing.T) {
	b, err := json.Marshal(CreateBackupJobRequest{Storage: "s", Schedule: "*", Enabled: new(false)})
	require.NoError(t, err)
	assert.JSONEq(t, `{"storage":"s","schedule":"*","enabled":false}`, string(b))
}

// TestCreateBackupJobRequestMarshalEnabledNil verifies that a nil Enabled is
// omitted entirely (the job keeps its current/default state).
func TestCreateBackupJobRequestMarshalEnabledNil(t *testing.T) {
	b, err := json.Marshal(CreateBackupJobRequest{Storage: "s", Schedule: "*"})
	require.NoError(t, err)
	assert.JSONEq(t, `{"storage":"s","schedule":"*"}`, string(b))
}

// TestCreateBackupJobRequestRoundTrip verifies a fully-populated request
// marshals and unmarshals back to an identical value without loss.
func TestCreateBackupJobRequestRoundTrip(t *testing.T) {
	in := CreateBackupJobRequest{
		ID:            "backup-100",
		All:           true,
		VMID:          "100,101",
		Storage:       "local",
		Schedule:      "00 2 * * 1",
		Mode:          "snapshot",
		Compress:      "zstd",
		Enabled:       new(false),
		NotesTemplate: "{{guestname}}",
		Comment:       "prod",
		Node:          "pve1",
	}
	b, err := json.Marshal(in)
	require.NoError(t, err)

	var out CreateBackupJobRequest
	require.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, in, out)
}
