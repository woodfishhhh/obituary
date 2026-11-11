package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
)

// Keep the Win32 lock helpers: Go's filelock is internal to cmd/go, and
// gofrs/flock v0.13.2 leaves its handle open when Unlock fails. Its Close
// only calls Unlock again; there is no public unconditional-close API.
// Both release closures here must still close the file after an unlock error.

// lockFile takes a blocking lock on path and returns its release function.
func lockFile(path string, exclusive bool) (func(), error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	var flags uint32
	if exclusive {
		flags = windows.LOCKFILE_EXCLUSIVE_LOCK
	}
	overlapped := new(windows.Overlapped)
	if err := windows.LockFileEx(windows.Handle(f.Fd()), flags, 0, 1, 0, overlapped); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, overlapped)
		f.Close()
	}, nil
}

// LockFile takes a blocking lock on path (created if needed) and returns its
// release function; other packages use it to serialise their own files.
func LockFile(path string, exclusive bool) (func(), error) { return lockFile(path, exclusive) }

// replaceFile replaces an audit file in the same directory, through one handle.
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

// TryLockExclusive takes a non-blocking exclusive lock; ok is false if another
// process holds it. The returned function releases the lock.
func TryLockExclusive(path string) (release func(), ok bool, err error) {
	return tryLockFile(path, true)
}

func tryLockFile(path string, exclusive bool) (release func(), ok bool, err error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, err
	}
	overlapped := new(windows.Overlapped)
	flags := uint32(windows.LOCKFILE_FAIL_IMMEDIATELY)
	if exclusive {
		flags |= windows.LOCKFILE_EXCLUSIVE_LOCK
	}
	err = windows.LockFileEx(windows.Handle(f.Fd()),
		flags, 0, 1, 0, overlapped)
	if errors.Is(err, windows.ERROR_LOCK_VIOLATION) || errors.Is(err, windows.ERROR_IO_PENDING) {
		f.Close()
		return nil, false, nil
	}
	if err != nil {
		f.Close()
		return nil, false, err
	}
	return func() {
		_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, overlapped)
		f.Close()
	}, true, nil
}

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
