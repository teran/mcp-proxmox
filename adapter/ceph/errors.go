// Package ceph implements port.CephGateway as an HTTP client for the
// PVE-managed Ceph subset of the Proxmox VE REST API (/api2/json). This is the
// ONLY place net/http is used for Ceph. Ceph rides the PVE API token
// (Authorization: PVEAPIToken=...), so it shares the PVE endpoint/token
// configuration (SPEC.md §3.4 / §4.5).
package ceph

import (
	"errors"
)

// ErrNotImplemented marks scaffold stub bodies. It is defined for parity with
// the pve/pbs adapters (and required by the error-sentinel contract) but is
// never returned by the ceph gateway — every method is implemented.
var ErrNotImplemented = errors.New("ceph: not implemented")

// UpstreamError wraps transport / HTTP 5xx / decode / marshal errors. It is the
// ONLY error kind that the gateway retries (SPEC.md §7.2 / §7.5).
type UpstreamError struct {
	Op     string
	Status int
	Err    error
}

func (e *UpstreamError) Error() string {
	return "ceph: upstream error (op=" + e.Op + "): " + e.Err.Error()
}

func (e *UpstreamError) Unwrap() error { return e.Err }

// StatusCode exposes the upstream HTTP status (0 for non-HTTP ops, so
// port.HTTPStatus filters it out).
func (e *UpstreamError) StatusCode() int { return e.Status }

// APIError is a semantic error from the Proxmox JSON body
// ({"errors": {...}} or the data/message envelope). It is never retried.
type APIError struct {
	Messages []string
}

func (e *APIError) Error() string {
	if len(e.Messages) == 0 {
		return "ceph: api error"
	}

	return "ceph: api error: " + errors.Join(messagesToErrors(e.Messages)...).Error()
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
	return "ceph: empty data (op=" + e.Op + ")"
}
