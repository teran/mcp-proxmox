// Package token implements port.TokenSource for mcp-proxmox.
//
// Local mode: StaticTokenSource returns the configured API token as the
// Authorization header value. PVE and PBS use different schemes and value
// formats:
//
//	PVE: PVEAPIToken=user@realm!tokenid=secret   (equals separator)
//	PBS: PBSAPIToken=user@realm!tokenid:secret   (colon separator)
//
// The config stores both in the PVE-style "user@realm!tokenid=secret" form;
// the PBS source converts the separator to ':' to match PBS's documented
// TOKENID:TOKENSECRET header value. A future remote/OAuth2 mode will add an
// OAuth2TokenSource (or a per-request header relay) that returns
// "Bearer <token>"; see SPEC.md §2.4 / §4.2.
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
	sep    string // separator between tokenid and secret ('=' for PVE, ':' for PBS)
	token  string
}

// NewStaticTokenSource creates a PVE StaticTokenSource (scheme "PVEAPIToken",
// '=' separator). The token value is stored and never logged.
func NewStaticTokenSource(rawToken string) *StaticTokenSource {
	return &StaticTokenSource{scheme: pveScheme, sep: "=", token: rawToken}
}

// NewPBSStaticTokenSource creates a PBS StaticTokenSource (scheme "PBSAPIToken",
// ':' separator per PBS docs). The token value is stored and never logged.
func NewPBSStaticTokenSource(rawToken string) *StaticTokenSource {
	return &StaticTokenSource{scheme: pbsScheme, sep: ":", token: rawToken}
}

// AuthorizationHeader returns the ready-to-send Authorization header value.
// The configured token is in the PVE-style "user@realm!tokenid=secret" form;
// for PBS the '=' separator is rewritten to ':' so the header matches PBS's
// documented TOKENID:TOKENSECRET value, e.g.
//
//	PVE: PVEAPIToken=user@realm!tokenid=secret
//	PBS: PBSAPIToken=user@realm!tokenid:secret
func (s *StaticTokenSource) AuthorizationHeader(ctx context.Context) (string, error) {
	if strings.TrimSpace(s.token) == "" {
		return "", errors.New("token: empty API token")
	}
	value := s.token
	if i := strings.LastIndex(value, "="); i >= 0 {
		value = value[:i] + s.sep + value[i+1:]
	}
	return s.scheme + "=" + value, nil
}

var _ port.TokenSource = (*StaticTokenSource)(nil)
