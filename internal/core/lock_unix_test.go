//go:build linux || darwin

package core

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
)

func TestPOSIXLocksSerializeAndRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "store.lock")
	release, err := lockFile(path, true)
	if err != nil {
		t.Fatal(err)
	}
	other, ok, err := tryLockFile(path, true)
	if other != nil {
		other()
	}
	if err != nil || ok {
		release()
		t.Fatalf("exclusive lock was bypassed: %v %v", ok, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := lockFileContext(ctx, path, true); !errors.Is(err, context.Canceled) {
		release()
		t.Fatal(err)
	}
	release()
	other, ok, err = tryLockFile(path, true)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	other()
}
