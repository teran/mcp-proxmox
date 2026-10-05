package pbs

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpstreamError(t *testing.T) {
	inner := errors.New("boom")
	e := &UpstreamError{Op: "http", Status: 502, Err: inner}
	assert.Equal(t, "pbs: upstream error (op=http): boom", e.Error())
	assert.ErrorIs(t, e, inner)
}

func TestAPIError(t *testing.T) {
	t.Run("with messages", func(t *testing.T) {
		e := &APIError{Messages: []string{"first", "second"}}
		assert.Contains(t, e.Error(), "first")
		assert.Contains(t, e.Error(), "second")
	})

	t.Run("no messages", func(t *testing.T) {
		e := &APIError{}
		assert.Equal(t, "pbs: api error", e.Error())
	})
}

func TestMessageErrs(t *testing.T) {
	out := messageErrs([]string{"a", "b"})
	assert.Len(t, out, 2)
	assert.Equal(t, "a", out[0].Error())
	assert.Equal(t, "b", out[1].Error())

	empty := messageErrs(nil)
	assert.Len(t, empty, 0)
}

func TestErrNotImplementedIsDefined(t *testing.T) {
	assert.Error(t, ErrNotImplemented)
}

func TestUpstreamErrorStatusCode(t *testing.T) {
	e := &UpstreamError{Op: "http", Status: 502}
	assert.Equal(t, 502, e.StatusCode())
	zero := &UpstreamError{Op: "transport"}
	assert.Equal(t, 0, zero.StatusCode())
}
