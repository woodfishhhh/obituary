//go:build linux || darwin

package core

import (
	"context"

	"github.com/woodfishhhh/obituary/internal/core/internal/fileio"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

func lockFile(path string, exclusive bool) (func(), error) { return fileio.LockFile(path, exclusive) }

func LockFile(path string, exclusive bool) (func(), error) { return ledger.LockFile(path, exclusive) }

func TryLockExclusive(path string) (func(), bool, error) { return ledger.TryLockExclusive(path) }

func tryLockFile(path string, exclusive bool) (func(), bool, error) {
	return fileio.TryLockFile(path, exclusive)
}

func lockFileContext(ctx context.Context, path string, exclusive bool) (func(), error) {
	return fileio.LockFileContext(ctx, path, exclusive)
}

func replaceFile(src, dst string) error { return fileio.ReplaceFile(src, dst) }
