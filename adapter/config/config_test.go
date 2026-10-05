package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// clearEnv removes all mcp-proxmox env vars to give each test a clean slate.
func clearEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"PVE_ENDPOINT", "PVE_TOKEN", "PVE_CA_CERT_PATH",
		"PBS_ENDPOINT", "PBS_TOKEN", "PBS_CA_CERT_PATH",
		"LOG_LEVEL", "LOG_FILENAME", "LOG_FORMAT",
		"ENABLE_MUTATIONS",
	} {
		t.Setenv(k, "")
	}
	// ENABLE_MUTATIONS="" is normalized to "false" by Load(), so setting it to
	// empty here yields a clean disabled baseline for every test.
}

func TestLoad(t *testing.T) {
	t.Run("empty env loads with all disabled", func(t *testing.T) {
		clearEnv(t)
		c, err := Load()
		require.NoError(t, err)
		assert.Equal(t, "", c.PVEEndpoint)
		assert.Equal(t, "", c.PVEToken)
		assert.False(t, c.PVEEnabled())
		assert.False(t, c.PBSEnabled())
		assert.Equal(t, "", c.LogLevel)
		assert.False(t, c.EnableMutations)
	})

	t.Run("full env", func(t *testing.T) {
		clearEnv(t)
		t.Setenv("PVE_ENDPOINT", "https://pve:8006")
		t.Setenv("PVE_TOKEN", "user@pam!t=secret")
		t.Setenv("PVE_CA_CERT_PATH", "/certs/pve.pem")
		t.Setenv("PBS_ENDPOINT", "https://pbs:8007")
		t.Setenv("PBS_TOKEN", "user@pbs!t=secret")
		t.Setenv("PBS_CA_CERT_PATH", "/certs/pbs.pem")
		t.Setenv("LOG_LEVEL", "debug")
		t.Setenv("LOG_FORMAT", "json")
		t.Setenv("LOG_FILENAME", "/var/log/mcp.log")
		t.Setenv("ENABLE_MUTATIONS", "true")

		c, err := Load()
		require.NoError(t, err)
		assert.Equal(t, "https://pve:8006", c.PVEEndpoint)
		assert.Equal(t, "user@pam!t=secret", c.PVEToken)
		assert.Equal(t, "/certs/pve.pem", c.PVECACertPath)
		assert.True(t, c.PVEEnabled())
		assert.Equal(t, "https://pbs:8007", c.PBSEndpoint)
		assert.Equal(t, "user@pbs!t=secret", c.PBSToken)
		assert.Equal(t, "/certs/pbs.pem", c.PBSCACertPath)
		assert.True(t, c.PBSEnabled())
		assert.Equal(t, "debug", c.LogLevel)
		assert.Equal(t, "json", c.LogFormat)
		assert.Equal(t, "/var/log/mcp.log", c.LogFileName)
		assert.True(t, c.EnableMutations)
	})
}

// TestEnableMutations parses the ENABLE_MUTATIONS env var (bool, default
// false) into Config.EnableMutations. An explicitly-set empty string is
// normalized by Load() to false and must not fail.
func TestEnableMutations(t *testing.T) {
	tests := []struct {
		name string
		set  bool
		val  string
		want bool
	}{
		{"unset defaults false", false, "", false},
		{"empty string treated false", true, "", false},
		{"true", true, "true", true},
		{"1", true, "1", true},
		{"false", true, "false", false},
		{"0", true, "0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearEnv(t)
			if tt.set {
				t.Setenv("ENABLE_MUTATIONS", tt.val)
			}
			c, err := Load()
			require.NoError(t, err)
			assert.Equal(t, tt.want, c.EnableMutations)
		})
	}
}

func TestLoggingDefaults(t *testing.T) {
	clearEnv(t)
	c, err := Load()
	require.NoError(t, err)
	// LOG_LEVEL unset -> logging disabled; LOG_FILENAME / LOG_FORMAT default.
	assert.Equal(t, "", c.LogLevel)
	assert.Equal(t, "", c.LogFileName)
	assert.Equal(t, "", c.LogFormat)
}

func TestPVEEnabledPredicate(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		token    string
		want     bool
	}{
		{"both set", "https://pve:8006", "tok", true},
		{"missing endpoint", "", "tok", false},
		{"missing token", "https://pve:8006", "", false},
		{"both missing", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Config{PVEEndpoint: tt.endpoint, PVEToken: tt.token}
			assert.Equal(t, tt.want, c.PVEEnabled())
		})
	}
}

func TestPBSEnabledPredicate(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		token    string
		want     bool
	}{
		{"both set", "https://pbs:8007", "tok", true},
		{"missing endpoint", "", "tok", false},
		{"missing token", "https://pbs:8007", "", false},
		{"both missing", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Config{PBSEndpoint: tt.endpoint, PBSToken: tt.token}
			assert.Equal(t, tt.want, c.PBSEnabled())
		})
	}
}
