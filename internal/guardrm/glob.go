package guardrm

import (
	"errors"
	"io"
	"os"
	"strings"
	"time"
)

// The scan only avoids denying a preview/error caused by option-shaped names.
// Original and suggested commands receive the same shell-expanded argv, so an
// incomplete scan must keep the denial; only a confirmed dash entry skips it.
func checkLeadingGlob(cwd string) string {
	started := time.Now()
	f, err := os.Open(cwd)
	if err != nil {
		return ""
	}
	defer f.Close()
	return scanLeadingGlob(f.ReadDir, func() time.Duration { return time.Since(started) })
}

// ReadDir is synchronous: check the time budget before and after each batch.
// A stalled filesystem call cannot be interrupted by this elapsed-time check.
func scanLeadingGlob(readDir func(int) ([]os.DirEntry, error), elapsed func() time.Duration) string {
	const maxEntries = 5000
	checked := 0
	for {
		if elapsed() >= 50*time.Millisecond {
			return ""
		}
		entries, err := readDir(min(128, maxEntries-checked+1))
		// A confirmed option-shaped name wins over an incomplete scan, even
		// when this batch exhausted the time or entry budget.
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "-") {
				return "leading glob before -- may expand into options"
			}
		}
		if elapsed() >= 50*time.Millisecond {
			return ""
		}
		if checked+len(entries) > maxEntries {
			return ""
		}
		checked += len(entries)
		if errors.Is(err, io.EOF) {
			return ""
		}
		if err != nil {
			return ""
		}
	}
}
