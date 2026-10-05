package port

import "errors"

// HTTPStatusError carries the upstream HTTP status code alongside an underlying
// error while preserving errors.Is / errors.As matching against the wrapped
// sentinel. It is used so a 401/403/404 (mapped to a port sentinel) or a 5xx
// (mapped to an upstream error) can still surface its real status code to the
// MCP tool logger, which emits it as the upstream_http_status_code field.
type HTTPStatusError struct {
	Status int
	Err    error
}

func (e *HTTPStatusError) Error() string { return e.Err.Error() }

func (e *HTTPStatusError) Unwrap() error { return e.Err }

// StatusCode reports the upstream HTTP status carried by the error.
func (e *HTTPStatusError) StatusCode() int { return e.Status }

// HTTPStatus returns the upstream HTTP status code associated with err, if any
// (walking the error chain via errors.As). It reports ok=false when err carries
// no meaningful (non-zero) HTTP status, so callers omit the field rather than
// log upstream_http_status_code=0.
func HTTPStatus(err error) (int, bool) {
	type statusCoder interface{ StatusCode() int }
	var sc statusCoder
	if errors.As(err, &sc) && sc.StatusCode() > 0 {
		return sc.StatusCode(), true
	}
	return 0, false
}
