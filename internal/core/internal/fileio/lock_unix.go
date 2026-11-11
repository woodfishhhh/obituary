//go:build linux || darwin

package fileio

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

// Kernel flock locks serialize ledger access across processes. Always close
// the descriptor, including when unlocking fails, to release its resources.
func flockFile(path string, exclusive, nonblock bool) (func(), bool, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, err
	}
	flags := unix.LOCK_SH
	if exclusive {
		flags = unix.LOCK_EX
	}
	if nonblock {
		flags |= unix.LOCK_NB
	}
	for {
		err = unix.Flock(int(f.Fd()), flags)
		if !errors.Is(err, unix.EINTR) {
			break
		}
	}
	if err != nil {
		f.Close()
		if nonblock && errors.Is(err, unix.EWOULDBLOCK) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return func() { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN); f.Close() }, true, nil
}

func lockFile(path string, exclusive bool) (func(), error) {
	release, _, err := flockFile(path, exclusive, false)
	return release, err
}

func LockFile(path string, exclusive bool) (func(), error) { return lockFile(path, exclusive) }

func tryLockFile(path string, exclusive bool) (func(), bool, error) {
	return flockFile(path, exclusive, true)
}

func TryLockExclusive(path string) (func(), bool, error) { return tryLockFile(path, true) }

func lockFileBefore(path string, exclusive bool, deadline time.Time) (func(), error) {
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	return lockFileContext(ctx, path, exclusive)
}

func lockFileContext(ctx context.Context, path string, exclusive bool) (func(), error) {
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		release, ok, err := tryLockFile(path, exclusive)
		if err != nil || ok {
			return release, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

func replaceFile(src, dst string) error {
	root, err := os.OpenRoot(filepath.Dir(src))
	if err != nil {
		return err
	}
	defer root.Close()
	f, err := root.OpenFile(filepath.Base(src), os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	return replaceDurably(root, filepath.Base(src), filepath.Base(dst), f)
}
