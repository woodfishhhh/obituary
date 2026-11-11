package app

import (
	"fmt"
	"os"
	"path/filepath"
)

// explorerStampFile records which ledger version Explorer labels were last
// synchronized against, so a tick can tell a committed-but-unlabelled change
// (made by a hook or another command) from an idle round.
const explorerStampFile = "explorer.stamp"

// ledgerStamp identifies the current ledger file by size and modification
// time; an unchanged update is never committed, so neither moves when idle.
func ledgerStamp(home string) string {
	info, err := os.Stat(filepath.Join(home, "store.json"))
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d %d", info.Size(), info.ModTime().UnixNano())
}

// explorerStale reports whether the ledger changed since the last successful
// Explorer synchronization. Without a ledger there is nothing to label.
func explorerStale(home string) bool {
	current := ledgerStamp(home)
	if current == "" {
		return false
	}
	saved, err := os.ReadFile(filepath.Join(home, explorerStampFile))
	return err != nil || string(saved) != current
}

// writeExplorerStamp is best effort: a missing stamp only causes one more
// synchronization on the next tick.
func writeExplorerStamp(home, stamp string) {
	if stamp == "" {
		return
	}
	_ = os.WriteFile(filepath.Join(home, explorerStampFile), []byte(stamp), 0o600)
}
