package model

import (
	"bytes"
	"encoding/json"
	"testing"
)

// TestDatastore_OnlyOfficialFields pins the Datastore wire schema to exactly the
// five fields of GET /admin/datastore. Fields that used to leak in (path,
// keep-*, notify-user, gc-schedule, verify-new) must NOT be serialized anymore.
func TestDatastore_OnlyOfficialFields(t *testing.T) {
	in := Datastore{
		Store: "backup", BackendType: "filesystem", MountStatus: "mounted",
		Comment: "primary", Maintenance: "none",
	}

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `{"store":"backup","backend-type":"filesystem","mount-status":"mounted","comment":"primary","maintenance":"none"}`
	if string(b) != want {
		t.Fatalf("mismatch\n got: %s\nwant: %s", b, want)
	}

	// None of the removed fields may appear anywhere in the output.
	removed := []string{`"path"`, `"keep-`, `"notify-user"`, `"gc-schedule"`, `"verify-new"`}
	for _, frag := range removed {
		if bytes.Contains(b, []byte(frag)) {
			t.Fatalf("Datastore must not serialize removed field %q; got: %s", frag, b)
		}
	}
}

// TestDatastoreConfig_omitempty verifies that zero-valued optional fields are
// omitted while a fully-populated config serializes every field.
func TestDatastoreConfig_omitempty(t *testing.T) {
	t.Run("only path required", func(t *testing.T) {
		b, err := json.Marshal(DatastoreConfig{Path: "/backup"})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if string(b) != `{"path":"/backup"}` {
			t.Fatalf("mismatch\n got: %s\nwant: {\"path\":\"/backup\"}", b)
		}
	})

	t.Run("all fields present", func(t *testing.T) {
		in := DatastoreConfig{
			Path: "/backup", Comment: "primary", Backend: "filesystem", BackingDevice: "/dev/sdb",
			KeepHourly: 1, KeepDaily: 7, KeepWeekly: 4, KeepMonthly: 6, KeepYearly: 2, KeepLast: 1,
			GCSchedule: "sun", VerifyNew: true, PruneSchedule: "daily", NotificationMode: "auto",
			NotifyUser: "root@pam", Notify: "always", MaintenanceMode: "off",
			CounterResetSchedule: "daily", GCOnUnmount: true, Tuning: "example",
		}
		b, err := json.Marshal(in)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		want := `{"path":"/backup","comment":"primary","backend":"filesystem","backing-device":"/dev/sdb","keep-hourly":1,"keep-daily":7,"keep-weekly":4,"keep-monthly":6,"keep-yearly":2,"keep-last":1,"gc-schedule":"sun","verify-new":true,"prune-schedule":"daily","notification-mode":"auto","notify-user":"root@pam","notify":"always","maintenance-mode":"off","counter-reset-schedule":"daily","gc-on-unmount":true,"tuning":"example"}`
		if string(b) != want {
			t.Fatalf("mismatch\n got: %s\nwant: %s", b, want)
		}
	})
}

// TestDatastoreConfig_Unmarshal verifies a real /config/datastore/{name} payload
// decodes into the struct (PBS returns kebab-case JSON keys).
func TestDatastoreConfig_Unmarshal(t *testing.T) {
	raw := `{"path":"/backup","keep-daily":7,"verify-new":true,"gc-schedule":"sun","notify-user":"root@pam"}`
	var c DatastoreConfig
	if err := json.Unmarshal([]byte(raw), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c.Path != "/backup" || c.KeepDaily != 7 || !c.VerifyNew || c.GCSchedule != "sun" || c.NotifyUser != "root@pam" {
		t.Fatalf("unexpected decoded value: %#v", c)
	}
}
