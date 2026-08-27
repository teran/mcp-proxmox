package port

import "context"

// AppLogger is the minimal logging contract for the core (application).
// Implemented by adapter/logging on top of logrus. The core does NOT see logrus.
type AppLogger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// RequestAwareLogger is an OPTIONAL extension of AppLogger whose methods accept
// a context and tag each emitted line with the session/request ID present in it
// (via SessionIDFromContext / RequestIDFromContext). Application services
// type-assert their AppLogger to this interface so lifecycle lines carry the
// same session_id / request_id as the tool lines. Test/dummy loggers need not
// implement it — callers fall back to the plain AppLogger when it is absent.
type RequestAwareLogger interface {
	AppLogger
	DebugfContext(ctx context.Context, format string, args ...any)
	InfofContext(ctx context.Context, format string, args ...any)
	WarnfContext(ctx context.Context, format string, args ...any)
	ErrorfContext(ctx context.Context, format string, args ...any)
}

// CtxLogger is a context-aware logger used by the application and adapter
// layers. It wraps an AppLogger and, when the underlying logger implements
// RequestAwareLogger, tags each line with the session/request ID from ctx
// (native logrus field); otherwise it falls back to the plain AppLogger. A
// CtxLogger with a nil AppLogger is a no-op.
type CtxLogger struct {
	plain AppLogger
	aware RequestAwareLogger
}

// NewCtxLogger wraps l for context-aware logging.
func NewCtxLogger(l AppLogger) CtxLogger {
	return CtxLogger{plain: l, aware: ToRequestAwareLogger(l)}
}

// ToRequestAwareLogger returns the ctx-aware view of l, or nil when l does not
// implement RequestAwareLogger (e.g. a test dummy).
func ToRequestAwareLogger(l AppLogger) RequestAwareLogger {
	if r, ok := l.(RequestAwareLogger); ok {
		return r
	}
	return nil
}

// Debugf logs at debug level, attaching the session/request ID from ctx when
// the underlying logger supports it.
func (c CtxLogger) Debugf(ctx context.Context, format string, args ...any) {
	if c.aware != nil {
		c.aware.DebugfContext(ctx, format, args...)
		return
	}
	if c.plain != nil {
		c.plain.Debugf(format, args...)
	}
}

// Infof logs at info level, attaching the session/request ID from ctx when the
// underlying logger supports it.
func (c CtxLogger) Infof(ctx context.Context, format string, args ...any) {
	if c.aware != nil {
		c.aware.InfofContext(ctx, format, args...)
		return
	}
	if c.plain != nil {
		c.plain.Infof(format, args...)
	}
}

// Warnf logs at warning level, attaching the session/request ID from ctx when
// the underlying logger supports it.
func (c CtxLogger) Warnf(ctx context.Context, format string, args ...any) {
	if c.aware != nil {
		c.aware.WarnfContext(ctx, format, args...)
		return
	}
	if c.plain != nil {
		c.plain.Warnf(format, args...)
	}
}

// Errorf logs at error level, attaching the session/request ID from ctx when
// the underlying logger supports it.
func (c CtxLogger) Errorf(ctx context.Context, format string, args ...any) {
	if c.aware != nil {
		c.aware.ErrorfContext(ctx, format, args...)
		return
	}
	if c.plain != nil {
		c.plain.Errorf(format, args...)
	}
}
