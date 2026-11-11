package fileio

import "os"

// SyncFile is the single durability boundary for ledger, manifest and audit
// writes. Production always starts with the real filesystem flush; only test
// setup replaces it, without consulting linker flags or environment variables.
var SyncFile = (*os.File).Sync

// SetFileSyncForTesting replaces the flush operation until the returned restore
// function is called. Only test setup may call it, before running parallel tests.
// Core's durability tests restore the real operation explicitly and run serially.
func SetFileSyncForTesting(fn func(*os.File) error) func() {
	previous := SyncFile
	SyncFile = fn
	return func() { SyncFile = previous }
}

func syncFile(file *os.File) error { return SyncFile(file) }

func SyncParentDirectory(root *os.Root) error { return syncParentDirectory(root) }
