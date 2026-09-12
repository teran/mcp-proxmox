package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDescWithInstructions(t *testing.T) {
	assert.Equal(t, "just a desc", descWithInstructions("just a desc", ""))
	assert.Equal(t,
		"desc\n\nInstructions: guidance",
		descWithInstructions("desc", "guidance"))
}

func TestBoolPtr(t *testing.T) {
	tf := boolPtr(true)
	ff := boolPtr(false)
	assert.NotNil(t, tf)
	assert.NotNil(t, ff)
	assert.True(t, *tf)
	assert.False(t, *ff)
}

func TestWrapOutput(t *testing.T) {
	// nil passes through.
	assert.Nil(t, wrapOutput(nil))

	// slice -> {"items": [...]}
	out := wrapOutput([]string{"a"})
	m, ok := out.(map[string]any)
	assert.True(t, ok)
	_, hasItems := m["items"]
	assert.True(t, hasItems)

	// pointer to slice -> wrapped
	out = wrapOutput(&[]int{1})
	_, ok = out.(map[string]any)
	assert.True(t, ok)

	// struct -> passes through
	type s struct{ X int }
	assert.IsType(t, s{}, wrapOutput(s{X: 1}))

	// scalar -> {"value": ...}
	v, ok := wrapOutput(42).(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, 42, v["value"])
}
