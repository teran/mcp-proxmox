// Package pve implements port.PVEGateway as an HTTP client for the Proxmox VE
// REST API (/api2/json). This is the ONLY place net/http is used for PVE.
//
// Scaffold note: the read-only gateway method bodies are stubs (they return
// ErrNotImplemented) until the developer fills in the real request/response/
// error/retry handling described in SPEC.md §7. The Config struct, constructors
// and the port method signatures are the stable contract.
package pve

import (
	"errors"
)

// ErrNotImplemented marks the scaffold stub bodies. It is replaced by the
// developer's real implementation; it must never be returned in a final release.
var ErrNotImplemented = errors.New("pve: not implemented")

// UpstreamError wraps transport / HTTP 5xx / decode / marshal errors. It is the
// ONLY error kind that the gateway retries (SPEC.md §7.2 / §7.5).
type UpstreamError struct {
	Op     string
	Status int
	Err    error
}

func (e *UpstreamError) Error() string {
	return "pve: upstream error (op=" + e.Op + "): " + e.Err.Error()
}

func (e *UpstreamError) Unwrap() error { return e.Err }

// APIError is a semantic error from the Proxmox JSON body ({"errors": {...}} or
// the data/message envelope). It is never retried.
type APIError struct {
	Messages []string
}

func (e *APIError) Error() string {
	if len(e.Messages) == 0 {
		return "pve: api error"
	}
	return "pve: api error: " + errors.Join(messagesToErrors(e.Messages)...).Error()
}

func messagesToErrors(m []string) []error {
	out := make([]error, 0, len(m))
	for _, s := range m {
		out = append(out, errors.New(s))
	}
	return out
}

// EmptyDataError indicates that a getter received an empty/null `data` field
// from the API, i.e. the requested single entity does not exist. Getters never
// return a silently-zero struct; a missing entity surfaces as an explicit error.
// It is not retried.
type EmptyDataError struct {
	Op string
}

func (e *EmptyDataError) Error() string {
	return "pve: empty data (op=" + e.Op + ")"
}
