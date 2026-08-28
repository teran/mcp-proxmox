// Package token implements port.TokenSource for mcp-proxmox.
//
// Local mode: StaticTokenSource returns the configured API token verbatim as
// the Authorization header value. PVE uses the "PVEAPIToken" scheme and PBS
// uses the "PBSAPIToken" scheme (e.g. "PVEAPIToken=user@realm!tokenid=uuid").
// A future remote/OAuth2 mode will add an OAuth2TokenSource (or a per-request
// header relay) that returns "Bearer <token>"; see SPEC.md §2.4 / §4.2.
package token

import (
	"context"
	"errors"
	"strings"

	"github.com/teran/mcp-proxmox/domain/port"
)

// Authorization schemes for the two Proxmox products. PVE and PBS each use
// their own scheme name for API-token authentication.
const (
	pveScheme = "PVEAPIToken"
	pbsScheme = "PBSAPIToken"
)

// StaticTokenSource is a port.TokenSource that returns a fixed API token as the
// Authorization header value. It is created once per backend at startup.
type StaticTokenSource struct {
	scheme string
	token  string
}

// NewStaticTokenSource creates a PVE StaticTokenSource (scheme "PVEAPIToken")
// for the given raw API token. The token value is stored and never logged.
func NewStaticTokenSource(rawToken string) *StaticTokenSource {
	return &StaticTokenSource{scheme: pveScheme, token: rawToken}
}

// NewPBSStaticTokenSource creates a PBS StaticTokenSource (scheme "PBSAPIToken")
// for the given raw API token. The token value is stored and never logged.
func NewPBSStaticTokenSource(rawToken string) *StaticTokenSource {
	return &StaticTokenSource{scheme: pbsScheme, token: rawToken}
}

// AuthorizationHeader returns the ready-to-send Authorization header value,
// e.g. "PVEAPIToken=<raw token>" or "PBSAPIToken=<raw token>".
func (s *StaticTokenSource) AuthorizationHeader(ctx context.Context) (string, error) {
	if strings.TrimSpace(s.token) == "" {
		return "", errors.New("token: empty API token")
	}
	return s.scheme + "=" + s.token, nil
}

var _ port.TokenSource = (*StaticTokenSource)(nil)
