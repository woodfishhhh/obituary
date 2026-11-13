//go:build linux || darwin

package winfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/unix"
)

func TestPOSIXRestoreParents(t *testing.T) {
	dir := t.TempDir()
	fs := FS{}
	parent, err := fs.OpenRestoreParent(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	for _, name := range []string{"", ".", "..", "a/b"} {
		if _, err := fs.MkdirRestoreParent(parent, name); !errors.Is(err, corefs.ErrUnsafe) {
			t.Fatalf("unsafe child name accepted: %q %v", name, err)
		}
	}
	child, err := fs.MkdirRestoreParent(parent, "child")
	if err != nil {
		t.Fatal(err)
	}
	defer child.Close()
	if got := posixID(t, filepath.Join(dir, "child")); got != child.Identity() {
		t.Fatalf("child not pinned: %+v", got)
	}
	if err := os.Symlink("child", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.MkdirRestoreParent(parent, "link"); err == nil {
		t.Fatal("restore parent followed a symlink")
	}
	if err := child.Rollback(); err == nil || !strings.Contains(err.Error(), "retained") {
		t.Fatalf("unsafe rollback was not reported: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "child")); err != nil {
		t.Fatalf("rollback deleted a directory by pathname: %v", err)
	}
}

func TestPOSIXFailedRestoreRetainsCreatedParents(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "parent", "child", "source")
	if err := os.MkdirAll(filepath.Dir(source), 0700); err != nil {
		t.Fatal(err)
	}
	makePOSIXPayload(t, source, false, "tracked")
	want := posixID(t, source)
	svc := &core.Service{Store: core.Store{Home: filepath.Join(dir, "data")}, FS: FS{}}
	rows, err := svc.Bury([]string{source}, false, "")
	if err != nil || core.GraveFailed(rows) {
		t.Fatalf("bury: %+v %v", rows, err)
	}
	b := *rows[0].Burial
	oldParent := filepath.Join(dir, "old-parent")
	mustPOSIXRename(t, filepath.Join(dir, "parent"), oldParent)
	svc.FS = FS{rename: func(int, string, int, string) error { return unix.EINVAL }}
	rows, err = svc.Resurrect([]string{source})
	if err != nil || len(rows) != 1 || !core.GraveFailed(rows) {
		t.Fatalf("failed restore: %+v %v", rows, err)
	}
	for _, path := range []string{filepath.Join(dir, "parent"), filepath.Dir(source)} {
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			t.Fatalf("created parent deleted: %s %v", path, err)
		}
		if !strings.Contains(rows[0].Warning, path) {
			t.Fatalf("cleanup warning omits %s: %+v", path, rows[0])
		}
	}
	if _, err := os.Stat(filepath.Join(oldParent, "child")); err != nil {
		t.Fatalf("original parent deleted: %v", err)
	}
	assertPOSIXPayload(t, b.Grave, false, want, "tracked")
	st, err := svc.Store.ReadSnapshot()
	if err != nil || len(st.Burials) != 1 {
		t.Fatalf("failed restore discarded grave: %+v %v", st, err)
	}
}

func TestPOSIXRestoreLongUnicodeCollision(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, strings.Repeat("雪", 82)+".txt")
	makePOSIXPayload(t, source, false, "tracked")
	want := posixID(t, source)
	svc := &core.Service{Store: core.Store{Home: filepath.Join(dir, "data")}, FS: FS{}}
	rows, err := svc.Bury([]string{source}, false, "")
	if err != nil || core.GraveFailed(rows) {
		t.Fatalf("bury: %+v %v", rows, err)
	}
	makePOSIXPayload(t, source, false, "occupied")
	rows, err = svc.Resurrect([]string{source})
	if err != nil || core.GraveFailed(rows) || len(rows) != 1 {
		t.Fatalf("POSIX Unicode collision did not restore: %+v %v", rows, err)
	}
	assertPOSIXPayload(t, rows[0].Path, false, want, "tracked")
	if data, err := os.ReadFile(source); err != nil || string(data) != "occupied" {
		t.Fatalf("collision overwrote existing file: %q %v", data, err)
	}
	if len(filepath.Base(rows[0].Path)) > 255 {
		t.Fatal("restored name exceeds POSIX byte limit")
	}
}

func TestPOSIXFindRestoredChecksBothIdentities(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "renamed")
	makePOSIXPayload(t, path, false, "tracked")
	parent, source := posixID(t, dir), posixID(t, path)
	fs := FS{}
	got, err := fs.FindRestoredIn(dir, parent, source)
	if err != nil || got != path {
		t.Fatalf("restored identity not found: %q %v", got, err)
	}
	other := t.TempDir()
	if _, err := fs.FindRestoredIn(other, parent, source); err == nil {
		t.Fatal("replaced parent accepted")
	}
	mustPOSIXRename(t, path, filepath.Join(other, "moved"))
	makePOSIXPayload(t, path, false, "stranger")
	if _, err := fs.FindRestoredIn(dir, parent, source); err == nil {
		t.Fatal("replaced child accepted")
	}
}

func TestPOSIXRestoreRaceRollback(t *testing.T) {
	for _, blocked := range []bool{false, true} {
		t.Run(fmt.Sprint(blocked), func(t *testing.T) {
			dir := t.TempDir()
			grave := filepath.Join(dir, "grave")
			if err := os.Mkdir(grave, 0700); err != nil {
				t.Fatal(err)
			}
			source, other := filepath.Join(grave, "source"), filepath.Join(dir, "other")
			makePOSIXPayload(t, source, false, "tracked")
			makePOSIXPayload(t, other, false, "stranger")
			want, foreign := posixID(t, source), posixID(t, other)
			fs := FS{beforeRename: func() {
				mustPOSIXRename(t, source, source+"-old")
				mustPOSIXRename(t, other, source)
			}}
			if blocked {
				fs.beforeRollback = func() { makePOSIXPayload(t, source, false, "occupied") }
			}
			parent, err := fs.OpenRestoreParent(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer parent.Close()
			_, err = fs.MovePinnedInto(source, want, parent, "restored")
			var mismatch *corefs.MoveMismatchError
			if !errors.As(err, &mismatch) {
				t.Fatalf("restore race accepted: %v", err)
			}
			retained := source
			if blocked {
				retained = mismatch.Retained
				if filepath.Dir(retained) != grave {
					t.Fatalf("stranger left outside grave: %v", err)
				}
			} else if mismatch.Retained != "" {
				t.Fatalf("rollback failed: %v", err)
			}
			assertPOSIXPayload(t, retained, false, foreign, "stranger")
			assertPOSIXPayload(t, source+"-old", false, want, "tracked")
			if _, err := os.Stat(filepath.Join(dir, "restored")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("failed restore left destination: %v", err)
			}
		})
	}
}

type lostRestoreResultFS struct{ FS }

func (fs lostRestoreResultFS) MovePinnedInto(path string, want corefs.Identity, parent corefs.RestoreParent, name string) (string, error) {
	actual, err := fs.FS.MovePinnedInto(path, want, parent, name)
	if err == nil {
		err = errors.New("move result was interrupted")
	}
	return actual, err
}

func TestPOSIXRestoreRecovery(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "nested", "source")
	if err := os.Mkdir(filepath.Dir(source), 0700); err != nil {
		t.Fatal(err)
	}
	makePOSIXPayload(t, source, false, "tracked")
	want := posixID(t, source)
	svc := &core.Service{Store: core.Store{Home: filepath.Join(dir, "data")}, FS: FS{}}
	rows, err := svc.Bury([]string{source}, false, "")
	if err != nil || core.GraveFailed(rows) {
		t.Fatalf("bury: %+v %v", rows, err)
	}
	mustPOSIXRename(t, filepath.Dir(source), filepath.Join(dir, "old-parent"))
	svc.FS = lostRestoreResultFS{}
	rows, err = svc.Resurrect([]string{source})
	if err != nil || !core.GraveFailed(rows) {
		t.Fatalf("interruption missing: %+v %v", rows, err)
	}
	mustPOSIXRename(t, source, source+"-renamed")
	svc.FS = FS{}
	view, err := svc.ReadGraveyard()
	if err != nil || len(view.Pending) != 0 || len(view.Burials) != 0 {
		t.Fatalf("restore recovery: %+v %v", view, err)
	}
	assertPOSIXPayload(t, source+"-renamed", false, want, "tracked")
}

func TestPOSIXRestoreMismatchNeverPurges(t *testing.T) {
	dir := t.TempDir()
	source, other := filepath.Join(dir, "source"), filepath.Join(dir, "other")
	makePOSIXPayload(t, source, false, "tracked")
	makePOSIXPayload(t, other, false, "stranger")
	want := posixID(t, source)
	now := time.Now()
	svc := &core.Service{Store: core.Store{Home: filepath.Join(dir, "data")}, FS: FS{}, Now: func() time.Time { return now }}
	rows, err := svc.Bury([]string{source}, false, "")
	if err != nil || core.GraveFailed(rows) {
		t.Fatalf("bury: %+v %v", rows, err)
	}
	b := *rows[0].Burial
	svc.FS = FS{beforeRename: func() {
		mustPOSIXRename(t, b.Grave, filepath.Join(dir, "original"))
		mustPOSIXRename(t, other, b.Grave)
	}, beforeRollback: func() { makePOSIXPayload(t, b.Grave, false, "occupied") }}
	rows, err = svc.Resurrect([]string{source})
	if err != nil || !core.GraveFailed(rows) {
		t.Fatalf("restore race: %+v %v", rows, err)
	}
	svc.FS = FS{}
	st, err := svc.Store.ReadSnapshot()
	if err != nil || len(st.Burials) != 1 || st.Burials[0].IdentityMismatch == "" {
		t.Fatalf("restore mismatch not persisted: %+v %v", st, err)
	}
	entries, err := os.ReadDir(filepath.Dir(b.Grave))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		mustPOSIXRename(t, filepath.Join(filepath.Dir(b.Grave), entry.Name()), filepath.Join(dir, "saved-"+entry.Name()))
	}
	mustPOSIXRename(t, filepath.Join(dir, "original"), b.Grave)
	// The ledger veto also survives a missing manifest.
	mustPOSIXRename(t, filepath.Dir(b.Grave)+".json", filepath.Join(dir, "saved-manifest"))
	now = now.Add(30 * 24 * time.Hour)
	if _, err := svc.Purge(true, false); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PurgeRefs([]string{fmt.Sprint(b.ID)}, false); err != nil {
		t.Fatal(err)
	}
	rows, err = svc.Resurrect([]string{fmt.Sprint(b.ID)})
	if err != nil || !core.GraveFailed(rows) {
		t.Fatalf("quarantined restore accepted: %+v %v", rows, err)
	}
	assertPOSIXPayload(t, b.Grave, false, want, "tracked")
}

func TestPOSIXQuarantinedMissingGraveRetainsRecord(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source")
	makePOSIXPayload(t, source, false, "tracked")
	svc := &core.Service{Store: core.Store{Home: filepath.Join(dir, "data")}, FS: FS{}}
	rows, err := svc.Bury([]string{source}, false, "")
	if err != nil || core.GraveFailed(rows) {
		t.Fatalf("bury: %+v %v", rows, err)
	}
	b := *rows[0].Burial
	if err := svc.Store.UpdateGraveyard(func(st *core.State) error {
		st.Burials[0].IdentityMismatch = "identity mismatch; never purge; manually inspect retained content"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	mustPOSIXRename(t, filepath.Dir(b.Grave), filepath.Join(dir, "retained"))
	if _, err := svc.PurgeRefs([]string{fmt.Sprint(b.ID)}, false); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Store.ReadSnapshot()
	if err != nil || len(st.Burials) != 1 {
		t.Fatalf("quarantine record retired: %+v %v", st, err)
	}
}
