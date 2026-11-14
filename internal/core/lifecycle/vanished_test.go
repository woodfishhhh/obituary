package lifecycle_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

// An object deleted outside Obituary is closed at the next check. One that
// moved, a different object now at its path, and an expired item (the
// sweep's) all stay; nothing on disk is touched.
func TestSettleVanishedClosesOnlyDeletedObjects(t *testing.T) {
	f := newFixture(t)
	gone := f.file("gone.txt")
	f.add(gone, 24*time.Hour)
	moved := f.file("moved.txt")
	f.add(moved, 24*time.Hour)
	replaced := f.file("replaced.txt")
	f.add(replaced, 24*time.Hour)
	due := f.file("due.txt")
	f.add(due, time.Hour)
	for _, path := range []string{gone, replaced, due} {
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Rename(moved, moved+".renamed"); err != nil {
		t.Fatal(err)
	}
	f.file("replaced.txt")
	f.clock = f.clock.Add(2 * time.Hour)
	outcomes, err := f.svc.SettleVanished()
	if runtime.GOOS != "windows" {
		// Without lookup by file ID a missing object is only "not found
		// here", never proven gone, so nothing is closed early.
		if err != nil || len(outcomes) != 0 {
			t.Fatalf("closed an object it cannot prove gone: %+v %v", outcomes, err)
		}
		return
	}
	if err != nil || len(outcomes) != 1 || outcomes[0].Path != gone || outcomes[0].Action != core.ActionVanished {
		t.Fatalf("outcomes %+v %v", outcomes, err)
	}
	var left []string
	for _, it := range f.items() {
		left = append(left, filepath.Base(it.Path))
	}
	if len(left) != 3 {
		t.Fatalf("items left %v, want moved, replaced and due", left)
	}
	if data, err := os.ReadFile(moved + ".renamed"); err != nil || string(data) != "moved.txt" {
		t.Fatalf("moved object touched: %q %v", data, err)
	}
}
