package core

import "os"

// syncFile is the single durability boundary for ledger, manifest and audit
// writes. Production always starts with the real filesystem flush; only test
// setup replaces it, without consulting linker flags or environment variables.
var syncFile = (*os.File).Sync

// SetFileSyncForTesting replaces the flush operation until the returned restore
// function is called. Only test setup may call it, before running parallel tests.
// Core's durability tests restore the real operation explicitly and run serially.
func SetFileSyncForTesting(fn func(*os.File) error) func() {
	previous := syncFile
	syncFile = fn
	return func() { syncFile = previous }
}
