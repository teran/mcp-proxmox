package model

import (
	"encoding/json"
	"testing"
)

// TestVMConfigDecodeStringNumericFields verifies that VMConfig decodes numeric
// config fields that Proxmox VE emits as JSON strings (the conformance bug:
// "memory":"8192"), while still accepting plain JSON numbers. See SPEC.md §9.
func TestVMConfigDecodeStringNumericFields(t *testing.T) {
	const stringJSON = `{"vmid":100,"name":"web","memory":"8192","cores":"2","sockets":"1","balloon":"512",` +
		`"template":"0","onboot":"1","ostype":"l26","boot":"order=scsi0"}`
	const numberJSON = `{"vmid":100,"name":"web","memory":8192,"cores":2,"sockets":1,"balloon":512,` +
		`"template":0,"onboot":1,"ostype":"l26","boot":"order=scsi0"}`

	var stringDecoded VMConfig
	if err := json.Unmarshal([]byte(stringJSON), &stringDecoded); err != nil {
		t.Fatalf("unmarshal string-typed config: %v", err)
	}

	var numberDecoded VMConfig
	if err := json.Unmarshal([]byte(numberJSON), &numberDecoded); err != nil {
		t.Fatalf("unmarshal number-typed config: %v", err)
	}

	for name, cfg := range map[string]VMConfig{
		"string-typed": stringDecoded,
		"number-typed": numberDecoded,
	} {
		t.Run(name, func(t *testing.T) {
			if cfg.VMID != 100 {
				t.Errorf("VMID = %d, want 100", cfg.VMID)
			}
			if cfg.Memory != FlexInt(8192) {
				t.Errorf("Memory = %d, want 8192", cfg.Memory)
			}
			if cfg.Cores != FlexInt(2) {
				t.Errorf("Cores = %d, want 2", cfg.Cores)
			}
			if cfg.Sockets != FlexInt(1) {
				t.Errorf("Sockets = %d, want 1", cfg.Sockets)
			}
			if cfg.Balloon != FlexInt(512) {
				t.Errorf("Balloon = %d, want 512", cfg.Balloon)
			}
			if cfg.Template != FlexInt(0) {
				t.Errorf("Template = %d, want 0", cfg.Template)
			}
			if cfg.OnBoot != FlexInt(1) {
				t.Errorf("OnBoot = %d, want 1", cfg.OnBoot)
			}
		})
	}
}

// TestVMConfigDecodeFlatForm verifies VMConfig decodes the flat (non-nested)
// form that the real PVE GET /nodes/{node}/qemu/{vmid}/config emits: device
// keys (scsi0/ide2/net0) sit at the root alongside description/onboot and are
// aggregated into the Disks/Storage/Networks maps.
func TestVMConfigDecodeFlatForm(t *testing.T) {
	const flatJSON = `{"scsi0":"local:32","ide2":"local:iso.iso","net0":"virtio=AA:BB",` +
		`"description":"my vm","onboot":0}`

	var cfg VMConfig
	if err := json.Unmarshal([]byte(flatJSON), &cfg); err != nil {
		t.Fatalf("unmarshal flat config: %v", err)
	}

	if cfg.Disks["scsi0"] != "local:32" {
		t.Errorf("Disks[scsi0] = %q, want local:32", cfg.Disks["scsi0"])
	}
	if cfg.Storage["ide2"] != "local:iso.iso" {
		t.Errorf("Storage[ide2] = %q, want local:iso.iso", cfg.Storage["ide2"])
	}
	if cfg.Networks["net0"] != "virtio=AA:BB" {
		t.Errorf("Networks[net0] = %q, want virtio=AA:BB", cfg.Networks["net0"])
	}
	if cfg.Description != "my vm" {
		t.Errorf("Description = %q, want my vm", cfg.Description)
	}
	if cfg.OnBoot != FlexInt(0) {
		t.Errorf("OnBoot = %d, want 0", cfg.OnBoot)
	}
}

// TestLXCConfigDecodeStringNumericFields verifies that LXCConfig decodes
// numeric config fields that Proxmox VE emits as JSON strings.
func TestLXCConfigDecodeStringNumericFields(t *testing.T) {
	const stringJSON = `{"vmid":200,"cores":"1","memory":"256","swap":"64"}`

	var cfg LXCConfig
	if err := json.Unmarshal([]byte(stringJSON), &cfg); err != nil {
		t.Fatalf("unmarshal string-typed lxc config: %v", err)
	}

	if cfg.Cores != FlexInt(1) {
		t.Errorf("Cores = %d, want 1", cfg.Cores)
	}
	if cfg.Memory != FlexInt(256) {
		t.Errorf("Memory = %d, want 256", cfg.Memory)
	}
	if cfg.Swap != FlexInt(64) {
		t.Errorf("Swap = %d, want 64", cfg.Swap)
	}
}
