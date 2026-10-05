package mcp

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/teran/mcp-proxmox/domain/port"
)

func TestNewSessionID(t *testing.T) {
	a := NewSessionID()
	b := NewSessionID()
	assert.NotEmpty(t, a)
	assert.NotEmpty(t, b)
	assert.NotEqual(t, a, b, "session IDs should differ across calls")
	// 8 random bytes -> 16 hex chars.
	assert.Len(t, a, 16)
}

func TestNewRequestID(t *testing.T) {
	a := newRequestID()
	b := newRequestID()
	assert.NotEmpty(t, a)
	assert.NotEqual(t, a, b)
	assert.Len(t, a, 16)
}

func TestWithSessionContext(t *testing.T) {
	t.Run("adds request id always", func(t *testing.T) {
		deps := Deps{}
		fn := withSessionContext(deps)
		ctx := fn(context.Background())
		rid, ok := port.RequestIDFromContext(ctx)
		assert.True(t, ok)
		assert.NotEmpty(t, rid)
		if _, ok := port.SessionIDFromContext(ctx); ok {
			t.Fatal("session id should not be set when deps.SessionID empty")
		}
	})

	t.Run("adds session id when configured", func(t *testing.T) {
		deps := Deps{SessionID: "sess-abc"}
		fn := withSessionContext(deps)
		ctx := fn(context.Background())
		sid, ok := port.SessionIDFromContext(ctx)
		assert.True(t, ok)
		assert.Equal(t, "sess-abc", sid)
	})
}

// fakeTransport implements mcpSDK.Transport by delegating to a controllable
// connection factory.
type fakeTransport struct {
	connectFn func(ctx context.Context) (mcpSDK.Connection, error)
}

func (f *fakeTransport) Connect(ctx context.Context) (mcpSDK.Connection, error) {
	if f.connectFn != nil {
		return f.connectFn(ctx)
	}
	return nil, nil
}

func (f *fakeTransport) Close() error { return nil }

// fakeConnection is a minimal mcpSDK.Connection stub.
type fakeConnection struct {
	mcpSDK.Connection
}

func TestSessionTransportWrapsConnection(t *testing.T) {
	inner := &fakeTransport{}
	tp := NewSessionTransport(inner, "sess-xyz")

	conn, err := tp.Connect(context.Background())
	require.NoError(t, err)
	sc, ok := conn.(*sessionConn)
	require.True(t, ok, "connection should be wrapped in sessionConn")
	assert.Equal(t, "sess-xyz", sc.SessionID())
}

func TestSessionTransportPropagatesError(t *testing.T) {
	inner := &fakeTransport{connectFn: func(ctx context.Context) (mcpSDK.Connection, error) {
		return nil, context.Canceled
	}}
	tp := NewSessionTransport(inner, "s")
	_, err := tp.Connect(context.Background())
	assert.ErrorIs(t, err, context.Canceled)
}

func TestSessionConnDelegates(t *testing.T) {
	base := &fakeConnection{}
	sc := &sessionConn{Connection: base, id: "id"}
	assert.Equal(t, "id", sc.SessionID())
	// The embedded interface should delegate calls to the inner connection.
	_ = base
}
