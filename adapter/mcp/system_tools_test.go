package mcp

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcpSDK "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestSysToolErrorPath verifies sysTool's handler-error path: when the handler
// returns an error, the tool result is flagged IsError (the ping/status handlers
// never fail, so this branch is only reachable for a sysTool registered with an
// erroring handler).
func TestSysToolErrorPath(t *testing.T) {
	s := mcpSDK.NewServer(&mcpSDK.Implementation{Name: "t", Version: "v"}, nil)
	sysTool(s, "sys_test", "T", "d", "", toolLogger{log: nopLogger{}},
		func(context.Context, emptyIn) (any, error) { return nil, errors.New("boom") })
	sess := connectTestServer(t, s)

	res, err := sess.CallTool(context.Background(), &mcpSDK.CallToolParams{Name: "sys_test"})
	require.NoError(t, err)
	assert.True(t, res.IsError, "a sysTool handler error must flag IsError")
}
