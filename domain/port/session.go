package port

import "context"

// sessionIDCtxKey is a private key for storing the MCP session ID in context.
type sessionIDCtxKey struct{}

// WithSessionID stores an MCP session ID in the context so that request-level
// loggers can correlate log lines per session. The value is not sensitive (it
// identifies a connection, not credentials).
func WithSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessionIDCtxKey{}, id)
}

// SessionIDFromContext returns the session ID set by WithSessionID.
func SessionIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(sessionIDCtxKey{}).(string)
	return id, ok
}

// requestIDCtxKey is a private key for storing the per-request ID in context.
type requestIDCtxKey struct{}

// WithRequestID stores a per-request ID in the context so all log lines from a
// single MCP tool call share it and can be correlated.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDCtxKey{}, id)
}

// RequestIDFromContext returns the request ID set by WithRequestID.
func RequestIDFromContext(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestIDCtxKey{}).(string)
	return id, ok
}
