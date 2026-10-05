package logging

import (
	"context"
	"log/slog"

	"github.com/sirupsen/logrus"
)

// LogrusHandler is a slog.Handler that writes to logrus.
// It lets the internal logs of go-sdk (which only accepts a *slog.Logger)
// go into a single logrus journal. See SPEC.md §5.1.
type LogrusHandler struct {
	log   *logrus.Logger
	level slog.Level
	attrs []slog.Attr
}

// NewLogrusHandler creates a LogrusHandler on top of a logrus.Logger.
func NewLogrusHandler(l *logrus.Logger) *LogrusHandler {
	return &LogrusHandler{log: l}
}

// Enabled drops records below the logrus threshold.
func (h *LogrusHandler) Enabled(_ context.Context, l slog.Level) bool {
	return h.log.IsLevelEnabled(slogToLogrus(l))
}

// Handle maps slog.Level -> logrus.Level, writes rec.Message,
// and spreads rec.Attrs into logrus.WithFields(...).
func (h *LogrusHandler) Handle(_ context.Context, rec slog.Record) error {
	entry := h.log.WithFields(attrsToFields(h.attrs))
	rec.Attrs(func(a slog.Attr) bool {
		entry = entry.WithField(a.Key, a.Value.Any())
		return true
	})
	entry.Log(slogToLogrus(rec.Level), rec.Message)
	return nil
}

// WithAttrs accumulates attributes (for nested contexts).
func (h *LogrusHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	merged := make([]slog.Attr, 0, len(h.attrs)+len(attrs))
	merged = append(merged, h.attrs...)
	merged = append(merged, attrs...)
	return &LogrusHandler{log: h.log, level: h.level, attrs: merged}
}

// WithGroup accumulates a group (key prefix).
func (h *LogrusHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	prefixed := make([]slog.Attr, 0, len(h.attrs))
	for _, a := range h.attrs {
		prefixed = append(prefixed, slog.Attr{Key: name + "." + a.Key, Value: a.Value})
	}
	return &LogrusHandler{log: h.log, level: h.level, attrs: prefixed}
}

// NewSlogLogger returns a *slog.Logger that writes to logrus.
func NewSlogLogger(l *logrus.Logger) *slog.Logger {
	return slog.New(NewLogrusHandler(l))
}

// slogToLogrus maps an slog level to logrus.
func slogToLogrus(l slog.Level) logrus.Level {
	switch {
	case l <= slog.LevelDebug:
		return logrus.TraceLevel
	case l < slog.LevelWarn:
		return logrus.InfoLevel
	case l < slog.LevelError:
		return logrus.WarnLevel
	default:
		return logrus.ErrorLevel
	}
}

// attrsToFields spreads the accumulated attributes into a map for WithFields.
func attrsToFields(attrs []slog.Attr) logrus.Fields {
	if len(attrs) == 0 {
		return nil
	}
	f := make(logrus.Fields, len(attrs))
	for _, a := range attrs {
		f[a.Key] = a.Value.Any()
	}
	return f
}
