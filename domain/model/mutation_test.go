package model

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestMutationRequestRoundTrip verifies that every mutation request type
// marshals and unmarshals to the JSON the real Proxmox VE / PBS REST APIs
// expect as form-parameter payloads, and back again without loss. This pins the
// struct tags in domain/model/mutation.go (SPEC.md §6.x).
func TestMutationRequestRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		in   any
	}{
		{
			name: "create-vm",
			in: CreateVMRequest{
				VMID: 100, Name: "web", Cores: 2, Memory: 1024, Sockets: 1,
				Ostype: "l26", Net0: "virtio=AA:BB", Scsi0: "local:32", Ide2: "local:iso.iso",
				Boot: "order=scsi0", Balloon: 512, Description: "prod web", Tags: "web;prod",
				Agent: 1, OnBoot: 1, CPU: "host", Machine: "q35", VGA: "std",
			},
		},
		{
			name: "resize-vm",
			in:   ResizeVMRequest{Disk: "scsi0", SizeGB: 20},
		},
		{
			name: "migrate-vm",
			in:   MigrateVMRequest{Target: "pve2", Online: true, WithLocalDisks: true},
		},
		{
			name: "ha-resource",
			in:   HAResourceRequest{SID: "vm:100", Type: "vm", Nodes: []string{"pve1", "pve2"}, Comment: "primary"},
		},
		{
			name: "vm-backup",
			in: VMBackupRequest{
				VMID: 100, Storage: "backup", Mode: "snapshot",
				NotesTemplate: "{{guestname}}", Compress: "zstd",
			},
		},
		{
			name: "pbs-sync",
			in: PBSSyncRequest{
				Remote: "remote1", RemoteStore: "backup", Owner: "root@pam",
				MaxFiles: 3, RemoveVanished: true, RateIn: 1000, RateOut: 2000,
				SkipLost: true, NotifyUser: "admin@example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			typ := reflect.TypeOf(tt.in)
			rv := reflect.New(typ)
			if err := json.Unmarshal(b, rv.Interface()); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if !reflect.DeepEqual(rv.Elem().Interface(), tt.in) {
				t.Fatalf("round-trip mismatch\n got: %#v\nwant: %#v", rv.Elem().Interface(), tt.in)
			}
		})
	}
}

// TestMutationRequestOptionalOmitted verifies that zero-valued optional fields
// are omitted so the adapter only sends what was requested. Required fields
// (disk/size for resize, target for migrate, sid/type for HA, vmid/storage/mode
// for backup) are always present.
func TestMutationRequestOptionalOmitted(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"create-vm-minimal", CreateVMRequest{Name: "web"}, `{"name":"web"}`},
		{"resize-vm", ResizeVMRequest{Disk: "scsi0", SizeGB: 20}, `{"disk":"scsi0","size_gb":20}`},
		{"migrate-vm-minimal", MigrateVMRequest{Target: "pve2"}, `{"target":"pve2"}`},
		{"ha-resource-minimal", HAResourceRequest{SID: "vm:100", Type: "vm"}, `{"sid":"vm:100","type":"vm"}`},
		{"vm-backup-minimal", VMBackupRequest{VMID: 100, Storage: "local", Mode: "snapshot"},
			`{"vmid":100,"storage":"local","mode":"snapshot"}`},
		{"pbs-sync-empty", PBSSyncRequest{}, `{}`},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(b) != tt.want {
				t.Fatalf("mismatch\n got: %s\nwant: %s", b, tt.want)
			}
		})
	}
}
