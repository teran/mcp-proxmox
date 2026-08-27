package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVersionString(t *testing.T) {
	// versionString uses the package-level `version` var (default v0.0.0-dev,
	// overridable via -ldflags -X main.version=...).
	assert.Equal(t, "mcp-proxmox "+version, versionString())
}
