package port

import (
	"context"
	"testing"
)

func TestSessionIDRoundTrip(t *testing.T) {
	ctx := context.Background()
	if _, ok := SessionIDFromContext(ctx); ok {
		t.Fatal("expected no session ID in empty context")
	}

	got := WithSessionID(ctx, "sess-123")
	id, ok := SessionIDFromContext(got)
	if !ok || id != "sess-123" {
		t.Fatalf("got session ID %q, %v; want %q, true", id, ok, "sess-123")
	}

	// Empty ID is still stored and retrievable.
	got2 := WithSessionID(ctx, "")
	id2, ok2 := SessionIDFromContext(got2)
	if !ok2 || id2 != "" {
		t.Fatalf("empty ID not round-tripped: %q, %v", id2, ok2)
	}

	// A different type stored under a different key must not collide.
	withTok := WithToken(ctx, "Bearer x")
	if _, ok := SessionIDFromContext(withTok); ok {
		t.Fatal("token context must not expose a session ID")
	}
}

func TestRequestIDRoundTrip(t *testing.T) {
	ctx := context.Background()
	if _, ok := RequestIDFromContext(ctx); ok {
		t.Fatal("expected no request ID in empty context")
	}

	got := WithRequestID(ctx, "req-456")
	id, ok := RequestIDFromContext(got)
	if !ok || id != "req-456" {
		t.Fatalf("got request ID %q, %v; want %q, true", id, ok, "req-456")
	}
}

func TestSessionAndRequestIDsCoexist(t *testing.T) {
	ctx := context.Background()
	ctx = WithSessionID(ctx, "s1")
	ctx = WithRequestID(ctx, "r1")

	if sid, _ := SessionIDFromContext(ctx); sid != "s1" {
		t.Fatalf("session ID = %q, want s1", sid)
	}
	if rid, _ := RequestIDFromContext(ctx); rid != "r1" {
		t.Fatalf("request ID = %q, want r1", rid)
	}
}
