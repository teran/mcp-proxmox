package model

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
)

// Node is a Proxmox VE cluster node as returned by GET /nodes.
type Node struct {
	Node           string  `json:"node"`
	Status         string  `json:"status"`
	CPU            float64 `json:"cpu,omitempty"`
	MaxCPU         int     `json:"maxcpu,omitempty"`
	Mem            int64   `json:"mem,omitempty"`
	MaxMem         int64   `json:"maxmem,omitempty"`
	Disk           int64   `json:"disk,omitempty"`
	MaxDisk        int64   `json:"maxdisk,omitempty"`
	Uptime         int64   `json:"uptime,omitempty"`
	SSLFingerprint string  `json:"sslfingerprint,omitempty"`
	Level          string  `json:"level,omitempty"`
	ID             string  `json:"id,omitempty"`
	Type           string  `json:"type,omitempty"`
}

// SwapInfo is the swap usage of a node from GET /nodes/{node}/status.
type SwapInfo struct {
	Total int64 `json:"total,omitempty"`
	Used  int64 `json:"used,omitempty"`
	Free  int64 `json:"free,omitempty"`
}

// LoadAvg is a node's 1/5/15-minute load averages. Proxmox returns them as an
// array of decimal strings (["0.36","0.60","2.73"]), so it unmarshals from
// either a JSON array of strings or a JSON array of numbers, and marshals back
// as a JSON array of numbers.
type LoadAvg []float64

func (l *LoadAvg) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || bytes.Equal(b, []byte("null")) {
		*l = nil

		return nil
	}
	var nums []float64
	if err := json.Unmarshal(b, &nums); err == nil {
		*l = LoadAvg(nums)

		return nil
	}
	var strs []string
	if err := json.Unmarshal(b, &strs); err != nil {
		return err
	}
	out := make(LoadAvg, 0, len(strs))
	for _, s := range strs {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return fmt.Errorf("parse loadavg %q: %w", s, err)
		}
		out = append(out, f)
	}
	*l = out

	return nil
}

func (l LoadAvg) MarshalJSON() ([]byte, error) { return json.Marshal([]float64(l)) }

// MemoryInfo is the memory usage of a node from GET /nodes/{node}/status.
type MemoryInfo struct {
	Used      int64 `json:"used,omitempty"`
	Free      int64 `json:"free,omitempty"`
	Available int64 `json:"available,omitempty"`
	Total     int64 `json:"total,omitempty"`
}

// RootFSInfo is the root filesystem usage of a node from GET /nodes/{node}/status.
type RootFSInfo struct {
	Avail int64 `json:"avail,omitempty"`
	Free  int64 `json:"free,omitempty"`
	Used  int64 `json:"used,omitempty"`
	Total int64 `json:"total,omitempty"`
}

// KSMInfo is the KSM (Kernel Samepage Merging) usage of a node.
type KSMInfo struct {
	Shared int64 `json:"shared,omitempty"`
}

// CPUInfo describes the node's CPU as reported by GET /nodes/{node}/status.
type CPUInfo struct {
	MHz     string `json:"mhz,omitempty"`
	Cores   int    `json:"cores,omitempty"`
	CPUs    int    `json:"cpus,omitempty"`
	Sockets int    `json:"sockets,omitempty"`
	Vendor  string `json:"vendor,omitempty"`
	Model   string `json:"model,omitempty"`
	Family  string `json:"family,omitempty"`
	Flags   string `json:"flags,omitempty"`
}

// BootInfo describes the boot mode of a node.
type BootInfo struct {
	SecureBoot FlexInt `json:"secureboot,omitempty"`
	Mode       string  `json:"mode,omitempty"`
}

// KernelInfo describes the running kernel of a node.
type KernelInfo struct {
	Machine string `json:"machine,omitempty"`
	Release string `json:"release,omitempty"`
	Sysname string `json:"sysname,omitempty"`
	Version string `json:"version,omitempty"`
}

// NodeStatus is the status of a single node from GET /nodes/{node}/status.
type NodeStatus struct {
	CPU           float64    `json:"cpu,omitempty"`
	CPUInfo       CPUInfo    `json:"cpuinfo,omitzero"`
	BootInfo      BootInfo   `json:"boot-info,omitzero"`
	CurrentKernel KernelInfo `json:"current-kernel,omitzero"`
	Idle          float64    `json:"idle,omitempty"`
	KSM           KSMInfo    `json:"ksm,omitzero"`
	KVersion      string     `json:"kversion,omitempty"`
	LoadAvg       LoadAvg    `json:"loadavg,omitempty"`
	Memory        MemoryInfo `json:"memory,omitzero"`
	PVEVersion    string     `json:"pveversion,omitempty"`
	RootFS        RootFSInfo `json:"rootfs,omitzero"`
	Swap          SwapInfo   `json:"swap,omitzero"`
	Uptime        int64      `json:"uptime,omitempty"`
	Wait          float64    `json:"wait,omitempty"`
}
