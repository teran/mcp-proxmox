// Package config loads the mcp-proxmox server configuration from environment
// variables via kelseyhightower/envconfig. It replaces the earlier YAML
// (~/.config/mcp-proxmox/config.yml) model: both the endpoint/token credentials
// and the log level are now environment variables. See SPEC.md §4.4.
//
// A backend is ENABLED only when both its endpoint and token are non-empty;
// otherwise it is disabled and its tools are not registered (nothing fails).
package config

import (
	"github.com/kelseyhightower/envconfig"
)

// Config is the full server configuration, loaded once at startup by Load().
type Config struct {
	// PVE backend.
	PVEEndpoint   string `envconfig:"PVE_ENDPOINT" desc:"Proxmox VE API endpoint (e.g. https://pve.example.com:8006). Empty disables the PVE backend."`
	PVEToken      string `envconfig:"PVE_TOKEN" desc:"Proxmox VE API token (user@realm!tokenid=uuid). Empty disables the PVE backend."`
	PVECACertPath string `envconfig:"PVE_CA_CERT_PATH" desc:"Optional custom CA (PEM) used to verify the PVE TLS endpoint. Empty = system roots."`

	// PBS backend.
	PBSEndpoint   string `envconfig:"PBS_ENDPOINT" desc:"Proxmox Backup Server API endpoint (e.g. https://pbs.example.com:8007). Empty disables the PBS backend."`
	PBSToken      string `envconfig:"PBS_TOKEN" desc:"Proxmox Backup Server API token (user@pbs!tokenid=uuid). Empty disables the PBS backend."`
	PBSCACertPath string `envconfig:"PBS_CA_CERT_PATH" desc:"Optional custom CA (PEM) used to verify the PBS TLS endpoint. Empty = system roots."`

	// Logging.
	// LogLevel maps to LOG_LEVEL. When set (any non-empty value), logs are
	// written to /tmp/mcp-proxmox.log at that level (stdio-friendly file sink);
	// when unset, the default info level and default destination are used.
	LogLevel string `envconfig:"LOG_LEVEL" desc:"Log level: trace|debug|info|warn|error. When set, logs go to /tmp/mcp-proxmox.log."`
}

// Load reads the configuration from environment variables. It never fails on a
// missing variable — envconfig leaves fields empty, and empty endpoint/token
// simply mean the backend is disabled.
func Load() (Config, error) {
	var c Config
	if err := envconfig.Process("", &c); err != nil {
		return c, err
	}
	return c, nil
}

// PVEEnabled reports whether the PVE backend is active (both endpoint and token
// are non-empty).
func (c Config) PVEEnabled() bool {
	return c.PVEEndpoint != "" && c.PVEToken != ""
}

// PBSEnabled reports whether the PBS backend is active (both endpoint and token
// are non-empty).
func (c Config) PBSEnabled() bool {
	return c.PBSEndpoint != "" && c.PBSToken != ""
}
