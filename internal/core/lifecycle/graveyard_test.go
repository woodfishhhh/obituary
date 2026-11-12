//go:build windows

package lifecycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func TestGraveyardRootUsesVolumeSpelling(t *testing.T) {
	f := newFixture(t)
	path := f.file("payload.txt")
	unc := `\\localhost\` + strings.TrimSuffix(filepath.VolumeName(path), ":") + "$" + path[len(filepath.VolumeName(path)):]
	// No filesystem identity (including cloned serial/root IDs) proves two
	// volume spellings share a namespace. These assertions create no graves.
	wantPrefix := `\\localhost\` + strings.TrimSuffix(filepath.VolumeName(path), ":") + `$\.obituary\graveyard\`
	if got := core.GraveyardRoot(f.svc, unc); !strings.HasPrefix(got, wantPrefix) {
		t.Fatalf("volume alias merged into another namespace: %s", got)
	}
	f.svc.Store.Home = `\\localhost\` + strings.TrimSuffix(filepath.VolumeName(path), ":") + "$" + f.svc.Store.Home[2:]
	if got := core.GraveyardRoot(f.svc, unc); !samePath(got, filepath.Join(f.svc.Store.Home, "graveyard")) {
		t.Fatalf("same namespace did not use Home: %s", got)
	}
}

func TestEquivalentDataHomeUsesSameLedgerKey(t *testing.T) {
	f := newFixture(t)
	want := core.GraveyardRoot(f.svc, `Z:\payload.txt`)
	t.Setenv("OBIT_HOME", `\\?\`+f.svc.Store.Home)
	home, err := userdirs.Data()
	if err != nil {
		t.Fatal(err)
	}
	f.svc.Store.Home = home
	if got := core.GraveyardRoot(f.svc, `Z:\payload.txt`); got != want {
		t.Fatalf("equivalent Home changed the existing ledger key: %s != %s", got, want)
	}
}

func TestPurgeOtherLocalUnicodeRoot(t *testing.T) {
	f := newFixture(t)
	f.svc.Store.Home = filepath.Join(filepath.Dir(f.dir), "K")
	b := buryOne(t, f, "keep.txt")
	state, err := f.svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	other := *f.svc
	other.Store.Home = filepath.Join(filepath.Dir(f.dir), "K")
	if err := os.MkdirAll(filepath.Join(other.Store.Home, "graveyard"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := other.Store.Update(func(st *core.State) error { *st = state; return nil }); err != nil {
		t.Fatal(err)
	}
	rows, err := other.Purge(true, false)
	if err == nil && !core.GraveFailed(rows) || !exists(b.Grave) {
		t.Fatalf("purge authorized another local Unicode root: %+v %v", rows, err)
	}
	if read(t, b.Grave) != "keep.txt" {
		t.Fatal("other ledger content changed")
	}
}

func TestRootIdentityRejectsForeignRootJunction(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, manifest := range []bool{false, true} {
		t.Run(fmt.Sprint(manifest), func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "keep.txt")
			st, err := f.svc.Store.Read()
			if err != nil {
				t.Fatal(err)
			}
			other := *f.svc
			other.Store.Home = filepath.Join(filepath.Dir(f.dir), "other-home")
			if err := other.Store.Update(func(s *core.State) error { *s = st; return nil }); err != nil {
				t.Fatal(err)
			}
			root := filepath.Join(other.Store.Home, "graveyard")
			if out, err := exec.Command("cmd", "/c", "mklink", "/J", root, filepath.Join(f.svc.Store.Home, "graveyard")).CombinedOutput(); err != nil {
				t.Fatalf("mklink: %v %s", err, out)
			}
			if manifest {
				metadata := b
				metadata.Size = 0
				data, err := json.Marshal(struct {
					Version int `json:"version"`
					core.Burial
				}{Version: 1, Burial: metadata})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Dir(b.Grave)+".json", data, 0600); err != nil {
					t.Fatal(err)
				}
				pending, err := other.PendingGraves()
				if err != nil || len(pending) != 1 || pending[0].Status != "unverifiable" || !strings.Contains(pending[0].Error, "escapes this ledger's graveyard root") {
					t.Fatalf("foreign root junction authorized manifest: %+v %v", pending, err)
				}
			}
			rows, err := other.Purge(true, false)
			if err == nil && !core.GraveFailed(rows) || !exists(b.Grave) {
				t.Fatalf("foreign root junction authorized purge: %+v %v", rows, err)
			}
			if read(t, b.Grave) != "keep.txt" || len(f.burials()) != 1 {
				t.Fatal("foreign grave changed")
			}
		})
	}
}

func TestGraveyardThroughJunctionHome(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	f := newFixture(t)
	real := f.svc.Store.Home
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	f.svc.Store.Home = filepath.Join(filepath.Dir(real), "home-alias")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", f.svc.Store.Home, real).CombinedOutput(); err != nil {
		t.Fatalf("mklink: %v %s", err, out)
	}
	b := buryOne(t, f, "restore.txt")
	rows, err := f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
	if err != nil || core.GraveFailed(rows) || len(rows) != 1 || read(t, rows[0].Path) != "restore.txt" {
		t.Fatalf("junction Home failed resurrection: %+v %v", rows, err)
	}
	b = buryOne(t, f, "purge.txt")
	rows, err = f.svc.PurgeRefs([]string{fmt.Sprint(b.ID)}, false)
	if err != nil || core.GraveFailed(rows) || len(rows) != 1 || rows[0].Action != core.ActionPurged || exists(b.Grave) {
		t.Fatalf("junction Home failed purge: %+v %v", rows, err)
	}
}

// Let names pass to exercise the independent final-component identity check.
// All identities still come from real Windows handles.
type rootNamesEqualFS struct{ *testFS }

func (rootNamesEqualFS) SameName(a, b string) (bool, error) { return true, nil }

func TestFoldedRootJunction(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, mode := range []string{"purge", "import", "identity", "import-identity"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			f.svc.Store.Home = filepath.Join(filepath.Dir(f.dir), "K")
			stop := core.SetBurialStep(f.svc, func(stage string) error {
				if stage == "committed" {
					return errors.New("interrupted before manifest cleanup")
				}
				return nil
			})
			defer stop()
			if _, err := f.svc.Bury([]string{f.file("keep.txt")}, false, ""); err == nil {
				t.Fatal("fixture did not interrupt")
			}
			stop()
			st, err := f.svc.Store.Read()
			if err != nil {
				t.Fatal(err)
			}
			b := st.Burials[0]
			other := *f.svc
			other.Store.Home = filepath.Join(filepath.Dir(f.dir), "K")
			if err := os.MkdirAll(other.Store.Home, 0700); err != nil {
				t.Fatal(err)
			}
			if out, err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(other.Store.Home, "graveyard"), filepath.Join(f.svc.Store.Home, "graveyard")).CombinedOutput(); err != nil {
				t.Fatalf("mklink: %v %s", err, out)
			}
			if strings.Contains(mode, "identity") {
				other.FS = rootNamesEqualFS{f.fs}
			}
			if strings.HasPrefix(mode, "import") {
				if _, err := other.Sweep(false); err != nil {
					t.Fatal(err)
				}
				got, err := other.Store.Read()
				if err != nil {
					t.Fatal(err)
				}
				if len(got.Burials) != 0 {
					t.Fatalf("folded junction imported foreign manifest: %+v", got.Burials)
				}
			} else {
				if err := other.Store.Update(func(s *core.State) error { *s = st; return nil }); err != nil {
					t.Fatal(err)
				}
				rows, err := other.PurgeRefs([]string{fmt.Sprint(b.ID)}, false)
				if err == nil && !core.GraveFailed(rows) || !exists(b.Grave) {
					t.Fatalf("folded junction authorized foreign purge: %+v %v", rows, err)
				}
			}
			if read(t, b.Grave) != "keep.txt" {
				t.Fatal("foreign payload changed")
			}
		})
	}
}

type rootFollowingIdentityFS struct{ *testFS }

func (f rootFollowingIdentityFS) Identify(path string) (core.Identity, error) {
	return f.IdentifyFollowing(path)
}

func TestRootNameBoundary(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	f := newFixture(t)
	real := filepath.Join(f.dir, "K")
	if err := os.Mkdir(real, 0700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(f.dir, "K")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", alias, real).CombinedOutput(); err != nil {
		t.Fatalf("mklink: %v %s", err, out)
	}
	// Intentionally following here isolates name comparison from the identity
	// barrier. Both IDs are real; the production FS does not follow the leaf.
	f.svc.FS = rootFollowingIdentityFS{f.fs}
	got, err := core.SameDirectory(f.svc, real, alias)
	if err != nil || got {
		t.Fatalf("root name boundary accepted another location: %v %v", got, err)
	}
}

func TestMissingRootRetainsRenamedGraveObject(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "keep.txt")
	oldRoot := filepath.Join(filepath.Dir(f.dir), "old-graveyard")
	if err := os.Rename(filepath.Join(f.svc.Store.Home, "graveyard"), oldRoot); err != nil {
		t.Fatal(err)
	}
	b.Grave = filepath.Join(oldRoot, filepath.Base(filepath.Dir(b.Grave)), filepath.Base(b.Grave))
	if err := os.Rename(b.Grave, b.Grave+".renamed"); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Store.Update(func(st *core.State) error { st.Burials[0] = b; return nil }); err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.Purge(true, false)
	if len(f.burials()) != 1 || err == nil && !core.GraveFailed(rows) {
		t.Fatalf("missing leaf discarded an existing grave directory: %+v %v", rows, err)
	}
	if read(t, b.Grave+".renamed") != "keep.txt" {
		t.Fatal("renamed payload changed")
	}
}

func TestEmptyGraveRequiresExplicitPurge(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "keep.txt")
	if err := os.Remove(b.Grave); err != nil {
		t.Fatal(err)
	}
	ref := fmt.Sprint(b.ID)
	rows, err := f.svc.Resurrect([]string{ref})
	want := "grave is empty; its object was removed outside Obituary; obit purge " + ref + " clears the record"
	if err != nil || len(rows) != 1 || !strings.Contains(rows[0].Error, want) {
		t.Fatalf("empty grave diagnostic missing: %+v %v", rows, err)
	}
	pending, err := f.svc.PendingGraves()
	if err != nil || len(pending) != 1 || !strings.Contains(pending[0].Error, want) {
		t.Fatalf("empty grave not reported for doctor: %+v %v", pending, err)
	}
	if _, err := f.svc.Purge(true, false); err != nil || len(f.burials()) != 1 {
		t.Fatalf("bulk purge cleared empty grave: %v", err)
	}
	rows, err = f.svc.PurgeRefs([]string{ref}, false)
	if err != nil || core.GraveFailed(rows) || len(rows) != 1 || rows[0].Action != core.ActionLost || len(f.burials()) != 0 || exists(filepath.Dir(b.Grave)) {
		t.Fatalf("explicit purge did not clear verified empty grave: %+v %v", rows, err)
	}
}

func TestEmptyGravePurgePreservesUnverifiedDirectory(t *testing.T) {
	for _, mode := range []string{"identity", "foreign", "pending"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "keep.txt")
			if err := os.Remove(b.Grave); err != nil {
				t.Fatal(err)
			}
			dir := filepath.Dir(b.Grave)
			switch mode {
			case "identity":
				if err := os.Rename(dir, dir+"-old"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
			case "foreign":
				if err := os.WriteFile(filepath.Join(dir, "foreign"), []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			case "pending":
				if err := os.WriteFile(dir+".json", []byte("broken"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			rows, err := f.svc.PurgeRefs([]string{fmt.Sprint(b.ID)}, false)
			if err == nil && !core.GraveFailed(rows) || len(f.burials()) != 1 || !exists(dir) {
				t.Fatalf("empty purge removed an unverified grave: %+v %v", rows, err)
			}
		})
	}
}

func TestMissingRootSnapshotAfterFirstBurial(t *testing.T) {
	f := newFixture(t)
	ancestor := filepath.Join(filepath.Dir(f.dir), "Long Home Ancestor")
	if err := os.Mkdir(ancestor, 0700); err != nil {
		t.Fatal(err)
	}
	f.svc.Store.Home = filepath.Join(shortPath(t, ancestor), "missing-home")
	a := filepath.Dir(f.file("directory-a/keep.txt"))
	b := f.file("payload-b.txt")
	first := f.add(a, time.Hour)
	second := f.add(b, time.Hour)
	parent, err := f.fs.OpenRestoreParent(a)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	f.svc.FS = &moveHookFS{testFS: f.fs, after: func(to string) {
		if _, err := f.fs.MovePinnedInto(b, *second.Identity, parent, "payload-b.txt"); err != nil {
			t.Fatal(err)
		}
	}}
	f.clock = first.ExpiresAt
	rows, err := f.svc.Sweep(false)
	graves := f.burials()
	if err != nil || len(graves) != 1 || graves[0].ID != first.ID || len(f.items()) != 1 || f.items()[0].ID != second.ID {
		t.Fatalf("cached missing root allowed a second burial inside a grave: %+v graves=%+v err=%v", rows, graves, err)
	}
	if read(t, filepath.Join(graves[0].Grave, "payload-b.txt")) != "payload-b.txt" {
		t.Fatal("concurrent move into pinned directory lost content")
	}
	restored, err := f.svc.Resurrect([]string{fmt.Sprint(first.ID)})
	if err != nil || core.GraveFailed(restored) || read(t, filepath.Join(a, "payload-b.txt")) != "payload-b.txt" {
		t.Fatalf("parent burial no longer restores its content: %+v %v", restored, err)
	}
}

type resolvingCreatedRootFS struct {
	core.FS
	root, real string
}

func (fs resolvingCreatedRootFS) RealPath(path string) (string, error) {
	if path == fs.root {
		if _, err := os.Stat(path); err != nil {
			return "", err
		}
		return fs.real, nil
	}
	return fs.FS.RealPath(path)
}

func TestMissingRootSnapshotResolvesAfterCreation(t *testing.T) {
	f := newFixture(t)
	root, real := filepath.Join(f.dir, "missing"), filepath.Join(f.dir, "resolved")
	f.svc.FS = resolvingCreatedRootFS{FS: f.fs, root: root, real: real}
	inside := core.GraveyardContainment(f.svc, root)
	if inside(filepath.Join(real, "payload")) {
		t.Fatal("missing root unexpectedly resolved")
	}
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	if !inside(filepath.Join(real, "payload")) {
		t.Fatal("cached missing root failed to resolve after creation")
	}
}

func TestGraveyardScanUsesOriginalPathRoot(t *testing.T) {
	f := newFixture(t)
	path := f.file("payload.txt")
	// Obtain this ledger's old key with a syntactic, unavailable volume. No
	// directory is created there or at the real volume root.
	external := core.GraveyardRoot(f.svc, `Z:\payload.txt`)
	old := filepath.VolumeName(path) + external[2:]
	b := core.Burial{ID: 1, Path: path, Grave: filepath.Join(old, "1-20261107T120000Z-abcdef", "payload.txt")}
	for _, st := range []core.State{{Burials: []core.Burial{b}}, {RestoreCleanups: []core.RestoreCleanup{{Burial: b}}}} {
		roots := core.GraveyardRoots(f.svc, st)
		found := false
		for _, root := range roots {
			found = found || samePath(root, old)
		}
		if found || len(roots) != 1 || !samePath(roots[0], filepath.Join(f.svc.Store.Home, "graveyard")) {
			t.Fatalf("graveyard accepted obsolete development layout: %v", roots)
		}
	}
}

func TestReadGraveyardHonorsStoreLock(t *testing.T) {
	for _, mode := range []string{"context", "store-deadline"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			buryOne(t, f, "keep.txt")
			release, err := core.LockFile(filepath.Join(f.svc.Store.Home, "store.lock"), true)
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
			defer cancel()
			svc := f.svc.WithContext(ctx)
			if mode == "store-deadline" {
				svc = f.svc.WithContext(t.Context())
				svc.Store = svc.Store.WithDeadline(time.Now().Add(100 * time.Millisecond))
			}
			view, err := svc.ReadGraveyard()
			if !errors.Is(err, context.DeadlineExceeded) || view != nil {
				t.Fatalf("graveyard read bypassed store lock: %+v %v", view, err)
			}
		})
	}
}

func TestReadGraveyardConcurrentUpdates(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	f := newFixture(t)
	buryOne(t, f, "keep.txt")
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for range 4 {
		wg.Go(func() {
			for range 100 {
				if _, err := f.svc.ReadGraveyard(); err != nil {
					errs <- err
					return
				}
			}
		})
	}
	for range 100 {
		if err := f.svc.Store.Update(func(st *core.State) error { st.Burials[0].Note = strings.Repeat("x", 8192); return nil }); err != nil {
			t.Errorf("concurrent graveyard reader broke ledger commit: %v", err)
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestReadGraveyardLockDeadline(t *testing.T) {
	for _, mode := range []string{"context", "store-deadline"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "keep.txt")
			release, err := core.LockFile(filepath.Join(f.svc.Store.Home, "graveyard.lock"), true)
			if err != nil {
				t.Fatal(err)
			}
			defer release()
			ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
			defer cancel()
			svc := f.svc.WithContext(ctx)
			if mode == "store-deadline" {
				svc = f.svc.WithContext(t.Context())
				svc.Store = svc.Store.WithDeadline(time.Now().Add(100 * time.Millisecond))
			}
			view, err := svc.ReadGraveyard()
			if !errors.Is(err, context.DeadlineExceeded) || view != nil || !exists(b.Grave) {
				t.Fatalf("graveyard view ignored its lock deadline: %+v %v", view, err)
			}
		})
	}
}

func TestExpiredItemIsBuriedAndCanBeResurrected(t *testing.T) {
	f := newFixture(t)
	path := f.file("debug.log")
	item := f.add(path, time.Hour)

	f.sweepAt(time.Hour)
	graves := f.burials()
	if exists(path) || len(graves) != 1 || graves[0].ID != item.ID || !exists(graves[0].Grave) {
		t.Fatalf("want the file moved into a grave, got %+v (exists=%v)", graves, exists(path))
	}
	if root := filepath.Join(f.svc.Store.Home, "graveyard"); !strings.HasPrefix(strings.ToLower(graves[0].Grave), strings.ToLower(root)) {
		t.Fatalf("grave %s is not under %s", graves[0].Grave, root)
	}

	results, err := f.svc.Resurrect([]string{"#1"})
	if err != nil || results[0].Error != "" {
		t.Fatalf("resurrect: %v %+v", err, results)
	}
	if !exists(path) || read(t, path) != "debug.log" || len(f.burials()) != 0 {
		t.Fatalf("want the file back with its content and no burial left")
	}
	if sum, _ := f.svc.Summarize(); sum.Lifetime.Buried != 1 || sum.Lifetime.Resurrected != 1 {
		t.Fatalf("lifetime stats %+v", sum.Lifetime)
	}
}

func TestReadGraveyardClearsReadyRestore(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	f.fs.fail = errors.New("move busy")
	got, err := f.svc.Resurrect([]string{b.Path})
	f.fs.fail = nil
	if err != nil || !core.GraveFailed(got) {
		t.Fatal(got, err)
	}
	view, err := f.svc.ReadGraveyard()
	if err != nil || len(view.Pending) != 0 || len(view.Burials) != 1 || exists(filepath.Dir(b.Grave)+".json") {
		t.Fatalf("graveyard retained ready restore intent: %+v %v", view, err)
	}
	got, err = f.svc.Purge(true, false)
	if err != nil || core.GraveFailed(got) || exists(b.Grave) {
		t.Fatalf("ready restore cleanup blocked purge: %+v %v", got, err)
	}
}

func TestLedgersNeverShareAGraveOnAnotherVolume(t *testing.T) {
	a := &core.Service{Store: core.Store{Home: `C:\Users\me\AppData\Local\obituary`}, FS: &testFS{}}
	b := &core.Service{Store: core.Store{Home: `C:\Users\me\test-home`}, FS: &testFS{}}
	ra, rb := core.GraveyardRoot(a, `D:\work\x.log`), core.GraveyardRoot(b, `D:\work\x.log`)
	if strings.EqualFold(ra, rb) || !strings.HasPrefix(ra, `D:\.obituary\graveyard\`) {
		t.Fatalf(`roots %s and %s must differ and stay in D:\.obituary`, ra, rb)
	}
	if got := core.GraveyardRoot(a, `C:\work\x.log`); got != `C:\Users\me\AppData\Local\obituary\graveyard` {
		t.Fatalf("own volume root = %s", got)
	}
}

func TestUnavailableVolumeDoesNotLoseBurial(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "keep.txt")
	f.svc.FS = offlineGraveFS{testFS: f.fs, root: core.GraveyardRoot(f.svc, b.Path)}
	got, err := f.svc.Purge(true, false)
	if err != nil || len(got) != 1 || !core.GraveFailed(got) || got[0].Action == core.ActionLost || len(f.burials()) != 1 || read(t, b.Grave) != "keep.txt" {
		t.Fatalf("unavailable volume was mistaken for lost grave: %+v, %v", got, err)
	}
}

func TestMissingGraveyardRootIsLost(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "keep.txt")
	root := core.GraveyardRoot(f.svc, b.Path)
	if err := os.Rename(root, root+".outside-obituary"); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.Purge(true, false)
	if err != nil || len(got) != 1 || got[0].Action != core.ActionLost || len(f.burials()) != 0 {
		t.Fatalf("missing root on an online volume must clear only the index: %+v, %v", got, err)
	}
	kept := filepath.Join(root+".outside-obituary", filepath.Base(filepath.Dir(b.Grave)), filepath.Base(b.Grave))
	if read(t, kept) != "keep.txt" {
		t.Fatal("lost-index cleanup touched the object")
	}
}

func TestReplacedVolumeDoesNotLoseBurial(t *testing.T) {
	for _, missing := range []string{"root", "grave"} {
		t.Run(missing, func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "keep.txt")
			root := core.GraveyardRoot(f.svc, b.Path)
			if missing == "grave" {
				if err := os.Rename(filepath.Dir(b.Grave), filepath.Dir(b.Grave)+".saved"); err != nil {
					t.Fatal(err)
				}
				root = ""
			}
			f.svc.FS = replacedVolumeFS{testFS: f.fs, missing: root}
			rows, err := f.svc.Purge(true, false)
			if err != nil || len(rows) != 1 || !core.GraveFailed(rows) || rows[0].Action == core.ActionLost || len(f.burials()) != 1 {
				t.Fatalf("different volume lost the burial index: %+v, %v", rows, err)
			}
		})
	}
}
