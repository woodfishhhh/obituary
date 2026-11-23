//go:build windows

package app

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/i18n"
)

// explorerStampFile records which ledger version Explorer labels were last
// synchronized against, so a tick can tell a committed-but-unlabelled change
// (made by a hook or another command) from an idle round.
const explorerStampFile = "explorer.stamp"

// Pre-language stamps have no language suffix, so old labels refresh once.
func explorerStamp(home string, lang i18n.Language) string {
	info, err := os.Stat(filepath.Join(home, "store.json"))
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d %d %s", info.Size(), info.ModTime().UnixNano(), lang)
}

// explorerStale reports whether the ledger or language changed since the last successful
// Explorer synchronization. Without a ledger there is nothing to label.
func explorerStale(home, setting string) bool {
	current := explorerStamp(home, i18n.Resolve(setting, i18n.SystemLanguage))
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
