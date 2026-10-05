package port

import (
	"context"
	"testing"
)

func TestTokenRoundTrip(t *testing.T) {
	ctx := context.Background()
	if _, ok := TokenFromContext(ctx); ok {
		t.Fatal("expected no token in empty context")
	}

	got := WithToken(ctx, "PVEAPIToken=user@realm!tokenid=secret")
	tok, ok := TokenFromContext(got)
	if !ok || tok != "PVEAPIToken=user@realm!tokenid=secret" {
		t.Fatalf("got token %q, %v; want the header back, true", tok, ok)
	}
}

func TestTokenAndIDsAreIndependent(t *testing.T) {
	ctx := WithSessionID(context.Background(), "s")
	ctx = WithRequestID(ctx, "r")
	ctx = WithToken(ctx, "Bearer tok")

	if sid, _ := SessionIDFromContext(ctx); sid != "s" {
		t.Fatalf("session ID lost: %q", sid)
	}
	if rid, _ := RequestIDFromContext(ctx); rid != "r" {
		t.Fatalf("request ID lost: %q", rid)
	}
	if tok, _ := TokenFromContext(ctx); tok != "Bearer tok" {
		t.Fatalf("token lost: %q", tok)
	}
}
