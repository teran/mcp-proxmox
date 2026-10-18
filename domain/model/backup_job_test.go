package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// boolPtr returns a pointer to b, so an explicit false can be serialized for
// the Enabled field (which must be present when set to false).
func boolPtr(b bool) *bool { return &b }

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
		Enabled:       boolPtr(true),
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
	b, err := json.Marshal(CreateBackupJobRequest{Storage: "s", Schedule: "*", Enabled: boolPtr(false)})
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
		Enabled:       boolPtr(false),
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
