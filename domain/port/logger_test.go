package port

import (
	"context"
	"sync"
	"testing"
)

// dummyLogger is a plain AppLogger that records emitted lines. It does NOT
// implement RequestAwareLogger, so it exercises the fallback path.
type dummyLogger struct {
	mu    sync.Mutex
	lines []string
}

func (d *dummyLogger) Tracef(format string, args ...any) { d.add("trace", format, args...) }
func (d *dummyLogger) Debugf(format string, args ...any) { d.add("debug", format, args...) }
func (d *dummyLogger) Infof(format string, args ...any)  { d.add("info", format, args...) }
func (d *dummyLogger) Warnf(format string, args ...any)  { d.add("warn", format, args...) }
func (d *dummyLogger) Errorf(format string, args ...any) { d.add("error", format, args...) }

func (d *dummyLogger) add(level, format string, args ...any) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.lines = append(d.lines, level+":"+sprintf(format, args...))
}

func sprintf(format string, args ...any) string {
	return format // args ignored for the fallback assertions
}

// awareLogger is a RequestAwareLogger that only records the count of context
// calls (it does NOT record lines into dummyLogger), so tests can confirm the
// ctx-aware path is taken rather than the plain fallback.
type awareLogger struct {
	dummyLogger
	ctxCalls int
}

func (a *awareLogger) TracefContext(ctx context.Context, format string, args ...any) { a.ctxCalls++ }
func (a *awareLogger) DebugfContext(ctx context.Context, format string, args ...any) { a.ctxCalls++ }
func (a *awareLogger) InfofContext(ctx context.Context, format string, args ...any)  { a.ctxCalls++ }
func (a *awareLogger) WarnfContext(ctx context.Context, format string, args ...any)  { a.ctxCalls++ }
func (a *awareLogger) ErrorfContext(ctx context.Context, format string, args ...any) { a.ctxCalls++ }

func TestToRequestAwareLogger(t *testing.T) {
	if got := ToRequestAwareLogger(&dummyLogger{}); got != nil {
		t.Fatalf("dummy logger should not be request-aware, got %#v", got)
	}
	a := &awareLogger{}
	if got := ToRequestAwareLogger(a); got != a {
		t.Fatalf("aware logger should be returned, got %#v", got)
	}
	if got := ToRequestAwareLogger(nil); got != nil {
		t.Fatalf("nil logger should yield nil, got %#v", got)
	}
}

func TestCtxLoggerFallsBackToPlain(t *testing.T) {
	d := &dummyLogger{}
	c := NewCtxLogger(d)

	c.Tracef(context.Background(), "trc %d", 9)
	c.Debugf(context.Background(), "dbg %d", 1)
	c.Infof(context.Background(), "inf")
	c.Warnf(context.Background(), "wrn")
	c.Errorf(context.Background(), "err")

	got := d.lines
	want := []string{"trace:trc %d", "debug:dbg %d", "info:inf", "warn:wrn", "error:err"}
	if len(got) != len(want) {
		t.Fatalf("expected %d lines, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestCtxLoggerUsesAwarePath(t *testing.T) {
	a := &awareLogger{}
	c := NewCtxLogger(a)

	c.Tracef(context.Background(), "trc")
	c.Debugf(context.Background(), "dbg")
	c.Infof(context.Background(), "inf")
	c.Warnf(context.Background(), "wrn")
	c.Errorf(context.Background(), "err")

	if a.ctxCalls != 5 {
		t.Fatalf("expected 5 ctx-aware calls, got %d", a.ctxCalls)
	}
	// No plain lines should have been recorded.
	if len(a.lines) != 0 {
		t.Fatalf("expected no plain lines, got %v", a.lines)
	}
}

func TestCtxLoggerNilIsNoop(t *testing.T) {
	var c CtxLogger // zero value: plain and aware are nil
	// Must not panic.
	c.Tracef(context.Background(), "trc")
	c.Debugf(context.Background(), "dbg")
	c.Infof(context.Background(), "inf")
	c.Warnf(context.Background(), "wrn")
	c.Errorf(context.Background(), "err")
}
