package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/teran/mcp-proxmox/domain/port"
)

// newSlogRecord returns a fresh slog.Record with the given level and message.
func newSlogRecord(level slog.Level, msg string) *slog.Record {
	rec := slog.NewRecord(time.Now(), level, msg, 0)
	return &rec
}

func TestParseLevel(t *testing.T) {
	tests := []struct {
		in   string
		want logrus.Level
	}{
		{"trace", logrus.TraceLevel},
		{"debug", logrus.DebugLevel},
		{"info", logrus.InfoLevel},
		{"warn", logrus.WarnLevel},
		{"warning", logrus.WarnLevel},
		{"error", logrus.ErrorLevel},
		{"INFO", logrus.InfoLevel},
		{"  debug  ", logrus.DebugLevel},
		{"unknown", logrus.InfoLevel},
		{"", logrus.InfoLevel},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			assert.Equal(t, tt.want, parseLevel(tt.in))
		})
	}
}

func TestNewLogger(t *testing.T) {
	t.Run("default info text", func(t *testing.T) {
		l := NewLogger("", "")
		assert.Equal(t, logrus.InfoLevel, l.GetLevel())
		// TextFormatter with FullTimestamp.
		f, ok := l.Formatter.(*logrus.TextFormatter)
		require.True(t, ok)
		assert.True(t, f.FullTimestamp)
	})

	t.Run("json format", func(t *testing.T) {
		l := NewLogger("error", "json")
		assert.Equal(t, logrus.ErrorLevel, l.GetLevel())
		_, ok := l.Formatter.(*logrus.JSONFormatter)
		assert.True(t, ok)
	})

	t.Run("text explicit", func(t *testing.T) {
		l := NewLogger("debug", "text")
		assert.Equal(t, logrus.DebugLevel, l.GetLevel())
		_, ok := l.Formatter.(*logrus.TextFormatter)
		assert.True(t, ok)
	})
}

// TestAppLoggerLevels verifies each AppLogger method writes at the right level
// and that the AppLogger also implements RequestAwareLogger.
func TestAppLoggerLevels(t *testing.T) {
	var buf bytes.Buffer
	l := logrus.New()
	l.SetOutput(&buf)
	l.SetFormatter(&logrus.TextFormatter{DisableColors: true})
	l.SetLevel(logrus.DebugLevel)

	app := NewAppLogger(l)
	_, isAware := app.(port.RequestAwareLogger)
	assert.True(t, isAware, "appLogger must implement RequestAwareLogger")

	app.Debugf("dbg")
	app.Infof("inf")
	app.Warnf("wrn")
	app.Errorf("err")
	out := buf.String()

	assert.Contains(t, out, "level=debug")
	assert.Contains(t, out, "level=info")
	assert.Contains(t, out, "level=warning")
	assert.Contains(t, out, "level=error")
}

func TestAppLoggerContextTags(t *testing.T) {
	var buf bytes.Buffer
	l := logrus.New()
	l.SetOutput(&buf)
	l.SetFormatter(&logrus.JSONFormatter{})
	l.SetLevel(logrus.DebugLevel)

	app := NewAppLogger(l)
	aware := app.(port.RequestAwareLogger)

	ctx := port.WithSessionID(context.Background(), "sess-1")
	ctx = port.WithRequestID(ctx, "req-1")
	aware.DebugfContext(ctx, "hello %s", "world")

	out := buf.String()
	assert.Contains(t, out, `"session_id":"sess-1"`)
	assert.Contains(t, out, `"request_id":"req-1"`)
	assert.Contains(t, out, `"msg":"hello world"`)
}

// TestAppLoggerContextAllLevels verifies every ctx-aware level method (Debug,
// Info, Warn, Error) writes at the right level and carries the session/request
// tags.
func TestAppLoggerContextAllLevels(t *testing.T) {
	var buf bytes.Buffer
	l := logrus.New()
	l.SetOutput(&buf)
	l.SetFormatter(&logrus.TextFormatter{DisableColors: true})
	l.SetLevel(logrus.DebugLevel)

	app := NewAppLogger(l)
	aware := app.(port.RequestAwareLogger)
	ctx := port.WithSessionID(context.Background(), "sess")
	ctx = port.WithRequestID(ctx, "req")

	aware.DebugfContext(ctx, "d")
	aware.InfofContext(ctx, "i")
	aware.WarnfContext(ctx, "w")
	aware.ErrorfContext(ctx, "e")

	out := buf.String()
	assert.Contains(t, out, "level=debug")
	assert.Contains(t, out, "level=info")
	assert.Contains(t, out, "level=warning")
	assert.Contains(t, out, "level=error")
	assert.Contains(t, out, "session_id=sess")
	assert.Contains(t, out, "request_id=req")
}

// TestWithSessionTags verifies WithSession adds session_id/request_id fields
// only when present in the context.
func TestWithSessionTags(t *testing.T) {
	var buf bytes.Buffer
	l := logrus.New()
	l.SetOutput(&buf)
	l.SetFormatter(&logrus.JSONFormatter{})
	l.SetLevel(logrus.InfoLevel)

	t.Run("no ids", func(t *testing.T) {
		buf.Reset()
		WithSession(context.Background(), l).Info("plain")
		out := buf.String()
		assert.NotContains(t, out, "session_id")
		assert.NotContains(t, out, "request_id")
	})

	t.Run("empty ids not added", func(t *testing.T) {
		buf.Reset()
		ctx := port.WithSessionID(context.Background(), "")
		ctx = port.WithRequestID(ctx, "")
		WithSession(ctx, l).Info("empty")
		out := buf.String()
		assert.NotContains(t, out, `"session_id":""`)
		assert.NotContains(t, out, `"request_id":""`)
	})

	t.Run("both ids", func(t *testing.T) {
		buf.Reset()
		ctx := port.WithSessionID(context.Background(), "s")
		ctx = port.WithRequestID(ctx, "r")
		WithSession(ctx, l).Info("tagged")
		out := buf.String()
		assert.Contains(t, out, `"session_id":"s"`)
		assert.Contains(t, out, `"request_id":"r"`)
	})
}

// TestLogrusHandler verifies the slog->logrus adapter writes messages, attrs,
// groups and respects Enabled.
func TestLogrusHandler(t *testing.T) {
	var buf bytes.Buffer
	l := logrus.New()
	l.SetOutput(&buf)
	l.SetFormatter(&logrus.JSONFormatter{})
	l.SetLevel(logrus.InfoLevel)

	h := NewLogrusHandler(l)

	t.Run("enabled threshold", func(t *testing.T) {
		// info level: debug should be disabled, info enabled.
		assert.False(t, h.Enabled(context.Background(), -4)) // Debug
		assert.True(t, h.Enabled(context.Background(), 0))   // Info
		assert.True(t, h.Enabled(context.Background(), 4))   // Warn
	})

	t.Run("handle writes message and attrs", func(t *testing.T) {
		buf.Reset()
		rec := newSlogRecord(0, "sdk log")
		rec.Add("k", "v")
		require.NoError(t, h.Handle(context.Background(), *rec))
		out := buf.String()
		assert.Contains(t, out, `"msg":"sdk log"`)
		assert.Contains(t, out, `"k":"v"`)
	})

	t.Run("with attrs", func(t *testing.T) {
		buf.Reset()
		h2 := h.WithAttrs([]slog.Attr{slog.String("a", "1")})
		rec := newSlogRecord(0, "with attrs")
		require.NoError(t, h2.Handle(context.Background(), *rec))
		out := buf.String()
		assert.Contains(t, out, `"a":"1"`)
	})

	t.Run("with group", func(t *testing.T) {
		buf.Reset()
		h2 := h.WithGroup("g")
		rec := newSlogRecord(0, "grouped")
		rec.Add("b", "2")
		require.NoError(t, h2.Handle(context.Background(), *rec))
		out := buf.String()
		assert.Contains(t, out, `"b":"2"`)
	})

	t.Run("with group and pre-existing attrs prefixes keys", func(t *testing.T) {
		buf.Reset()
		h2 := h.WithAttrs([]slog.Attr{slog.String("k", "v")}).WithGroup("g")
		rec := newSlogRecord(0, "grouped attrs")
		require.NoError(t, h2.Handle(context.Background(), *rec))
		out := buf.String()
		assert.Contains(t, out, `"g.k":"v"`)
	})

	t.Run("empty group returns same handler", func(t *testing.T) {
		assert.Same(t, h, h.WithGroup(""))
	})

	t.Run("slogToLogrus mapping", func(t *testing.T) {
		assert.Equal(t, logrus.DebugLevel, slogToLogrus(-8))
		assert.Equal(t, logrus.DebugLevel, slogToLogrus(-4))
		assert.Equal(t, logrus.InfoLevel, slogToLogrus(0))
		assert.Equal(t, logrus.WarnLevel, slogToLogrus(4))
		assert.Equal(t, logrus.ErrorLevel, slogToLogrus(8))
	})
}

func TestNewSlogLogger(t *testing.T) {
	var buf bytes.Buffer
	l := logrus.New()
	l.SetOutput(&buf)
	l.SetFormatter(&logrus.JSONFormatter{})
	l.SetLevel(logrus.InfoLevel)

	slogLogger := NewSlogLogger(l)
	slogLogger.Info("via slog")
	out := buf.String()
	assert.Contains(t, out, `"msg":"via slog"`)
	assert.True(t, strings.Contains(out, "slog"), "expected output via slog")
}
