package testenv

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

// ReadyDeadline keeps process startup outside the measured timeout. The standard
// context timer cannot be reset, so cancellation carries DeadlineExceeded once
// the ready file is complete; the advertised deadline bounds setup separately.
func ReadyDeadline(t testing.TB, path string, timeout time.Duration) (context.Context, <-chan time.Time) {
	t.Helper()
	ctx, cancel := context.WithCancelCause(t.Context())
	started := make(chan time.Time, 1)
	done := make(chan struct{})
	t.Cleanup(func() { cancel(context.Canceled); <-done })
	deadline := time.Now().Add(30 * time.Second)
	go func() {
		defer close(done)
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			data, err := os.ReadFile(path)
			if err == nil && len(data) > 0 {
				break
			}
			if err != nil && !errors.Is(err, os.ErrNotExist) || time.Now().After(deadline) {
				t.Errorf("scenario not ready: %s: %v", path, err)
				cancel(context.Canceled)
				return
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
		started <- time.Now()
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		select {
		case <-ctx.Done():
		case <-timer.C:
			cancel(context.DeadlineExceeded)
		}
	}()
	return readyDeadlineContext{Context: ctx, deadline: deadline}, started
}

type readyDeadlineContext struct {
	context.Context
	deadline time.Time
}

func (c readyDeadlineContext) Deadline() (time.Time, bool) { return c.deadline, true }
func (c readyDeadlineContext) Err() error                  { return context.Cause(c.Context) }

// Hide the inner cancel context so derived contexts propagate our Err value.
func (c readyDeadlineContext) Value(key any) any {
	return context.WithoutCancel(c.Context).Value(key)
}

// ReadyElapsed fails explicitly when a call returns before its fixture is ready.
func ReadyElapsed(t testing.TB, started <-chan time.Time) time.Duration {
	t.Helper()
	select {
	case start := <-started:
		return time.Since(start)
	default:
		t.Fatal("scenario not ready: operation returned before the ready file was complete")
		return 0
	}
}
