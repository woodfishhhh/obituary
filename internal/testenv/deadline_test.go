package testenv_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestReadyDeadline(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ready")
	ctx, started := testenv.ReadyDeadline(t, path, 20*time.Millisecond)
	child, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	if _, ok := ctx.Deadline(); !ok {
		t.Fatal("missing setup deadline")
	}
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(40 * time.Millisecond)
	if ctx.Err() != nil || child.Err() != nil || len(started) != 0 {
		t.Fatal("deadline started before the ready file was complete")
	}
	if err := os.WriteFile(path, []byte("ready"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-child.Done():
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ready deadline did not propagate")
	}
	if !errors.Is(ctx.Err(), context.DeadlineExceeded) || !errors.Is(child.Err(), context.DeadlineExceeded) {
		t.Fatalf("wrong deadline error: parent=%v child=%v", ctx.Err(), child.Err())
	}
	if elapsed := testenv.ReadyElapsed(t, started); elapsed < 20*time.Millisecond {
		t.Fatalf("deadline fired early: %v", elapsed)
	}
}
