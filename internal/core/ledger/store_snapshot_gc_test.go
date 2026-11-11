package ledger

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writeSnapshotOrphans(t testing.TB, store Store, st State, count int) []string {
	t.Helper()
	generation := strings.Repeat("B", 26)
	data, err := marshalGraveyard(st, generation)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for i := range count {
		next := strings.Repeat("B", 24) + string(rune('A'+i/26)) + string(rune('A'+i%26))
		name := "graveyard." + next + ".json"
		content := bytes.Replace(data, []byte(generation), []byte(next), 1)
		if err := os.WriteFile(filepath.Join(store.Home, name), content, 0600); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	return names
}

func remainingSnapshotOrphans(t *testing.T, store Store, names []string) []string {
	t.Helper()
	var remaining []string
	for _, name := range names {
		if _, err := os.Lstat(filepath.Join(store.Home, name)); err == nil {
			remaining = append(remaining, name)
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	return remaining
}

func TestSnapshotCollectionBoundsCandidates(t *testing.T) {
	for _, invalid := range []bool{false, true} {
		t.Run(map[bool]string{false: "owned", true: "unrecognized"}[invalid], func(t *testing.T) {
			store := snapshotTestStore(t)
			st, err := store.ReadSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			names := writeSnapshotOrphans(t, store, st, 7)
			if invalid {
				for _, name := range names[:2] {
					if err := os.WriteFile(filepath.Join(store.Home, name), []byte("corrupt"), 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := store.UpdateGraveyard(func(st *State) error { st.Burials[0].Size++; return nil }); err != nil {
				t.Fatal(err)
			}
			want := names[2:]
			if invalid {
				want = names
			}
			if got := remainingSnapshotOrphans(t, store, names); !slices.Equal(got, want) {
				t.Fatalf("snapshot candidate budget exceeded or order changed: got %v, want %v", got, want)
			}
		})
	}
}

func TestSnapshotCollectionProgressAcrossStores(t *testing.T) {
	store := snapshotTestStore(t)
	st, err := store.ReadSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	names := writeSnapshotOrphans(t, store, st, 7)
	for _, name := range names[:2] {
		if err := os.WriteFile(filepath.Join(store.Home, name), []byte("corrupt"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	old := removeGraveyardSnapshot
	removeGraveyardSnapshot = func(root *os.Root, name string) error {
		if name == names[2] {
			return os.ErrPermission
		}
		return old(root, name)
	}
	t.Cleanup(func() { removeGraveyardSnapshot = old })
	for range 16 {
		store = Store{Home: store.Home}
		if err := store.UpdateGraveyard(func(st *State) error { st.Burials[0].Size++; return nil }); err != nil {
			t.Fatal(err)
		}
		current, err := store.ReadSnapshot()
		if err != nil {
			t.Fatal(err)
		}
		if *current.PreviousGraveyard != *st.Graveyard {
			t.Fatal("collection changed predecessor")
		}
		for _, ref := range []*GraveyardReference{current.Graveyard, current.PreviousGraveyard} {
			if _, err := os.Stat(filepath.Join(store.Home, ref.File)); err != nil {
				t.Fatal("collector removed current or previous generation", err)
			}
		}
		st = current
	}
	if got := remainingSnapshotOrphans(t, store, names); !slices.Equal(got, names[:3]) {
		t.Fatal("snapshot collection starved later candidates", got)
	}
	removeGraveyardSnapshot = old
	for range 16 {
		if err := (Store{Home: store.Home}).UpdateGraveyard(func(st *State) error { st.Burials[0].Size++; return nil }); err != nil {
			t.Fatal(err)
		}
	}
	if got := remainingSnapshotOrphans(t, store, names); !slices.Equal(got, names[:2]) {
		t.Fatal("snapshot collection did not retry failed candidate", got)
	}
}

func TestSnapshotCollectionPreservesUnrecognizedEvidence(t *testing.T) {
	for _, kind := range []string{"name", "owner", "version", "data-id", "generation", "content", "truncated", "symlink"} {
		t.Run(kind, func(t *testing.T) {
			store := snapshotTestStore(t)
			st, err := store.ReadSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			name := writeSnapshotOrphans(t, store, st, 1)[0]
			path := filepath.Join(store.Home, name)
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var snap graveyardSnapshot
			if err := json.Unmarshal(data, &snap); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "name":
				path += ".unknown"
			case "owner":
				snap.Owner = "foreign"
			case "version":
				snap.Version++
			case "data-id":
				snap.DataID = "foreign"
			case "generation":
				snap.Generation = strings.Repeat("Z", 26)
			case "content":
				snap.ParentChains = nil
			}
			data, err = json.Marshal(snap)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "truncated" {
				data = data[:len(data)/2]
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if kind == "symlink" {
				target := path + ".target"
				if err := os.Rename(path, target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Skipf("symlink unavailable: %v", err)
				}
			}
			for range 3 {
				if err := store.UpdateGraveyard(func(st *State) error { st.Burials[0].Size++; return nil }); err != nil {
					t.Fatal(err)
				}
			}
			if got, err := os.ReadFile(path); err != nil || !bytes.Equal(got, data) {
				t.Fatal("orphan collector removed unrecognized evidence", kind, err)
			}
		})
	}
}

func TestSnapshotCollectionWaitsForConfirmedCommit(t *testing.T) {
	for _, stage := range []string{"hot-write", "hot-flush", "hot-rename", "hot-confirm"} {
		t.Run(stage, func(t *testing.T) {
			store := snapshotTestStore(t)
			before, err := store.ReadSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			names := writeSnapshotOrphans(t, store, before, 4)
			reset := injectSnapshotFault(t, stage)
			err = store.UpdateGraveyard(func(st *State) error { st.Burials[0].Size++; return nil })
			if err == nil || !strings.Contains(err.Error(), "injected") {
				t.Fatal("commit fault was not reached", err)
			}
			reset()
			if got := remainingSnapshotOrphans(t, store, names); !slices.Equal(got, names) {
				t.Fatal("unconfirmed commit collected orphan evidence", got)
			}
			after, err := store.ReadSnapshot()
			if err != nil || stage != "hot-confirm" && after.GraveyardGCCursor != before.GraveyardGCCursor {
				t.Fatal("failed commit advanced persisted collection cursor", err)
			}
		})
	}
}

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
