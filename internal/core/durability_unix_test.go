//go:build linux || darwin

package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestPOSIXCommitReportsUnconfirmedFlush(t *testing.T) {
	store := Store{Home: t.TempDir()}
	sentinel := errors.New("injected device flush failure")
	before := syncReplacement
	// Fail only the hot ledger's post-rename flush; the graveyard snapshot is
	// published (and confirmed) before the hot commit and must still succeed.
	syncReplacement = func(f *os.File) error {
		if filepath.Base(f.Name()) == "store.json.tmp" {
			return sentinel
		}
		return before(f)
	}
	t.Cleanup(func() { syncReplacement = before })
	err := store.Update(func(st *State) error { st.NextID = 19; return nil })
	var uncertain *unconfirmedCommit
	if !errors.As(err, &uncertain) || !errors.Is(err, sentinel) {
		t.Fatalf("flush failure reported as success: %v", err)
	}
	got, err := store.Inspect()
	if err != nil || got.NextID != 19 {
		t.Fatal("new ledger lost after rename", got, err)
	}
}
