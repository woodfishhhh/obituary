package app

import (
	"os"
	"path/filepath"
	"runtime/debug"
	"testing"
)

func TestMemoryLimitFollowsTheLedger(t *testing.T) {
	for size, want := range map[int64]int64{0: 32 << 20, 400 << 10: 32 << 20, 4 << 20: 256 << 20, 64 << 20: 512 << 20} {
		if got := memoryLimitFor(size); got != want {
			t.Errorf("store.json of %d bytes: limit %d, want %d", size, got, want)
		}
	}
}

func TestFitMemoryLimitAppliesAndSettles(t *testing.T) {
	old := debug.SetMemoryLimit(-1)
	t.Cleanup(func() { debug.SetMemoryLimit(old); memoryLimit.Store(0) })
	memoryLimit.Store(0)
	home := t.TempDir()
	fitMemoryLimit(home) // no ledger yet
	if got := debug.SetMemoryLimit(-1); got != minMemoryLimit {
		t.Fatalf("an empty home gets the floor: %d", got)
	}
	if err := os.WriteFile(filepath.Join(home, "store.json"), make([]byte, 4<<20), 0o600); err != nil {
		t.Fatal(err)
	}
	fitMemoryLimit(home)
	if got := debug.SetMemoryLimit(-1); got != 256<<20 {
		t.Fatalf("a 4 MiB ledger raises the limit: %d", got)
	}
	// Within a quarter of the applied value nothing is changed again.
	debug.SetMemoryLimit(123 << 20)
	if err := os.WriteFile(filepath.Join(home, "store.json"), make([]byte, 4<<20+(100<<10)), 0o600); err != nil {
		t.Fatal(err)
	}
	fitMemoryLimit(home)
	if got := debug.SetMemoryLimit(-1); got != 123<<20 {
		t.Fatalf("a small change must not touch the runtime: %d", got)
	}
}
