package port

import "context"

// TokenSource is the source of the Authorization header value sent to the
// Proxmox VE / PBS REST APIs.
//
// Local mode: adapter/token.StaticTokenSource returns
// "PVEAPIToken=<user@realm!tokenid=uuid>" verbatim from the configured token.
//
// Future remote/OAuth2 mode: an OAuth2TokenSource (or a per-request header
// relay) returns "Bearer <token>". The gateways consume only the returned
// header string, never the token origin, so swapping the source does not touch
// domain or application. See SPEC.md §2.4 / §4.2.
type TokenSource interface {
	// AuthorizationHeader returns a ready-to-send Authorization header value.
	// The token value is never logged.
	AuthorizationHeader(ctx context.Context) (string, error)
}

// tokenCtxKey is a private key for storing a token/header in the context.
type tokenCtxKey struct{}

// WithToken stores an Authorization header value in the context. It is used by
// a future remote mode (per-request header relay placed by HTTP middleware);
// local mode does not use it.
func WithToken(ctx context.Context, header string) context.Context {
	return context.WithValue(ctx, tokenCtxKey{}, header)
}

// TokenFromContext retrieves the Authorization header value stored by WithToken.
func TokenFromContext(ctx context.Context) (string, bool) {
	t, ok := ctx.Value(tokenCtxKey{}).(string)
	return t, ok
}
