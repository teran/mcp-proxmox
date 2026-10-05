// Package port defines the domain contracts of mcp-proxmox: the secondary
// ports (gateways, token source, logger) that the application layer depends on,
// plus the sentinel errors and context helpers shared across layers.
//
// This package imports stdlib only. It must never import the MCP SDK,
// net/http, logrus, envconfig, or any Proxmox client (enforced by go-arch-lint
// and the depguard rule in .golangci.yml).
package port

import "errors"

// Sentinel errors of the port. They are returned by the adapter/pve and
// adapter/pbs gateways and checked by the application layer and the MCP
// adapter (e.g. for mapping into user-facing messages). See SPEC.md §7.
var (
	// ErrUnauthorized — HTTP 401 / invalid or missing API token.
	ErrUnauthorized = errors.New("proxmox: unauthorized")
	// ErrForbidden — HTTP 403 / insufficient privileges.
	ErrForbidden = errors.New("proxmox: forbidden")
	// ErrNotFound — HTTP 404 / resource not found.
	ErrNotFound = errors.New("proxmox: not found")
)
