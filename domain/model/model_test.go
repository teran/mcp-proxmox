package model

import (
	"encoding/json"
	"reflect"
	"testing"
)

// TestJSONRoundTrip verifies that every entity marshals and unmarshals to the
// exact JSON the real Proxmox VE / PBS REST APIs emit (/api2/json), and back
// again without loss. This pins the struct tags in domain/model.
func TestJSONRoundTrip(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want string
	}{
		{
			name: "node",
			in: Node{
				Node: "pve1", Status: "online", CPU: 0.12, MaxCPU: 8,
				Mem: 1024, MaxMem: 16384, Disk: 100, MaxDisk: 512,
				Uptime: 3600, SSLFingerprint: "AA:BB", Level: "b", ID: "node/pve1", Type: "node",
			},
			want: `{"node":"pve1","status":"online","cpu":0.12,"maxcpu":8,"mem":1024,"maxmem":16384,"disk":100,"maxdisk":512,"uptime":3600,"sslfingerprint":"AA:BB","level":"b","id":"node/pve1","type":"node"}`,
		},
		{
			name: "node-status",
			in: NodeStatus{
				Node: "pve1", Status: "online", CPU: 0.5, MaxCPU: 8, Mem: 512, MaxMem: 16384,
				Disk: 200, MaxDisk: 512, Uptime: 7200, LoadAvg: []float64{0.1, 0.2, 0.3},
				KVM: true, PVE: true, Swap: 64, KSmem: 16, Wait: 0.01,
			},
			want: `{"node":"pve1","status":"online","cpu":0.5,"maxcpu":8,"mem":512,"maxmem":16384,"disk":200,"maxdisk":512,"uptime":7200,"loadavg":[0.1,0.2,0.3],"kvm":true,"pve":true,"swap":64,"ksm":16,"wait":0.01}`,
		},
		{
			name: "vm",
			in: VM{
				VMID: 100, Name: "web", Status: "running", Node: "pve1", Type: "qemu",
				Lock: "backup", Uptime: 100, Mem: 512, MaxMem: 1024, Disk: 10, MaxDisk: 32, CPU: 2, Tags: "prod;web",
			},
			want: `{"vmid":100,"name":"web","status":"running","node":"pve1","type":"qemu","lock":"backup","uptime":100,"mem":512,"maxmem":1024,"disk":10,"maxdisk":32,"cpus":2,"tags":"prod;web"}`,
		},
		{
			name: "vm-config",
			in: VMConfig{
				VMID: 100, Name: "web", Cores: 2, Sockets: 1, Memory: 1024, Balloon: 512,
				OSType: "l26", Boot: "order=scsi0", Template: 0, Agent: 1,
				Networks: map[string]string{"net0": "virtio=AA:BB:CC"}, Disks: map[string]string{"scsi0": "local:32"},
				Storage: map[string]string{"ide2": "local:iso.iso"}, Tags: "web",
			},
			want: `{"vmid":100,"name":"web","cores":2,"sockets":1,"memory":1024,"balloon":512,"ostype":"l26","boot":"order=scsi0","agent":1,"net":{"net0":"virtio=AA:BB:CC"},"scsi":{"scsi0":"local:32"},"ide":{"ide2":"local:iso.iso"},"tags":"web"}`,
		},
		{
			name: "vm-status",
			in: VMStatus{
				VMID: 100, Status: "running", Name: "web", Node: "pve1", Uptime: 500,
				CPU: 0.25, Mem: 256, MaxMem: 1024, Disk: 5, MaxDisk: 32, QMPStatus: "running", Lock: "", Agent: "running",
			},
			want: `{"vmid":100,"status":"running","name":"web","node":"pve1","uptime":500,"cpu":0.25,"mem":256,"maxmem":1024,"disk":5,"maxdisk":32,"qmpstatus":"running","agent":"running"}`,
		},
		{
			name: "lxc",
			in: LXC{
				VMID: 200, Name: "ct1", Status: "running", Node: "pve1", Type: "lxc",
				Uptime: 60, Mem: 128, MaxMem: 256, Disk: 4, MaxDisk: 8, CPUs: 1, Tags: "ct",
			},
			want: `{"vmid":200,"name":"ct1","status":"running","node":"pve1","type":"lxc","uptime":60,"mem":128,"maxmem":256,"disk":4,"maxdisk":8,"cpus":1,"tags":"ct"}`,
		},
		{
			name: "lxc-config",
			in: LXCConfig{
				VMID: 200, Hostname: "ct1", OSType: "ubuntu", Cores: 1, Memory: 256, Swap: 64,
				RootFS: "local:8", Networks: map[string]string{"net0": "name=eth0,bridge=vmbr0"}, Tags: "ct",
			},
			want: `{"vmid":200,"hostname":"ct1","ostype":"ubuntu","cores":1,"memory":256,"swap":64,"rootfs":"local:8","net":{"net0":"name=eth0,bridge=vmbr0"},"tags":"ct"}`,
		},
		{
			name: "lxc-status",
			in: LXCStatus{
				VMID: 200, Status: "running", Name: "ct1", Type: "lxc", Uptime: 120,
				CPU: 0.1, Mem: 64, MaxMem: 256, Disk: 2, MaxDisk: 8, Swap: 0, Lock: "",
			},
			want: `{"vmid":200,"status":"running","name":"ct1","type":"lxc","uptime":120,"cpu":0.1,"mem":64,"maxmem":256,"disk":2,"maxdisk":8}`,
		},
		{
			name: "cluster-status",
			in: ClusterStatus{
				ID: "cluster/1", Name: "cluster", Type: "cluster", Nodes: 3, Quorum: 1, Online: 3,
			},
			want: `{"id":"cluster/1","name":"cluster","type":"cluster","nodes":3,"quorate":1,"online":3}`,
		},
		{
			name: "cluster-resource",
			in: ClusterResource{
				ID: "qemu/100", Type: "qemu", VMID: 100, Node: "pve1", Name: "web",
				Status: "running", Storage: "local", Disk: 10, MaxDisk: 32, Mem: 512, MaxMem: 1024,
				CPU: 0.1, MaxCPU: 2, Uptime: 300,
			},
			want: `{"id":"qemu/100","type":"qemu","vmid":100,"node":"pve1","name":"web","status":"running","storage":"local","disk":10,"maxdisk":32,"mem":512,"maxmem":1024,"cpu":0.1,"maxcpu":2,"uptime":300}`,
		},
		{
			name: "storage",
			in: Storage{
				Storage: "local", Type: "dir", Status: "available", Content: "images,rootdir",
				Nodes: "pve1", Shared: false, Active: true, Enabled: true, Path: "/var/lib/vz",
			},
			want: `{"storage":"local","type":"dir","status":"available","content":"images,rootdir","nodes":"pve1","active":true,"enabled":true,"path":"/var/lib/vz"}`,
		},
		{
			name: "storage-status",
			in: StorageStatus{
				Storage: "local", Type: "dir", Total: 1000, Used: 400, Avail: 600, UsedFrac: 0.4, Enabled: true, Active: true,
			},
			want: `{"storage":"local","type":"dir","total":1000,"used":400,"avail":600,"used_fraction":0.4,"enabled":true,"active":true}`,
		},
		{
			name: "network-interface",
			in: NetworkInterface{
				Iface: "eth0", Type: "eth", Bridge: "vmbr0", Address: "10.0.0.1", Netmask: "255.255.255.0",
				Gateway: "10.0.0.254", Method: "static", VLANID: 10, Active: true, Status: "up", Comments: "uplink",
			},
			want: `{"iface":"eth0","type":"eth","bridge":"vmbr0","address":"10.0.0.1","netmask":"255.255.255.0","gateway":"10.0.0.254","method":"static","vlan-id":10,"active":true,"status":"up","comments":"uplink"}`,
		},
		{
			name: "task",
			in: Task{
				UPID: "UPID:pve1:00000000:...", Type: "qmstart", Node: "pve1", User: "root@pam",
				StartTime: 1000, EndTime: 1005, Status: "stopped", ExitStatus: "OK", PID: 1234,
			},
			want: `{"upid":"UPID:pve1:00000000:...","type":"qmstart","node":"pve1","user":"root@pam","starttime":1000,"endtime":1005,"status":"stopped","exitstatus":"OK","pid":1234}`,
		},
		{
			name: "task-status",
			in: TaskStatus{
				UPID: "UPID:pve1:...", Type: "qmstart", Node: "pve1", User: "root@pam",
				StartTime: 1000, EndTime: 1005, Status: "stopped", ExitStatus: "OK", PID: 1234, Running: false,
			},
			want: `{"upid":"UPID:pve1:...","type":"qmstart","node":"pve1","user":"root@pam","starttime":1000,"endtime":1005,"status":"stopped","exitstatus":"OK","pid":1234}`,
		},
		{
			name: "task-log-entry",
			in:   TaskLogEntry{LineNumber: 1, Text: "TASK OK"},
			want: `{"n":1,"t":"TASK OK"}`,
		},
		{
			name: "pve-version",
			in:   PVEVersion{Version: "8.2.2", Release: "8.2", RepoID: "abcd", Keyboard: "en-us"},
			want: `{"version":"8.2.2","release":"8.2","repoid":"abcd","keyboard":"en-us"}`,
		},
		{
			name: "datastore",
			in: Datastore{
				Name: "backup", Path: "/backup", Comment: "primary", KeepDaily: 7, KeepWeekly: 4,
				KeepMonthly: 6, KeepYearly: 2, NotifyUser: "root@pam", GC: "sun", Verify: "mon",
			},
			want: `{"name":"backup","path":"/backup","comment":"primary","keep-daily":7,"keep-weekly":4,"keep-monthly":6,"keep-yearly":2,"notify-user":"root@pam","gc-schedule":"sun","verify-new":"mon"}`,
		},
		{
			name: "datastore-status",
			in: DatastoreStatus{
				Store: "backup", Total: 2000, Used: 500, Avail: 1500, UsedFrac: 0.25, Snapshots: 10, Chunks: 5, Maintenance: "none",
			},
			want: `{"store":"backup","total":2000,"used":500,"avail":1500,"used_fraction":0.25,"snapshots":10,"chunks":5,"maintenance":"none"}`,
		},
		{
			name: "backup",
			in: Backup{
				BackupID: "vm/100/2024-01-01T00:00:00Z", BackupTime: 1704067200, BackupType: "vm",
				Store: "backup", Comment: "nightly", Size: 1024, Files: 3, Encrypted: true, Protected: false,
			},
			want: `{"backup-id":"vm/100/2024-01-01T00:00:00Z","backup-time":1704067200,"backup-type":"vm","store":"backup","comment":"nightly","size":1024,"files":3,"encrypted":true}`,
		},
		{
			name: "backup-notes",
			in:   BackupNotes{Snapshot: "vm/100/...", Notes: "keep"},
			want: `{"snapshot":"vm/100/...","notes":"keep"}`,
		},
		{
			name: "verify-status",
			in:   VerifyStatus{UPID: "UPID:...", Store: "backup", Status: "ok", Errors: 0, Duration: 42},
			want: `{"upid":"UPID:...","store":"backup","status":"ok","duration":42}`,
		},
		{
			name: "prune-status",
			in:   PruneStatus{UPID: "UPID:...", Store: "backup", Status: "running", Duration: 7},
			want: `{"upid":"UPID:...","store":"backup","status":"running","duration":7}`,
		},
		{
			name: "pbs-version",
			in:   PBSVersion{Version: "3.2.3", Release: "3.2", RepoID: "efgh"},
			want: `{"version":"3.2.3","release":"3.2","repoid":"efgh"}`,
		},
		{
			name: "server-status",
			in:   ServerStatus{Version: "v1.0.0", Transport: "stdio", PVE: true, PBS: false},
			want: `{"version":"v1.0.0","transport":"stdio","pve":true,"pbs":false}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := json.Marshal(tt.in)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if string(b) != tt.want {
				t.Fatalf("marshal mismatch\n got: %s\nwant: %s", b, tt.want)
			}

			// Round-trip: unmarshal back into a fresh value of the same type.
			typ := reflect.TypeOf(tt.in)
			rv := reflect.New(typ) // *T for value type T
			if err := json.Unmarshal(b, rv.Interface()); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if !reflect.DeepEqual(rv.Elem().Interface(), tt.in) {
				t.Fatalf("round-trip mismatch\n got: %#v\nwant: %#v", rv.Elem().Interface(), tt.in)
			}
		})
	}
}

// TestOptionalFieldsOmitted verifies that zero-valued optional fields are
// omitted from the JSON so the wire format stays clean for optional values.
func TestOptionalFieldsOmitted(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want string
	}{
		{"node-minimal", Node{Node: "pve1", Status: "online"}, `{"node":"pve1","status":"online"}`},
		{"vm-minimal", VM{VMID: 100}, `{"vmid":100}`},
		{"lxc-minimal", LXC{VMID: 200}, `{"vmid":200}`},
		{"task-required", Task{UPID: "u", Type: "t", Node: "n", StartTime: 1}, `{"upid":"u","type":"t","node":"n","starttime":1}`},
		{"storage-minimal", Storage{Storage: "local", Type: "dir"}, `{"storage":"local","type":"dir"}`},
		{"cluster-minimal", ClusterStatus{ID: "id", Name: "n", Type: "t"}, `{"id":"id","name":"n","type":"t"}`},
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
