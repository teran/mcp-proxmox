package token

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/port"
)

func TestStaticTokenSource_AuthorizationHeader(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
		werr bool
		pbs  bool
	}{
		{
			name: "valid pve token",
			raw:  "user@pam!tokenid=secret",
			want: "PVEAPIToken=user@pam!tokenid=secret",
		},
		{
			name: "valid pbs token uses pbs scheme and colon separator",
			raw:  "user@pbs!tokenid=secret",
			want: "PBSAPIToken=user@pbs!tokenid:secret",
			pbs:  true,
		},
		{
			name: "pbs token with no '=' stays unchanged",
			raw:  "user@pbs!tokenid:secret",
			want: "PBSAPIToken=user@pbs!tokenid:secret",
			pbs:  true,
		},
		{
			name: "token with leading/trailing spaces is trimmed only for emptiness check",
			raw:  "  user@pam!t=x  ",
			want: "PVEAPIToken=  user@pam!t=x  ",
		},
		{
			name: "empty token errors",
			raw:  "",
			werr: true,
		},
		{
			name: "whitespace-only token errors",
			raw:  "   ",
			werr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s *StaticTokenSource
			if tt.pbs {
				s = NewPBSStaticTokenSource(tt.raw)
			} else {
				s = NewStaticTokenSource(tt.raw)
			}
			got, err := s.AuthorizationHeader(context.Background())
			if tt.werr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

// TestStaticTokenSourceImplementsTokenSource is a compile-time assertion that
// StaticTokenSource satisfies port.TokenSource.
func TestStaticTokenSourceImplementsTokenSource(t *testing.T) {
	var _ port.TokenSource = NewStaticTokenSource("x")
}

// TestStaticTokenSourceContextIgnored verifies the context is not consulted for
// the static (config-sourced) token source.
func TestStaticTokenSourceContextIgnored(t *testing.T) {
	s := NewStaticTokenSource("tok")
	// A context carrying data must not influence the static token source.
	got, err := s.AuthorizationHeader(context.WithValue(context.Background(), ctxKey("k"), "v"))
	require.NoError(t, err)
	assert.Equal(t, "PVEAPIToken=tok", got)
}

// ctxKey is a private context-key type to avoid SA1029 (built-in string keys).
type ctxKey string
