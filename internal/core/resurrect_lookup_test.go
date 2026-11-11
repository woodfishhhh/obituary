//go:build windows

package core_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestResurrectPathIgnoresUnrelatedRootFailure(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "local.txt")
	// This is only an offline ledger record and an injected read failure;
	// no directory or mapping is created on the other volume.
	offline := core.Burial{ID: b.ID + 1, Path: `Z:\offline.txt`, BuriedAt: f.clock, PurgeAt: f.clock.Add(time.Hour)}
	root := core.GraveyardRoot(f.svc, offline.Path)
	offline.Grave = filepath.Join(root, fmt.Sprintf("%d-20261107T120000Z-abcdef", offline.ID), "offline.txt")
	if err := f.svc.Store.Update(func(st *core.State) error {
		st.Burials = append(st.Burials, offline)
		st.NextID = offline.ID + 1
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	f.svc.FS = rootFailureFS{testFS: f.fs, root: root}
	rows, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || core.GraveFailed(rows) || len(rows) != 1 || rows[0].Action != core.ActionResurrected || read(t, b.Path) != "local.txt" || len(f.burials()) != 1 || f.burials()[0].ID != offline.ID {
		t.Fatalf("unrelated graveyard root blocked a local path resurrection: %+v %v", rows, err)
	}
}

func TestResurrectBatchContinuesAfterRootFailure(t *testing.T) {
	f := newFixture(t)
	first := buryOne(t, f, "first.txt")
	second := buryOne(t, f, "second.txt")
	f.svc.FS = &scanFailureFS{testFS: f.fs, root: core.GraveyardRoot(f.svc, first.Path)}
	rows, err := f.svc.Resurrect([]string{first.Path, second.Path})
	if err != nil || len(rows) != 2 || !strings.Contains(rows[0].Error, "retry later") || rows[1].Action != core.ActionResurrected || rows[1].Error != "" || !exists(first.Grave) || read(t, second.Path) != "second.txt" {
		t.Fatalf("related root failure aborted later resurrection references: %+v %v", rows, err)
	}
}

func TestCommittedRestoreRetryResolvesAllManifestsTogether(t *testing.T) {
	f := newFixture(t)
	first := buryOne(t, f, "payload.txt")
	_, unlock := interruptResurrection(t, f, first)
	unlock()
	holdManifest(t, filepath.Dir(first.Grave)+".json")
	if _, err := f.svc.ReadGraveyard(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(first.Path, first.Path+"-old"); err != nil {
		t.Fatal(err)
	}
	second := buryOne(t, f, "payload.txt")
	_, unlock = interruptResurrection(t, f, second)
	unlock()
	holdManifest(t, filepath.Dir(second.Grave)+".json")
	if _, err := f.svc.ReadGraveyard(); err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.Resurrect([]string{first.Path})
	if err != nil || core.GraveFailed(rows) || len(rows) != 1 || rows[0].Burial == nil || rows[0].Burial.ID != second.ID || !samePath(rows[0].Path, second.Path) {
		t.Fatalf("committed retry selected an older manifest: %+v %v", rows, err)
	}
}

func TestResurrectDistinguishesUnicodePaths(t *testing.T) {
	f := newFixture(t)
	paths := []string{f.file("k.txt"), f.file("\u212a.txt")}
	first, err := f.svc.FS.Identify(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	second, err := f.svc.FS.Identify(paths[1])
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Skip("filesystem aliases Kelvin sign to ASCII k")
	}
	for _, path := range paths {
		got, err := f.svc.Bury([]string{path}, false, "")
		if err != nil || core.GraveFailed(got) {
			t.Fatalf("bury: %+v %v", got, err)
		}
		f.clock = f.clock.Add(time.Second)
	}
	for _, path := range paths {
		got, err := f.svc.Resurrect([]string{path})
		if err != nil || core.GraveFailed(got) || got[0].Path != path || read(t, path) != filepath.Base(path) {
			t.Fatalf("resurrection selected a different Unicode path: %+v %v", got, err)
		}
	}
}

func TestResurrectUniqueASCIIPath(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "mixed.txt")
	got, err := f.svc.Resurrect([]string{strings.ToUpper(b.Path)})
	if err != nil || core.GraveFailed(got) || read(t, b.Path) != "mixed.txt" {
		t.Fatalf("unique ASCII alias not restored: %+v %v", got, err)
	}
}
