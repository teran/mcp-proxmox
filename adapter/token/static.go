// Package token implements port.TokenSource for mcp-proxmox.
//
// Local mode: StaticTokenSource returns the configured API token verbatim as
// the Authorization header value ("PVEAPIToken=user@realm!tokenid=uuid").
// A future remote/OAuth2 mode will add an OAuth2TokenSource (or a per-request
// header relay) that returns "Bearer <token>"; see SPEC.md §2.4 / §4.2.
package token

import (
	"context"
	"errors"
	"strings"

	"github.com/teran/mcp-proxmox/domain/port"
)

// scheme returns the Authorization scheme for the backend. PVE and PBS both use
// the "PVEAPIToken" scheme for API-token authentication.
const scheme = "PVEAPIToken"

// StaticTokenSource is a port.TokenSource that returns a fixed API token as the
// Authorization header value. It is created once per backend at startup.
type StaticTokenSource struct {
	token string
}

// NewStaticTokenSource creates a StaticTokenSource for the given raw API token.
// The token value is stored and never logged.
func NewStaticTokenSource(rawToken string) *StaticTokenSource {
	return &StaticTokenSource{token: rawToken}
}

// AuthorizationHeader returns the ready-to-send Authorization header value:
// "PVEAPIToken=<raw token>".
func (s *StaticTokenSource) AuthorizationHeader(ctx context.Context) (string, error) {
	if strings.TrimSpace(s.token) == "" {
		return "", errors.New("token: empty API token")
	}
	return scheme + "=" + s.token, nil
}

var _ port.TokenSource = (*StaticTokenSource)(nil)
