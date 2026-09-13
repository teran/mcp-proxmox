// Package config loads the mcp-proxmox server configuration from environment
// variables via kelseyhightower/envconfig. It replaces the earlier YAML
// (~/.config/mcp-proxmox/config.yml) model: both the endpoint/token credentials
// and the log level are now environment variables. See SPEC.md §4.4.
//
// A backend is ENABLED only when both its endpoint and token are non-empty;
// otherwise it is disabled and its tools are not registered (nothing fails).
package config

import (
	"os"

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
	// LogLevel maps to LOG_LEVEL. When set (any non-empty value), logging is
	// ENABLED and logs are written to LogFileName (default /tmp/mcp-proxmox.log)
	// at that level (stdio-friendly file sink); when unset, logging is DISABLED
	// and no logs are emitted anywhere (SPEC.md L2).
	LogLevel string `envconfig:"LOG_LEVEL" desc:"Log level: trace|debug|info|warn|error. When set, enables logging to LOG_FILENAME."`
	// LogFileName maps to LOG_FILENAME, the stdio-friendly file sink used when
	// LOG_LEVEL is set. Default /tmp/mcp-proxmox.log (mode 0600).
	LogFileName string `envconfig:"LOG_FILENAME" desc:"Path of the log file (default /tmp/mcp-proxmox.log, mode 0600). Only used when LOG_LEVEL is set."`
	// LogFormat maps to LOG_FORMAT: "text" (default, logrus text with full
	// timestamps) or "json".
	LogFormat string `envconfig:"LOG_FORMAT" desc:"Log format: text (default) | json."`

	// EnableMutations maps to ENABLE_MUTATIONS. When false (default), mutation
	// tools (pve_vm_create, pbs_verify_start, ...) are NOT registered; only the
	// read-only query surface is exposed. When true, mutation tools are
	// registered for enabled backends (SPEC.md §2.5).
	EnableMutations bool `envconfig:"ENABLE_MUTATIONS" default:"false" desc:"Register mutation tools (default false)."`
}

// Load reads the configuration from environment variables. It never fails on a
// missing variable — envconfig leaves fields empty, and empty endpoint/token
// simply mean the backend is disabled.
func Load() (Config, error) {
	// envconfig fails to parse an explicitly-set-empty value for a bool field
	// (strconv.ParseBool("")). Normalize ENABLE_MUTATIONS="" -> "false" so an
	// empty value means "disabled" (SPEC.md §2.5) instead of failing to load.
	// Unset behaviour is unchanged: default:"false" already yields false.
	if v, ok := os.LookupEnv("ENABLE_MUTATIONS"); ok && v == "" {
		if err := os.Setenv("ENABLE_MUTATIONS", "false"); err != nil {
			return Config{}, err
		}
	}

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
