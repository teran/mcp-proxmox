package mcp

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/teran/mcp-proxmox/adapter/logging"
	"github.com/teran/mcp-proxmox/domain/port"
)

// toolLogger is a context-aware logger for MCP tool handlers. It decorates each
// entry with the session/request ID from the context (via logging.WithSession),
// so tool-level log lines carry the same session_id / request_id as the
// underlying application/gateway logs. Falls back to the plain AppLogger when
// no logrus logger is available.
type toolLogger struct {
	log port.AppLogger
	l   *logrus.Logger
}

func (t toolLogger) entry(ctx context.Context) *logrus.Entry {
	if t.l == nil {
		return nil
	}
	return logging.WithSession(ctx, t.l)
}

func (t toolLogger) Debugf(ctx context.Context, format string, args ...any) {
	if e := t.entry(ctx); e != nil {
		e.Debugf(format, args...)
		return
	}
	t.log.Debugf(format, args...)
}

func (t toolLogger) Infof(ctx context.Context, format string, args ...any) {
	if e := t.entry(ctx); e != nil {
		e.Infof(format, args...)
		return
	}
	t.log.Infof(format, args...)
}

func (t toolLogger) Warnf(ctx context.Context, format string, args ...any) {
	if e := t.entry(ctx); e != nil {
		e.Warnf(format, args...)
		return
	}
	t.log.Warnf(format, args...)
}

func (t toolLogger) Errorf(ctx context.Context, format string, args ...any) {
	if e := t.entry(ctx); e != nil {
		e.Errorf(format, args...)
		return
	}
	t.log.Errorf(format, args...)
}
