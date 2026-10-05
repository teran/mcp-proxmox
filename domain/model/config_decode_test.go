package model

import (
	"encoding/json"
	"testing"
)

// TestVMConfigDecodeStringNumericFields verifies that VMConfig decodes numeric
// config fields that Proxmox VE emits as JSON strings (the conformance bug:
// "memory":"8192"), while still accepting plain JSON numbers. See SPEC.md §9.
func TestVMConfigDecodeStringNumericFields(t *testing.T) {
	const stringJSON = `{"vmid":100,"name":"web","memory":"8192","cores":"2","sockets":"1","balloon":"512","template":"0","onboot":"1","ostype":"l26","boot":"order=scsi0"}`
	const numberJSON = `{"vmid":100,"name":"web","memory":8192,"cores":2,"sockets":1,"balloon":512,"template":0,"onboot":1,"ostype":"l26","boot":"order=scsi0"}`

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
