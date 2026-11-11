//go:build linux || darwin

package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core/internal/fileio"
)

func TestPOSIXCommitReportsUnconfirmedFlush(t *testing.T) {
	store := Store{Home: t.TempDir()}
	sentinel := errors.New("injected device flush failure")
	before := fileio.SyncReplacement
	// Fail only the hot ledger's post-rename flush; the graveyard snapshot is
	// published (and confirmed) before the hot commit and must still succeed.
	fileio.SyncReplacement = func(f *os.File) error {
		if filepath.Base(f.Name()) == "store.json.tmp" {
			return sentinel
		}
		return before(f)
	}
	t.Cleanup(func() { fileio.SyncReplacement = before })
	err := store.Update(func(st *State) error { st.NextID = 19; return nil })
	if !fileio.IsCommitUnconfirmed(err) || !errors.Is(err, sentinel) {
		t.Fatalf("flush failure reported as success: %v", err)
	}
	got, err := store.Inspect()
	if err != nil || got.NextID != 19 {
		t.Fatal("new ledger lost after rename", got, err)
	}
}
