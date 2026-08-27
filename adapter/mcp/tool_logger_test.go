package mcp

import (
	"bytes"
	"context"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/port"
)

// captureLogger records plain AppLogger lines.
type captureLogger struct {
	lines []string
}

func (c *captureLogger) Debugf(f string, a ...any) { c.lines = append(c.lines, "debug") }
func (c *captureLogger) Infof(f string, a ...any)  { c.lines = append(c.lines, "info") }
func (c *captureLogger) Warnf(f string, a ...any)  { c.lines = append(c.lines, "warn") }
func (c *captureLogger) Errorf(f string, a ...any) { c.lines = append(c.lines, "error") }

func TestToolLoggerFallsBackToPlain(t *testing.T) {
	c := &captureLogger{}
	tl := toolLogger{log: c, l: nil} // no logrus logger -> fallback to plain

	ctx := context.Background()
	tl.Debugf(ctx, "d")
	tl.Infof(ctx, "i")
	tl.Warnf(ctx, "w")
	tl.Errorf(ctx, "e")

	assert.Equal(t, []string{"debug", "info", "warn", "error"}, c.lines)
}

func TestToolLoggerUsesLogrusEntry(t *testing.T) {
	var buf bytes.Buffer
	l := logrus.New()
	l.SetOutput(&buf)
	l.SetFormatter(&logrus.JSONFormatter{})
	l.SetLevel(logrus.DebugLevel)

	tl := toolLogger{log: &captureLogger{}, l: l}
	ctx := port.WithSessionID(context.Background(), "sess")
	ctx = port.WithRequestID(ctx, "req")

	tl.Infof(ctx, "hello %d", 1)
	out := buf.String()
	assert.Contains(t, out, `"msg":"hello 1"`)
	assert.Contains(t, out, `"session_id":"sess"`)
	assert.Contains(t, out, `"request_id":"req"`)
}

// TestToolLoggerAllLevelsWithLogrus covers the logrus-entry path for every
// level (Debugf/Infof/Warnf/Errorf), not just Infof.
func TestToolLoggerAllLevelsWithLogrus(t *testing.T) {
	var buf bytes.Buffer
	l := logrus.New()
	l.SetOutput(&buf)
	l.SetFormatter(&logrus.TextFormatter{DisableColors: true})
	l.SetLevel(logrus.DebugLevel)

	tl := toolLogger{log: &captureLogger{}, l: l}
	ctx := context.Background()
	tl.Debugf(ctx, "d")
	tl.Infof(ctx, "i")
	tl.Warnf(ctx, "w")
	tl.Errorf(ctx, "e")

	out := buf.String()
	assert.Contains(t, out, "level=debug")
	assert.Contains(t, out, "level=info")
	assert.Contains(t, out, "level=warning")
	assert.Contains(t, out, "level=error")
}

func TestToolLoggerEntryNilWhenNoLogrus(t *testing.T) {
	tl := toolLogger{log: &captureLogger{}, l: nil}
	require.Nil(t, tl.entry(context.Background()))
}
