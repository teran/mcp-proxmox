// Package logging wires logrus for mcp-proxmox: the *logrus.Logger, the
// port.AppLogger view the core sees, a slog→logrus adapter for the go-sdk's
// internal logs, and context-aware session tagging. See SPEC.md §5.
package logging

import (
	"strings"

	"github.com/sirupsen/logrus"
)

// NewLogger creates a configured *logrus.Logger.
// level: trace|debug|info|warn|error; format: text|json.
func NewLogger(level, format string) *logrus.Logger {
	l := logrus.New()
	l.SetLevel(parseLevel(level))
	if strings.EqualFold(format, "json") {
		l.SetFormatter(&logrus.JSONFormatter{})
	} else {
		// FullTimestamp: true shows absolute wall-clock time instead of the
		// relative "INFO[0002]" elapsed duration, which is more convenient when
		// correlating logs across sessions/requests.
		l.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})
	}
	return l
}

// parseLevel maps a level string to logrus.Level (default info).
func parseLevel(level string) logrus.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "trace":
		return logrus.TraceLevel
	case "debug":
		return logrus.DebugLevel
	case "warn", "warning":
		return logrus.WarnLevel
	case "error":
		return logrus.ErrorLevel
	default:
		return logrus.InfoLevel
	}
}
