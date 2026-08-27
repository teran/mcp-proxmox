package logging

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/teran/mcp-proxmox/domain/port"
)

// appLogger implements port.AppLogger (and port.RequestAwareLogger) on top of
// logrus. The ctx-aware methods use WithSession so lifecycle lines emitted by
// the application services carry the same session_id / request_id as the tool
// lines.
type appLogger struct {
	l *logrus.Logger
}

// NewAppLogger creates a port.AppLogger on top of a logrus.Logger.
func NewAppLogger(l *logrus.Logger) port.AppLogger {
	return &appLogger{l: l}
}

func (a *appLogger) Debugf(format string, args ...any) { a.l.Debugf(format, args...) }
func (a *appLogger) Infof(format string, args ...any)  { a.l.Infof(format, args...) }
func (a *appLogger) Warnf(format string, args ...any)  { a.l.Warnf(format, args...) }
func (a *appLogger) Errorf(format string, args ...any) { a.l.Errorf(format, args...) }

// DebugfContext logs at debug level tagged with the session/request ID from ctx.
func (a *appLogger) DebugfContext(ctx context.Context, format string, args ...any) {
	WithSession(ctx, a.l).Debugf(format, args...)
}

// InfofContext logs at info level tagged with the session/request ID from ctx.
func (a *appLogger) InfofContext(ctx context.Context, format string, args ...any) {
	WithSession(ctx, a.l).Infof(format, args...)
}

// WarnfContext logs at warning level tagged with the session/request ID from ctx.
func (a *appLogger) WarnfContext(ctx context.Context, format string, args ...any) {
	WithSession(ctx, a.l).Warnf(format, args...)
}

// ErrorfContext logs at error level tagged with the session/request ID from ctx.
func (a *appLogger) ErrorfContext(ctx context.Context, format string, args ...any) {
	WithSession(ctx, a.l).Errorf(format, args...)
}
