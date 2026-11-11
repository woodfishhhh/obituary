//go:build windows

package core_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/woodfishhhh/obituary/internal/core"
)

type restoreRollbackWarningFS struct{ core.FS }

type restoreRollbackWarningParent struct{ core.RestoreParent }

func (f restoreRollbackWarningFS) MkdirRestoreParent(parent core.RestoreParent, name string) (core.RestoreParent, error) {
	if wrapped, ok := parent.(restoreRollbackWarningParent); ok {
		parent = wrapped.RestoreParent
	}
	p, err := f.FS.MkdirRestoreParent(parent, name)
	if err != nil {
		return nil, err
	}
	return restoreRollbackWarningParent{p}, nil
}

func (p restoreRollbackWarningParent) Rollback() error {
	return errors.Join(p.RestoreParent.Rollback(), errors.New("rollback diagnostic"))
}

func TestRestoreMoveRollsBackCreatedParents(t *testing.T) {
	for _, failure := range []string{"move", "intent-write", "rollback-warning"} {
		t.Run(failure, func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "missing/child/payload.txt")
			for _, path := range []string{filepath.Dir(b.Path), filepath.Dir(filepath.Dir(b.Path))} {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			f.fs.fail = errors.New("restore move busy")
			if failure == "intent-write" {
				if err := os.Mkdir(filepath.Dir(b.Grave)+".json.tmp", 0700); err != nil {
					t.Fatal(err)
				}
			}
			if failure == "rollback-warning" {
				f.svc.FS = restoreRollbackWarningFS{f.fs}
			}
			rows, err := f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
			if err != nil || len(rows) != 1 || rows[0].Error == "" || !exists(b.Grave) || exists(filepath.Join(f.dir, "missing")) {
				t.Fatalf("failed move retained created parents or lost error: %+v %v", rows, err)
			}
			if failure == "rollback-warning" && !strings.Contains(rows[0].Warning, "rollback diagnostic") {
				t.Fatalf("rollback warning lost: %+v", rows)
			}
		})
	}
}

func TestRestoreCollisionAttemptsAreBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("writes the full collision retry sequence; move errors have fast coverage")
	}
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	fs := &collisionCountFS{FS: f.fs, remaining: 1002}
	f.svc.FS = fs
	rows, err := f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
	if err != nil || len(rows) != 1 || rows[0].Error == "" || fs.remaining != 1 || !exists(b.Grave) || exists(b.Path) || len(f.burials()) != 1 {
		t.Fatalf("collision retry limit changed: remaining=%d %+v %v", fs.remaining, rows, err)
	}
}

func TestResurrectLongCollisionName(t *testing.T) {
	for _, name := range []string{strings.Repeat("a", 243) + ".txt", strings.Repeat("😀", 121) + ".txt"} {
		t.Run(fmt.Sprint(len(name)), func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, name)
			if err := os.WriteFile(b.Path, []byte("occupied"), 0600); err != nil {
				t.Fatal(err)
			}
			rows, err := f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
			if err != nil || core.GraveFailed(rows) || len(rows) != 1 || rows[0].Action != core.ActionResurrected {
				t.Fatalf("long collision did not restore: %+v %v", rows, err)
			}
			base := filepath.Base(rows[0].Path)
			if !utf8.ValidString(base) || len(utf16.Encode([]rune(base))) > 255 || !strings.HasSuffix(base, " (resurrected).txt") || read(t, rows[0].Path) != name || read(t, b.Path) != "occupied" {
				t.Fatalf("invalid or destructive collision name: %q", base)
			}
		})
	}
}

type collisionCountFS struct {
	core.FS
	remaining int
}

func (f *collisionCountFS) MovePinnedInto(path string, want core.Identity, parent core.RestoreParent, name string) (string, error) {
	if f.remaining > 0 {
		f.remaining--
		return "", os.ErrExist
	}
	return f.FS.MovePinnedInto(path, want, parent, name)
}

func TestResurrectLongExtension(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, n := range []int{1, 10, 100} {
		for _, ext := range []string{strings.Repeat("e", 250), strings.Repeat("😀", 125)} {
			t.Run(fmt.Sprintf("%d/%d", n, len(ext)), func(t *testing.T) {
				f := newFixture(t)
				b := buryOne(t, f, "a."+ext)
				if err := os.WriteFile(b.Path, []byte("occupied"), 0600); err != nil {
					t.Fatal(err)
				}
				// The first collision is real. Later collisions model concurrent
				// occupants; the successful rename still uses a real pinned handle.
				if n > 1 {
					f.svc.FS = &collisionCountFS{FS: f.fs, remaining: n}
				}
				rows, err := f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
				if err != nil || len(rows) != 1 || core.GraveFailed(rows) || rows[0].Action != core.ActionResurrected {
					t.Fatalf("long extension exhausted suffix budget: %+v %v", rows, err)
				}
				name := filepath.Base(rows[0].Path)
				suffix := " (resurrected)"
				if n > 1 {
					suffix = fmt.Sprintf(" (resurrected %d)", n)
				}
				if !utf8.ValidString(name) || len(utf16.Encode([]rune(name))) > 255 || !strings.Contains(name, suffix) || read(t, rows[0].Path) != "a."+ext || read(t, b.Path) != "occupied" {
					t.Fatalf("invalid or destructive extension collision: %q", name)
				}
			})
		}
	}
}

type unavailableRestorePathFS struct{ core.FS }

func (fs unavailableRestorePathFS) MovePinnedInto(path string, want core.Identity, parent core.RestoreParent, name string) (string, error) {
	_, err := fs.FS.MovePinnedInto(path, want, parent, name)
	return "", err // successful move, but the final name could not be verified
}

func TestResurrectAuditsUnavailableRestoredPath(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "output/payload.txt")
	f.svc.FS = unavailableRestorePathFS{f.fs}
	got, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || core.GraveFailed(got) || got[0].Path != b.Path || read(t, b.Path) != "output/payload.txt" || len(f.burials()) != 0 {
		t.Fatalf("unavailable receipt path retried a committed move: %+v %v", got, err)
	}
	events, err := f.svc.Store.History(1)
	if err != nil || len(events) != 1 || events[0].Path != b.Path || !strings.Contains(events[0].Detail, "actual restored path unavailable; expected path used") {
		t.Fatalf("restore path fallback was not audited: %+v %v", events, err)
	}
}

func TestResurrectNeverOverwrites(t *testing.T) {
	f := newFixture(t)
	path := f.file("notes.txt")
	f.add(path, time.Hour)
	f.sweepAt(time.Hour)
	if err := os.WriteFile(path, []byte("new work"), 0o600); err != nil {
		t.Fatal(err)
	}

	results, err := f.svc.Resurrect([]string{path})
	if err != nil || results[0].Error != "" {
		t.Fatalf("resurrect: %v %+v", err, results)
	}
	want := filepath.Join(f.dir, "notes (resurrected).txt")
	if read(t, path) != "new work" || !samePath(results[0].Path, want) || read(t, want) != "notes.txt" {
		t.Fatalf("want new work kept and the old file at %s, got %+v", want, results[0])
	}
}

func TestResurrectProtectsCollisionSuffix(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "notes.txt")
	if err := os.WriteFile(b.Path, []byte("new work"), 0o600); err != nil {
		t.Fatal(err)
	}
	protected := filepath.Join(f.dir, "notes (resurrected).txt")
	f.svc.Guard.Keep = append(f.svc.Guard.Keep, protected)
	got, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || !core.GraveFailed(got) || exists(protected) || !exists(b.Grave) || read(t, b.Path) != "new work" {
		t.Fatalf("restore suffix bypassed protection: %+v %v", got, err)
	}
}
