package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core/internal/fileio"
)

func snapshotTestStore(t *testing.T) Store {
	t.Helper()
	store := Store{Home: t.TempDir()}
	st := benchmarkLedger()
	st.Burials, st.Items, st.Proposals = st.Burials[:1], st.Items[:1], nil
	if err := store.save(st); err != nil {
		t.Fatal(err)
	}
	return store
}

func injectSnapshotFault(t *testing.T, stage string) func() {
	t.Helper()
	oldWrite, oldSync, oldReplace, oldRename, oldLink := fileio.WriteStoreBytes, fileio.SyncFile, fileio.SyncReplacement, fileio.RenameStore, publishGraveyardLink
	fault := errors.New("injected " + stage)
	kind := func(f *os.File) string {
		if strings.HasPrefix(filepath.Base(f.Name()), "graveyard.") {
			return "snapshot"
		}
		if filepath.Base(f.Name()) == "store.json.tmp" {
			return "hot"
		}
		return "backup"
	}
	fileio.WriteStoreBytes = func(f *os.File, data []byte) (int, error) {
		if stage == kind(f)+"-write" {
			return 0, fault
		}
		return oldWrite(f, data)
	}
	fileio.SyncFile = func(f *os.File) error {
		if stage == kind(f)+"-flush" {
			return fault
		}
		return oldSync(f)
	}
	fileio.SyncReplacement = func(f *os.File) error {
		if stage == kind(f)+"-confirm" {
			return fault
		}
		return oldReplace(f)
	}
	fileio.RenameStore = func(root *os.Root, from, to string) error {
		if stage == "hot-rename" && to == "store.json" {
			return fault
		}
		return oldRename(root, from, to)
	}
	publishGraveyardLink = func(root *os.Root, from, to string) error {
		if stage == "snapshot-publish" {
			return fault
		}
		return oldLink(root, from, to)
	}
	reset := func() {
		fileio.WriteStoreBytes, fileio.SyncFile, fileio.SyncReplacement, fileio.RenameStore, publishGraveyardLink = oldWrite, oldSync, oldReplace, oldRename, oldLink
	}
	t.Cleanup(reset)
	return reset
}

func TestSnapshotCommitFailureBoundaries(t *testing.T) {
	RequireFsync(t)
	for _, stage := range []string{"snapshot-write", "snapshot-flush", "snapshot-publish", "snapshot-confirm", "hot-write", "hot-flush", "hot-rename", "hot-confirm"} {
		t.Run(stage, func(t *testing.T) {
			store := snapshotTestStore(t)
			before, _ := store.ReadSnapshot()
			original, _ := os.ReadFile(store.statePath())
			reset := injectSnapshotFault(t, stage)
			err := store.UpdateGraveyard(func(st *State) error {
				st.Burials[0].Size++
				st.Stats.BuriedBytes++
				st.queueAudit(Event{Action: "joint"})
				return nil
			})
			if err == nil || !strings.Contains(err.Error(), "injected") {
				t.Fatal("fault was not reached", err)
			}
			reset()
			got, err := store.ReadSnapshot()
			if err != nil {
				t.Fatal("interrupted commit lost consistent state", err)
			}
			if stage == "hot-confirm" {
				if got.Burials[0].Size != before.Burials[0].Size+1 || got.Stats.BuriedBytes != before.Stats.BuriedBytes+1 || len(got.PendingAudit) != 1 {
					t.Fatal("unconfirmed commit mixed generations or lost receipt")
				}
			} else if data, _ := os.ReadFile(store.statePath()); !bytes.Equal(data, original) {
				t.Fatal("failed dependency changed committed hot ledger")
			}
			if err := store.Confirm(func(*State) error { return nil }); err != nil {
				t.Fatal("confirmation did not recover", err)
			}
		})
	}
}

func TestSnapshotConfirmFlushesDependencyBeforeHotLedger(t *testing.T) {
	RequireFsync(t)
	store := snapshotTestStore(t)
	var names []string
	old := fileio.SyncReplacement
	fileio.SyncReplacement = func(f *os.File) error {
		names = append(names, filepath.Base(f.Name()))
		return old(f)
	}
	t.Cleanup(func() { fileio.SyncReplacement = old })
	if err := store.Confirm(func(*State) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || !strings.HasPrefix(names[0], "graveyard.") || names[1] != "store.json.tmp" {
		t.Fatalf("confirmation skipped or reordered snapshot dependency: %v", names)
	}
}

func TestSnapshotRejectsInvalidReference(t *testing.T) {
	for _, mode := range []string{"missing", "hash", "generation", "dataId", "version"} {
		t.Run(mode, func(t *testing.T) {
			store := snapshotTestStore(t)
			st, _ := store.ReadSnapshot()
			path := filepath.Join(store.Home, st.Graveyard.File)
			if mode == "missing" {
				if err := os.Rename(path, path+".held"); err != nil {
					t.Fatal(err)
				}
			} else {
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				var fields map[string]any
				if err := json.Unmarshal(data, &fields); err != nil {
					t.Fatal(err)
				}
				fields[mode] = "wrong"
				if mode == "version" {
					fields[mode] = 16
				}
				data, _ = json.Marshal(fields)
				if err := os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
				if mode != "hash" {
					st.Graveyard.Hash = fmt.Sprintf("%x", sha256.Sum256(data))
					hot, _ := marshalHot(st)
					if err := os.WriteFile(store.statePath(), hot, 0600); err != nil {
						t.Fatal(err)
					}
				}
			}
			for _, read := range []func() (State, error){store.ReadSnapshot, store.Inspect, store.Read} {
				if _, err := read(); err == nil {
					t.Fatal("invalid snapshot accepted as empty graveyard", mode)
				}
			}
		})
	}
}

func TestInspectRetriesRetiredSnapshot(t *testing.T) {
	store := snapshotTestStore(t)
	first, err := store.ReadSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	oldRead := readGraveyardSnapshot
	retired := false
	readGraveyardSnapshot = func(path string) ([]byte, error) {
		if !retired && filepath.Base(path) == first.Graveyard.File {
			retired = true
			for range 2 {
				if err := store.UpdateGraveyard(func(st *State) error { st.Burials[0].Size++; return nil }); err != nil {
					return nil, err
				}
			}
		}
		return oldRead(path)
	}
	t.Cleanup(func() { readGraveyardSnapshot = oldRead })
	st, err := store.Inspect()
	if err != nil || !retired || st.Burials[0].Size != first.Burials[0].Size+2 {
		t.Fatal("unlocked reader mixed or lost a retired generation", err)
	}
}

func TestSnapshotPublicationDoesNotReplaceExistingFile(t *testing.T) {
	store := Store{Home: t.TempDir()}
	name := "graveyard." + strings.Repeat("A", 26) + ".json"
	path := filepath.Join(store.Home, name)
	if err := os.WriteFile(path, []byte("unknown file"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.publishGraveyard(name, []byte("snapshot bytes")); err == nil {
		t.Fatal("snapshot publication replaced an existing file")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "unknown file" {
		t.Fatal("snapshot publication replaced an existing file", err)
	}
}

func TestStoreSeparatesGraveyard(t *testing.T) {
	store := Store{Home: t.TempDir()}
	if err := store.save(benchmarkLedger()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(store.statePath())
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"burials", "restoreCleanups", "parentChains", "operations", "origins", "volumes"} {
		if _, present := fields[field]; present {
			t.Fatalf("hot ledger contains graveyard field %s", field)
		}
	}
}
