//go:build linux || darwin

package core

import (
	"errors"
	"os"
	"testing"
)

func TestPOSIXCommitReportsUnconfirmedFlush(t *testing.T) {
	store := Store{Home: t.TempDir()}
	sentinel := errors.New("injected device flush failure")
	before := syncReplacement
	syncReplacement = func(*os.File) error { return sentinel }
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
