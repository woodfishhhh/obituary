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

func TestPOSIXMovesNeverReplace(t *testing.T) {
	dir := t.TempDir()
	source, target := filepath.Join(dir, "source"), filepath.Join(dir, "target")
	for _, path := range []string{source, target} {
		if err := os.WriteFile(path, []byte(path), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	fs := FS{}
	id, err := fs.Identify(source)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := fs.OpenRestoreParent(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	if err := fs.MovePinned(source, id, target); !errors.Is(err, os.ErrExist) {
		t.Fatalf("no-replace move: %v", err)
	}
	if err := fs.Recycle(source, id); !errors.Is(err, corefs.ErrUnsupported) {
		t.Fatal(err)
	}
	if _, err := fs.MovePinnedInto(source, id, parent, "target"); !errors.Is(err, os.ErrExist) {
		t.Fatalf("no-replace restoration: %v", err)
	}
	for _, path := range []string{source, target} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != path {
			t.Fatalf("object changed: %s %q %v", path, data, err)
		}
	}
}

func TestPOSIXBurialAndResurrection(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "payload")
	if err := os.WriteFile(source, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	svc := &core.Service{Store: core.Store{Home: filepath.Join(dir, "data")}, FS: FS{}, Now: func() time.Time { return now }}
	rows, err := svc.Add([]string{source}, core.AddOptions{ExpiresAt: now.Add(time.Hour)})
	if err != nil || core.Failed(rows) {
		t.Fatalf("register: %+v %v", rows, err)
	}
	now = now.Add(2 * time.Hour)
	out, err := svc.Sweep(false)
	if err != nil || len(out) != 1 || out[0].Action != core.ActionBuried {
		t.Fatalf("sweep: %+v %v", out, err)
	}
	state, err := svc.Store.ReadSnapshot()
	if err != nil || len(state.Items) != 0 || len(state.Burials) != 1 {
		t.Fatalf("burial not committed: %+v %v", state, err)
	}
	results, err := svc.Resurrect([]string{source})
	if err != nil || core.GraveFailed(results) || len(results) != 1 || results[0].Action != core.ActionResurrected {
		t.Fatalf("resurrect: %+v %v", results, err)
	}
	data, err := os.ReadFile(source)
	if err != nil || string(data) != "keep" {
		t.Fatalf("payload changed: %q %v", data, err)
	}
	buried, err := svc.Bury([]string{source}, false, "")
	if err != nil || core.GraveFailed(buried) {
		t.Fatalf("manual burial: %+v %v", buried, err)
	}
	if _, err := os.Stat(source); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("burial did not move source: %v", err)
	}
}

func TestPOSIXBurialWithChildren(t *testing.T) {
	dir := t.TempDir()
	parent := filepath.Join(dir, "目录", strings.Repeat("long", 35), strings.Repeat("path", 35))
	child := filepath.Join(parent, "子文件.txt")
	if err := os.MkdirAll(parent, 0700); err != nil {
		t.Fatal(err)
	}
	makePOSIXPayload(t, child, false, "child content")
	want := posixID(t, child)
	now := time.Now()
	svc := &core.Service{Store: core.Store{Home: filepath.Join(dir, "data")}, FS: FS{}, Now: func() time.Time { return now }}
	added, err := svc.Add([]string{child}, core.AddOptions{ExpiresAt: now.Add(time.Hour)})
	if err != nil || core.Failed(added) {
		t.Fatalf("add child: %+v %v", added, err)
	}
	now = now.Add(2 * time.Hour)
	rows, err := svc.Bury([]string{parent}, false, "")
	if err != nil || core.GraveFailed(rows) {
		t.Fatalf("directory with child did not bury: %+v %v", rows, err)
	}
	st, err := svc.Store.ReadSnapshot()
	if err != nil || len(st.Items) != 0 || len(st.Burials) != 1 || len(st.Burials[0].Contains) != 1 {
		t.Fatalf("child lifespan not retired: %+v %v", st, err)
	}
	rows, err = svc.Resurrect([]string{parent})
	if err != nil || core.GraveFailed(rows) {
		t.Fatalf("restore directory with child: %+v %v", rows, err)
	}
	assertPOSIXPayload(t, child, false, want, "child content")
	rows, err = svc.Bury([]string{parent}, false, "")
	if err != nil || core.GraveFailed(rows) {
		t.Fatalf("rebury: %+v %v", rows, err)
	}
	rows, err = svc.PurgeRefs([]string{fmt.Sprint(rows[0].Burial.ID)}, false)
	if err != nil || core.GraveFailed(rows) || len(rows) != 1 || rows[0].Action != core.ActionPurged {
		t.Fatalf("purge: %+v %v", rows, err)
	}
}

func TestPOSIXMoveRoundTrip(t *testing.T) {
	for _, name := range []string{"file", "目录-雪-🪦", strings.Repeat("long", 48)} {
		for _, directory := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/dir=%t", name, directory), func(t *testing.T) {
				dir := t.TempDir()
				source, target := filepath.Join(dir, name), filepath.Join(dir, "buried")
				payload := source
				if directory {
					if err := os.Mkdir(source, 0700); err != nil {
						t.Fatal(err)
					}
					payload = filepath.Join(source, "child")
				}
				if err := os.WriteFile(payload, []byte("original"), 0600); err != nil {
					t.Fatal(err)
				}
				fs := FS{}
				id, err := fs.Identify(source)
				if err != nil {
					t.Fatal(err)
				}
				if err := fs.MovePinned(source, id, target); err != nil {
					t.Fatalf("bury: %v", err)
				}
				parent, err := fs.OpenRestoreParent(dir)
				if err != nil {
					t.Fatal(err)
				}
				defer parent.Close()
				if _, err := fs.MovePinnedInto(target, id, parent, name); err != nil {
					t.Fatalf("restore: %v", err)
				}
				if got, err := fs.Identify(source); err != nil || got != id {
					t.Fatalf("identity changed: %+v %v", got, err)
				}
				if data, err := os.ReadFile(payload); err != nil || string(data) != "original" {
					t.Fatalf("content changed: %q %v", data, err)
				}
			})
		}
	}
}

func TestPOSIXMakeGraveIsExclusive(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "graveyard", "one")
	fs := FS{}
	if err := fs.MakeGrave(dir); err != nil {
		t.Fatal(err)
	}
	if err := fs.MakeGrave(dir); !errors.Is(err, os.ErrExist) {
		t.Fatal(err)
	}
}

func TestPOSIXMoveRaceRollback(t *testing.T) {
	for _, directory := range []bool{false, true} {
		t.Run(fmt.Sprint(directory), func(t *testing.T) {
			dir := t.TempDir()
			source, other, target := filepath.Join(dir, "source"), filepath.Join(dir, "other"), filepath.Join(dir, "grave")
			makePOSIXPayload(t, source, directory, "tracked")
			makePOSIXPayload(t, other, directory, "stranger")
			want, foreign := posixID(t, source), posixID(t, other)
			fs := FS{beforeRename: func() {
				mustPOSIXRename(t, source, source+"-old")
				mustPOSIXRename(t, other, source)
			}}
			err := fs.MovePinned(source, want, target)
			var mismatch *corefs.MoveMismatchError
			if !errors.As(err, &mismatch) || mismatch.Retained != "" {
				t.Fatalf("race was not rolled back: %v", err)
			}
			assertPOSIXPayload(t, source, directory, foreign, "stranger")
			assertPOSIXPayload(t, source+"-old", directory, want, "tracked")
			if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("rollback left moved object: %v", err)
			}
		})
	}
}

func TestPOSIXUnsupportedRenamePreservesObjects(t *testing.T) {
	dir := t.TempDir()
	source, target := filepath.Join(dir, "source"), filepath.Join(dir, "grave")
	makePOSIXPayload(t, source, false, "tracked")
	want := posixID(t, source)
	fs := FS{rename: func(int, string, int, string) error { return unix.EINVAL }}
	if err := fs.MovePinned(source, want, target); !errors.Is(err, corefs.ErrUnsupported) {
		t.Fatalf("unsupported rename: %v", err)
	}
	assertPOSIXPayload(t, source, false, want, "tracked")
	if _, err := os.Lstat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unsupported rename moved object: %v", err)
	}
}

func TestPOSIXMoveRejectsNewHardLink(t *testing.T) {
	dir := t.TempDir()
	source, target := filepath.Join(dir, "source"), filepath.Join(dir, "grave")
	makePOSIXPayload(t, source, false, "tracked")
	want := posixID(t, source)
	before, err := os.Stat(source)
	if err != nil {
		t.Fatal(err)
	}
	fs := FS{beforeRename: func() {
		if err := os.Link(source, filepath.Join(dir, "alias")); err != nil {
			t.Fatal(err)
		}
	}}
	var mismatch *corefs.MoveMismatchError
	if err := fs.MovePinned(source, want, target); !errors.As(err, &mismatch) || mismatch.Retained != "" {
		t.Fatalf("new hard link was accepted: %v", err)
	}
	after, err := os.Stat(source)
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("linked object not returned intact: %v", err)
	}
	if data, err := os.ReadFile(filepath.Join(dir, "alias")); err != nil || string(data) != "tracked" {
		t.Fatalf("hard link changed: %q %v", data, err)
	}
	if _, err := os.Stat(target); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("hard-linked object stayed in grave: %v", err)
	}
}

func TestPOSIXReturnedRaceRetainsEntry(t *testing.T) {
	dir := t.TempDir()
	source, other := filepath.Join(dir, "source"), filepath.Join(dir, "other")
	makePOSIXPayload(t, source, false, "tracked")
	makePOSIXPayload(t, other, false, "stranger")
	want, foreign := posixID(t, source), posixID(t, other)
	fs := FS{beforeRename: func() {
		mustPOSIXRename(t, source, source+"-old")
		mustPOSIXRename(t, other, source)
	}}
	now := time.Now()
	svc := &core.Service{Store: core.Store{Home: filepath.Join(dir, "data")}, FS: fs, Now: func() time.Time { return now }}
	rows, err := svc.Add([]string{source}, core.AddOptions{ExpiresAt: now.Add(time.Hour)})
	if err != nil || core.Failed(rows) {
		t.Fatalf("add: %+v %v", rows, err)
	}
	now = now.Add(2 * time.Hour)
	out, err := svc.Sweep(false)
	if err != nil || len(out) != 1 || out[0].Action != core.ActionRetry {
		t.Fatalf("race outcome: %+v %v", out, err)
	}
	st, err := svc.Store.ReadSnapshot()
	if err != nil || len(st.Items) != 1 || len(st.Burials) != 0 || *st.Items[0].Identity != want {
		t.Fatalf("returned race discarded entry: %+v %v", st, err)
	}
	assertPOSIXPayload(t, source, false, foreign, "stranger")
	assertPOSIXPayload(t, source+"-old", false, want, "tracked")
}

func TestPOSIXQuarantineNeverPurges(t *testing.T) {
	dir := t.TempDir()
	source, other := filepath.Join(dir, "source"), filepath.Join(dir, "other")
	makePOSIXPayload(t, source, false, "tracked")
	makePOSIXPayload(t, other, false, "stranger")
	want, foreign := posixID(t, source), posixID(t, other)
	fs := FS{beforeRename: func() {
		mustPOSIXRename(t, source, source+"-old")
		mustPOSIXRename(t, other, source)
	}, beforeRollback: func() { makePOSIXPayload(t, source, false, "occupied") }}
	now := time.Now()
	svc := &core.Service{Store: core.Store{Home: filepath.Join(dir, "data")}, FS: fs, Now: func() time.Time { return now }}
	rows, err := svc.Add([]string{source}, core.AddOptions{ExpiresAt: now.Add(time.Hour)})
	if err != nil || core.Failed(rows) {
		t.Fatalf("register: %+v %v", rows, err)
	}
	now = now.Add(2 * time.Hour)
	out, err := svc.Sweep(false)
	if err != nil || len(out) != 1 || out[0].Action != core.ActionRetry {
		t.Fatalf("quarantine: %+v %v", out, err)
	}
	svc.FS = FS{}
	pending, err := svc.PendingGraves()
	if err != nil || len(pending) != 1 || pending[0].Status != "identity-mismatch" {
		t.Fatalf("identity mismatch was not durably quarantined: %+v %v", pending, err)
	}
	b := *pending[0].Burial
	assertPOSIXPayload(t, b.Grave, false, foreign, "stranger")
	if data, err := os.ReadFile(source); err != nil || string(data) != "occupied" {
		t.Fatalf("rollback overwrote occupied source: %q %v", data, err)
	}
	st, err := svc.Store.ReadSnapshot()
	if err != nil || len(st.Items) != 1 {
		t.Fatalf("quarantine discarded tracked entry: %+v %v", st, err)
	}
	// Even if the expected inode later reappears, the recorded mismatch must
	// remain a permanent bar to automated recovery and explicit purge.
	mustPOSIXRename(t, b.Grave, filepath.Join(dir, "saved-stranger"))
	mustPOSIXRename(t, source+"-old", b.Grave)
	now = now.Add(30 * 24 * time.Hour)
	if _, err := svc.Purge(true, false); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.PurgeRefs([]string{fmt.Sprint(b.ID)}, false); err != nil {
		t.Fatal(err)
	}
	assertPOSIXPayload(t, b.Grave, false, want, "tracked")
	pending, err = svc.PendingGraves()
	if err != nil || len(pending) != 1 || pending[0].Status != "identity-mismatch" {
		t.Fatalf("quarantine became recoverable: %+v %v", pending, err)
	}
}

func makePOSIXPayload(t *testing.T, path string, directory bool, content string) {
	t.Helper()
	if directory {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
		path = filepath.Join(path, "child")
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func posixID(t *testing.T, path string) corefs.Identity {
	t.Helper()
	id, err := (FS{}).Identify(path)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func mustPOSIXRename(t *testing.T, from, to string) {
	t.Helper()
	if err := os.Rename(from, to); err != nil {
		t.Fatal(err)
	}
}

func assertPOSIXPayload(t *testing.T, path string, directory bool, want corefs.Identity, content string) {
	t.Helper()
	if got := posixID(t, path); got != want {
		t.Fatalf("identity changed: %+v != %+v", got, want)
	}
	if directory {
		path = filepath.Join(path, "child")
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != content {
		t.Fatalf("content changed: %q %v", data, err)
	}
}
