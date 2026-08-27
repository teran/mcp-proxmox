package logging

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/teran/mcp-proxmox/domain/port"
)

// WithSession returns a logrus entry carrying the MCP session ID and the
// per-request ID from ctx (if present). Use it as the base for request-scoped
// log lines so that session_id and request_id are added consistently without
// repeating them at every call site.
func WithSession(ctx context.Context, l *logrus.Logger) *logrus.Entry {
	e := l.WithContext(ctx)
	if sid, ok := port.SessionIDFromContext(ctx); ok && sid != "" {
		e = e.WithField("session_id", sid)
	}
	if rid, ok := port.RequestIDFromContext(ctx); ok && rid != "" {
		e = e.WithField("request_id", rid)
	}
	return e
}
