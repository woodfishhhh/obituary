//go:build linux || darwin

package fileio_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core/internal/fileio"
)

func TestPOSIXLocksSerializeAndRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.lock")
	release, err := fileio.LockFile(path, true)
	if err != nil {
		t.Fatal(err)
	}
	other, ok, err := fileio.TryLockFile(path, true)
	if other != nil {
		other()
	}
	if err != nil || ok {
		release()
		t.Fatalf("exclusive lock was bypassed: %v %v", ok, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := fileio.LockFileContext(ctx, path, true); !errors.Is(err, context.Canceled) {
		release()
		t.Fatal(err)
	}
	release()
	other, ok, err = fileio.TryLockFile(path, true)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	other()
}
