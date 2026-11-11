//go:build linux || darwin

package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPOSIXLedgerReplacementKeepsOpenSnapshot(t *testing.T) {
	store := Store{Home: t.TempDir()}
	if err := store.Update(func(st *State) error { st.NextID = 3; return nil }); err != nil {
		t.Fatal(err)
	}
	old, err := os.Open(filepath.Join(store.Home, "store.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	if err := store.Update(func(st *State) error { st.NextID = 7; return nil }); err != nil {
		t.Fatal(err)
	}
	got, err := store.ReadSnapshot()
	if err != nil || got.NextID != 7 {
		t.Fatal(got, err)
	}
	oldInfo, err := old.Stat()
	if err != nil {
		t.Fatal(err)
	}
	current, err := os.Stat(filepath.Join(store.Home, "store.json"))
	if err != nil || os.SameFile(oldInfo, current) {
		t.Fatal("ledger replacement reused an open inode", err)
	}
}
