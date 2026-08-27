// Package model defines the pure entities / value objects of mcp-proxmox. It
// imports stdlib only (encoding/json, time) and never logs. See SPEC.md §2.
//
// The JSON field tags mirror the real Proxmox VE / PBS REST API responses
// (/api2/json) so the adapters can unmarshal directly into these types and the
// test suite can assert exact JSON round-trips.
package model
