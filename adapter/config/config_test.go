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
		"LOG_LEVEL",
	} {
		t.Setenv(k, "")
	}
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
	})
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
