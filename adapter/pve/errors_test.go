package pve

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpstreamError(t *testing.T) {
	inner := errors.New("boom")
	e := &UpstreamError{Op: "http", Status: 500, Err: inner}
	assert.Equal(t, "pve: upstream error (op=http): boom", e.Error())
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
		assert.Equal(t, "pve: api error", e.Error())
	})
}

func TestMessagesToErrors(t *testing.T) {
	out := messagesToErrors([]string{"a", "b"})
	assert.Len(t, out, 2)
	assert.Equal(t, "a", out[0].Error())
	assert.Equal(t, "b", out[1].Error())

	empty := messagesToErrors(nil)
	assert.Len(t, empty, 0)
}

func TestErrNotImplementedIsDefined(t *testing.T) {
	assert.Error(t, ErrNotImplemented)
}
