package lifecycle

import (
	"context"

	"github.com/woodfishhhh/obituary/internal/core/internal/fileio"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

func LockFile(path string, exclusive bool) (func(), error) { return ledger.LockFile(path, exclusive) }

func TryLockExclusive(path string) (func(), bool, error) { return ledger.TryLockExclusive(path) }

func lockFileContext(ctx context.Context, path string, exclusive bool) (func(), error) {
	return fileio.LockFileContext(ctx, path, exclusive)
}
