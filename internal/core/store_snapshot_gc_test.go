package core

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshotCollectionPreservesUnknownFiles(t *testing.T) {
	store := snapshotTestStore(t)
	first, _ := store.ReadSnapshot()
	unknown := "graveyard." + strings.Repeat("A", 26) + ".json"
	content := []byte(`{"owner":"someone-else","version":17}`)
	if err := os.WriteFile(filepath.Join(store.Home, unknown), content, 0600); err != nil {
		t.Fatal(err)
	}
	foreign := snapshotTestStore(t)
	if err := foreign.UpdateGraveyard(func(st *State) error { st.DataID = "other-ledger"; return nil }); err != nil {
		t.Fatal(err)
	}
	foreignState, _ := foreign.ReadSnapshot()
	foreignData, _ := os.ReadFile(filepath.Join(foreign.Home, foreignState.Graveyard.File))
	if err := os.WriteFile(filepath.Join(store.Home, foreignState.Graveyard.File), foreignData, 0600); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if err := store.UpdateGraveyard(func(st *State) error { st.Burials[0].Size++; return nil }); err != nil {
			t.Fatal(err)
		}
		if data, err := os.ReadFile(filepath.Join(store.Home, unknown)); err != nil || !bytes.Equal(data, content) {
			t.Fatal("orphan collector removed unknown file", unknown, err)
		}
	}
	for name, want := range map[string][]byte{unknown: content, foreignState.Graveyard.File: foreignData} {
		data, err := os.ReadFile(filepath.Join(store.Home, name))
		if err != nil || !bytes.Equal(data, want) {
			t.Fatal("orphan collector removed unknown file", name, err)
		}
	}
	st, err := store.ReadSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, ref := range []*GraveyardReference{st.Graveyard, st.PreviousGraveyard} {
		if _, err := os.Stat(filepath.Join(store.Home, ref.File)); err != nil {
			t.Fatal("collector removed current or previous generation", err)
		}
	}
	if _, err := os.Stat(filepath.Join(store.Home, first.Graveyard.File)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("obsolete owned snapshot retained", err)
	}
}

func TestSnapshotCollectionFailureRetainsConsistentCommit(t *testing.T) {
	store := snapshotTestStore(t)
	first, _ := store.ReadSnapshot()
	old := removeGraveyardSnapshot
	removeGraveyardSnapshot = func(*os.Root, string) error { return os.ErrPermission }
	t.Cleanup(func() { removeGraveyardSnapshot = old })
	for range 2 {
		if err := store.UpdateGraveyard(func(st *State) error { st.Burials[0].Size++; return nil }); err != nil {
			t.Fatal("GC failure invalidated commit", err)
		}
	}
	st, err := store.ReadSnapshot()
	if err != nil || st.Burials[0].Size != first.Burials[0].Size+2 {
		t.Fatal("GC failure lost a committed update", err)
	}
	if _, err := os.Stat(filepath.Join(store.Home, first.Graveyard.File)); err != nil {
		t.Fatal("failed GC discarded evidence", err)
	}
	removeGraveyardSnapshot = old
	if err := store.UpdateGraveyard(func(st *State) error { st.Burials[0].Size++; return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(store.Home, first.Graveyard.File)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("owned orphan was not retried", err)
	}
}
