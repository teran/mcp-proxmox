package mcp

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"
)

// sessionConn wraps a mcpSDK.Connection and overrides SessionID to return the
// caller-supplied id. The go-sdk's ioConn (used by stdio) hardcodes
// SessionID() to return "" (transport.go), so without this wrapper the SDK logs
// "session_id=" (empty) on every connect/disconnect.
type sessionConn struct {
	mcpSDK.Connection
	id string
}

// SessionID returns the session ID supplied when the transport was created.
func (c *sessionConn) SessionID() string { return c.id }

// sessionTransport wraps a stdio transport so the connection it produces
// reports a generated session ID instead of the SDK's hardcoded empty string.
type sessionTransport struct {
	inner mcpSDK.Transport
	id    string
}

// Connect delegates to the inner transport and wraps the returned connection so
// that SessionID() returns the session ID.
func (t *sessionTransport) Connect(ctx context.Context) (mcpSDK.Connection, error) {
	c, err := t.inner.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &sessionConn{Connection: c, id: t.id}, nil
}

// NewSessionTransport wraps inner (typically &mcpSDK.StdioTransport{}) so the
// connection it produces reports id as its session ID. The same id should also
// be passed to Deps.SessionID so request-context loggers (tool and application
// lines) carry session_id.
func NewSessionTransport(inner mcpSDK.Transport, id string) mcpSDK.Transport {
	return &sessionTransport{inner: inner, id: id}
}

// NewSessionID returns a short random hex string used as a per-session ID. It
// reuses the same generator as newRequestID.
func NewSessionID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}
