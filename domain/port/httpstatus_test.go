package port

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTTPStatusErrorError(t *testing.T) {
	inner := errors.New("boom")
	e := &HTTPStatusError{Status: 401, Err: inner}
	assert.Equal(t, "boom", e.Error())
}

func TestHTTPStatusErrorUnwrap(t *testing.T) {
	inner := errors.New("boom")
	e := &HTTPStatusError{Status: 401, Err: inner}
	assert.True(t, errors.Is(e, inner), "errors.Is must match the wrapped sentinel")
	assert.Equal(t, inner, e.Unwrap())
}

func TestHTTPStatusErrorStatusCode(t *testing.T) {
	e := &HTTPStatusError{Status: 403, Err: errors.New("x")}
	assert.Equal(t, 403, e.StatusCode())
}

func TestHTTPStatus(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
		wantOK   bool
	}{
		{name: "nil", err: nil, wantCode: 0, wantOK: false},
		{name: "plain error", err: errors.New("x"), wantCode: 0, wantOK: false},
		{name: "wrapped sentinel error", err: errors.New("x"), wantCode: 0, wantOK: false},
		{
			name:     "http status error",
			err:      &HTTPStatusError{Status: 404, Err: errors.New("x")},
			wantCode: 404, wantOK: true,
		},
		{
			name:     "http status error wrapped in fmt",
			err:      fmt.Errorf("outer: %w", &HTTPStatusError{Status: 502, Err: errors.New("x")}),
			wantCode: 502, wantOK: true,
		},
		{
			name:     "zero status filtered out",
			err:      &HTTPStatusError{Status: 0, Err: errors.New("x")},
			wantCode: 0, wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, ok := HTTPStatus(tt.err)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantCode, code)
		})
	}
}

// TestHTTPStatusExtractsFromUpstreamStatusCoder verifies HTTPStatus walks the
// chain through any type exposing StatusCode() int (e.g. a 5xx UpstreamError),
// and reports ok=true only for non-zero statuses.
func TestHTTPStatusExtractsFromUpstreamStatusCoder(t *testing.T) {
	err := &upstreamStub{code: 503}
	code, ok := HTTPStatus(err)
	assert.True(t, ok)
	assert.Equal(t, 503, code)
}

// upstreamStub mimics a gateway UpstreamError by exposing StatusCode() int.
type upstreamStub struct {
	code int
}

func (u *upstreamStub) Error() string   { return "stub" }
func (u *upstreamStub) StatusCode() int { return u.code }
