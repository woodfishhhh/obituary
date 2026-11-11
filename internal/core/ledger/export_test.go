package ledger

import (
	"os"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core/internal/fileio"
)

func init() { fileio.SyncFile = func(*os.File) error { return nil } }

// RequireFsync is only available in core's test binary. Like SetReplacementSync,
// it is for serial tests: callers and their subtests must not use t.Parallel.
// The core suite is serial, including all recovery and flush-failure tests.
func RequireFsync(t *testing.T) {
	t.Helper()
	previous := fileio.SyncFile
	fileio.SyncFile = (*os.File).Sync
	t.Cleanup(func() { fileio.SyncFile = previous })
}

func SyncFileForTesting(file *os.File) error { return fileio.SyncFile(file) }

func SetReplacementSync(fn func(*os.File) error) func() {
	previous := fileio.SyncReplacement
	fileio.SyncReplacement = fn
	return func() { fileio.SyncReplacement = previous }
}
