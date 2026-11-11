//go:build windows

package core_test

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"golang.org/x/sys/windows"
)

func TestResurrectRejectsChangedParents(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, mode := range []string{"junction", "protected", "legacy-junction"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "output/payload.txt")
			if mode == "legacy-junction" {
				if err := f.svc.Store.Update(func(st *core.State) error { st.Burials[0].Parents = nil; return nil }); err != nil {
					t.Fatal(err)
				}
			}
			parent := filepath.Dir(b.Path)
			outside := filepath.Join(f.dir, "outside")
			if err := os.Mkdir(outside, 0o700); err != nil {
				t.Fatal(err)
			}
			if mode == "protected" {
				f.svc.Guard.System = append(f.svc.Guard.System, parent)
			} else {
				if err := os.Rename(parent, parent+"-old"); err != nil {
					t.Fatal(err)
				}
				if out, err := exec.Command("cmd", "/c", "mklink", "/J", parent, outside).CombinedOutput(); err != nil {
					t.Fatalf("junction: %v %s", err, out)
				}
			}
			got, err := f.svc.Resurrect([]string{b.Path})
			if err != nil || len(got) != 1 || got[0].Error == "" || !exists(b.Grave) || len(f.burials()) != 1 || exists(filepath.Join(outside, "payload.txt")) {
				t.Fatalf("changed parent allowed resurrection: %+v %v", got, err)
			}
		})
	}
}

func TestResurrectOrdinaryReplacement(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprint(legacy), func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "output/payload.txt")
			if legacy {
				if err := f.svc.Store.Update(func(st *core.State) error { st.Burials[0].Parents = nil; return nil }); err != nil {
					t.Fatal(err)
				}
			}
			parent := filepath.Dir(b.Path)
			if err := os.Rename(parent, parent+"-old"); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(parent, 0o700); err != nil {
				t.Fatal(err)
			}
			got, err := f.svc.Resurrect([]string{b.Path})
			if err != nil || core.GraveFailed(got) || !exists(b.Path) {
				t.Fatalf("ordinary replacement blocked resurrection: %+v %v", got, err)
			}
		})
	}
}

func TestResurrectParentRecreationWithUnavailableLedger(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "output/payload.txt")
	parent := filepath.Dir(b.Path)
	if err := os.Remove(parent); err != nil {
		t.Fatal(err)
	}
	var locked windows.Handle
	defer func() {
		if locked != 0 {
			windows.CloseHandle(locked)
		}
	}()
	f.fs.identifyFollowingErr = func(path string) error {
		if path != parent || locked != 0 {
			return nil
		}
		p, err := windows.UTF16PtrFromString(filepath.Join(f.svc.Store.Home, "store.json"))
		if err != nil {
			t.Fatal(err)
		}
		locked, err = windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.svc.Store.Confirm(func(*core.State) error { return nil }); err == nil {
			t.Fatal("ledger fault did not block a real write")
		}
		return nil
	}
	f.fs.fail = errors.New("temporary move failure")
	got, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || !core.GraveFailed(got) || !exists(b.Grave) || locked == 0 {
		t.Fatalf("retry setup lost burial: %+v %v", got, err)
	}
	windows.CloseHandle(locked)
	locked = 0
	f.fs.identifyFollowingErr, f.fs.fail = nil, nil
	if !reflect.DeepEqual(f.burials()[0].Parents, b.Parents) {
		t.Fatal("recreation rewrote recorded parents")
	}
	got, err = f.svc.Resurrect([]string{b.Path})
	if err != nil || core.GraveFailed(got) || read(t, b.Path) != "output/payload.txt" {
		t.Fatalf("unavailable ledger after recreation blocked retry: %+v %v", got, err)
	}
}

func TestResurrectRecordedJunction(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, mode := range []string{"unchanged", "retargeted", "ordinary", "missing", "protected-real"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			real, other, link := filepath.Join(f.dir, "real"), filepath.Join(f.dir, "other"), filepath.Join(f.dir, "project-link")
			for _, path := range []string{real, other} {
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, real).CombinedOutput(); err != nil {
				t.Fatalf("junction: %v %s", err, out)
			}
			b := buryOne(t, f, "project-link/payload.txt")
			if len(b.Parents) == 0 || !b.Parents[0].Reparse {
				t.Fatal("fixture did not record original junction")
			}
			if mode == "retargeted" || mode == "ordinary" || mode == "missing" {
				if err := os.Remove(link); err != nil {
					t.Fatal(err)
				}
				switch mode {
				case "ordinary":
					if err := os.Mkdir(link, 0o700); err != nil {
						t.Fatal(err)
					}
				case "retargeted":
					if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, other).CombinedOutput(); err != nil {
						t.Fatalf("junction: %v %s", err, out)
					}
				}
			}
			if mode == "protected-real" {
				f.svc.Guard.System = append(f.svc.Guard.System, real)
			}
			got, err := f.svc.Resurrect([]string{b.Path})
			if mode == "unchanged" {
				if err != nil || core.GraveFailed(got) || read(t, filepath.Join(real, "payload.txt")) != "project-link/payload.txt" {
					t.Fatalf("recorded junction blocked restoration: %+v %v", got, err)
				}
			} else if err != nil || !core.GraveFailed(got) || !exists(b.Grave) || exists(b.Path) {
				t.Fatalf("redirected or protected original junction accepted: %+v %v", got, err)
			}
		})
	}
}

func TestResurrectRejectsChangedParentVolume(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "output/payload.txt")
	if err := f.svc.Store.Update(func(st *core.State) error { st.Burials[0].Parents[0].Identity.Serial++; return nil }); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || !core.GraveFailed(got) || !exists(b.Grave) || exists(b.Path) {
		t.Fatalf("changed parent volume allowed resurrection: %+v %v", got, err)
	}
}

func TestResurrectRetriesConcurrentNameCollision(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "output/payload.txt")
	f.fs.onRecycle = func(string) {
		f.fs.onRecycle = nil
		if err := os.WriteFile(b.Path, []byte("new work"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || core.GraveFailed(got) || read(t, b.Path) != "new work" || read(t, got[0].Path) != "output/payload.txt" {
		t.Fatalf("concurrent collision overwritten or not retried: %+v %v", got, err)
	}
}

func TestRestoreBeforeMoveDirectoryReplacement(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, junction := range []bool{false, true} {
		t.Run(fmt.Sprint(junction), func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "output/payload.txt")
			parent := filepath.Dir(b.Path)
			outside := filepath.Join(f.dir, "outside")
			if err := os.Mkdir(outside, 0o700); err != nil {
				t.Fatal(err)
			}
			f.fs.onRecycle = func(string) {
				f.fs.onRecycle = nil
				if err := os.Rename(parent, parent+"-old"); err != nil {
					t.Fatal(err)
				}
				if junction {
					if out, err := exec.Command("cmd", "/c", "mklink", "/J", parent, outside).CombinedOutput(); err != nil {
						t.Fatalf("junction: %v %s", err, out)
					}
				} else if err := os.Mkdir(parent, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			got, err := f.svc.Resurrect([]string{b.Path})
			if exists(b.Path) || exists(filepath.Join(outside, "payload.txt")) {
				t.Fatal("restoration entered replacement parent instead of pinned directory")
			}
			if err != nil || core.GraveFailed(got) || read(t, filepath.Join(parent+"-old", "payload.txt")) != "output/payload.txt" {
				t.Fatalf("pinned parent did not receive restoration: %+v %v", got, err)
			}
			actual, err := filepath.EvalSymlinks(filepath.Join(parent+"-old", "payload.txt"))
			if err != nil || got[0].Path != actual {
				t.Fatalf("restore receipt does not name the moved file: %+v want %s: %v", got, actual, err)
			}
			events, err := f.svc.Store.History(1)
			if err != nil || len(events) != 1 || events[0].Path != actual {
				t.Fatalf("restore audit does not name the moved file: %+v %v", events, err)
			}
		})
	}
}

type restoreRaceFS struct {
	core.FS
	beforeResolve  func()
	afterFollowing func(string)
}

func (fs restoreRaceFS) Resolve(path string) (string, error) {
	if fs.beforeResolve != nil {
		fs.beforeResolve()
	}
	return fs.FS.Resolve(path)
}

func (fs restoreRaceFS) IdentifyFollowing(path string) (core.Identity, error) {
	id, err := fs.FS.IdentifyFollowing(path)
	if err == nil && fs.afterFollowing != nil {
		fs.afterFollowing(path)
	}
	return id, err
}

func TestResurrectParentPinningRace(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, timing := range []string{"resolve", "after-ancestor-check"} {
		for _, junction := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%v", timing, junction), func(t *testing.T) {
				f := newFixture(t)
				parent, outside := filepath.Join(f.dir, "output"), filepath.Join(f.dir, "outside")
				if err := os.Mkdir(outside, 0o700); err != nil {
					t.Fatal(err)
				}
				link := func(target string) {
					if out, err := exec.Command("cmd", "/c", "mklink", "/J", parent, target).CombinedOutput(); err != nil {
						t.Fatalf("junction: %v %s", err, out)
					}
				}
				if junction {
					real := filepath.Join(f.dir, "original")
					if err := os.Mkdir(real, 0o700); err != nil {
						t.Fatal(err)
					}
					link(real)
				}
				b := buryOne(t, f, "output/payload.txt")
				replaced := false
				replace := func() {
					if replaced {
						return
					}
					replaced = true
					if err := os.Rename(parent, parent+"-old"); err != nil {
						t.Fatal(err)
					}
					link(outside)
				}
				race := restoreRaceFS{FS: f.fs}
				if timing == "resolve" {
					race.beforeResolve = replace
				} else {
					checks := 0
					armed := false
					// Graveyard discovery also probes volume identity. Only count
					// ancestor checks after the restore destination is resolved.
					race.beforeResolve = func() { armed = true }
					race.afterFollowing = func(path string) {
						if armed && path == filepath.VolumeName(b.Path)+`\` {
							checks++
							if checks == 2 {
								replace()
							} // after the pinned parent's complete recheck
						}
					}
				}
				f.svc.FS = race
				got, err := f.svc.Resurrect([]string{b.Path})
				if !replaced {
					t.Fatal("parent replacement was not exercised")
				}
				if err != nil || len(got) != 1 || got[0].Error == "" || got[0].Action != "" || !exists(b.Grave) || len(f.burials()) != 1 || exists(filepath.Join(outside, "payload.txt")) {
					t.Fatalf("changed lexical parent was accepted: %+v %v", got, err)
				}
				if timing == "after-ancestor-check" && !strings.Contains(got[0].Error, "pinned directory") {
					t.Fatalf("parent identity mismatch was hidden by another check: %+v", got)
				}
			})
		}
	}
}

func TestResurrectRejectsNewAncestorJunctionSameTarget(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	f := newFixture(t)
	b := buryOne(t, f, "output/deep/payload.txt")
	parent := filepath.Dir(filepath.Dir(b.Path))
	if err := os.Rename(parent, parent+"-old"); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", parent, parent+"-old").CombinedOutput(); err != nil {
		t.Fatalf("junction: %v %s", err, out)
	}
	got, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || !core.GraveFailed(got) || !exists(b.Grave) || exists(b.Path) {
		t.Fatalf("new ancestor junction allowed resurrection: %+v %v", got, err)
	}
}

func TestLegacyVerifiedAncestorLongSuffix(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "DIRECT~1/ordinary/deep/payload.txt")
	if err := f.svc.Store.Update(func(st *core.State) error { st.Burials[0].LongNames = false; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Dir(b.Path)); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Dir(filepath.Dir(b.Path))); err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
	if err != nil || core.GraveFailed(rows) || !exists(b.Path) {
		t.Fatalf("verified ancestor contaminated ordinary missing suffix: %+v %v", rows, err)
	}
}

func TestLegacyUnicodeCaseSpelling(t *testing.T) {
	for _, parents := range []bool{false, true} {
		t.Run(fmt.Sprint(parents), func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "Àbc/payload.txt")
			dir := filepath.Dir(b.Path)
			if err := f.svc.Store.Update(func(st *core.State) error {
				st.Burials[0].LongNames = false
				st.Burials[0].Path = filepath.Join(f.dir, "àbc", "payload.txt")
				if parents {
					st.Burials[0].Parents[0].Path = filepath.Join(f.dir, "àbc")
				} else {
					st.Burials[0].Parents = nil
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if parents {
				if err := os.Rename(dir, dir+"-old"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			rows, err := f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
			if err != nil || core.GraveFailed(rows) || !exists(b.Path) || exists(b.Grave) {
				t.Fatalf("Unicode case spelling mistaken for short alias: %+v %v", rows, err)
			}
		})
	}
}

func TestLegacyAliasWithoutTilde(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, replaced := range []bool{false, true} {
		t.Run(fmt.Sprint(replaced), func(t *testing.T) {
			f := newFixture(t)
			path := f.file("Original Long Directory/payload.txt")
			dir := filepath.Dir(path)
			if out, err := exec.Command("fsutil", "file", "setshortname", dir, "NONTILDE").CombinedOutput(); err != nil {
				t.Skipf("custom short names need NTFS and privilege: %v %s", err, out)
			}
			rows, err := f.svc.Bury([]string{path}, false, "")
			if err != nil || core.GraveFailed(rows) {
				t.Fatal(rows, err)
			}
			b := *rows[0].Burial
			alias := filepath.Join(f.dir, "NONTILDE", "payload.txt")
			if err := f.svc.Store.Update(func(st *core.State) error {
				st.Burials[0].Path, st.Burials[0].LongNames = alias, false
				st.Burials[0].Parents[0].Path = filepath.Dir(alias)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if replaced {
				if err := os.Remove(dir); err != nil {
					t.Fatal(err)
				}
				dir = filepath.Join(f.dir, "Replacement Long Directory")
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
				if out, err := exec.Command("fsutil", "file", "setshortname", dir, "NONTILDE").CombinedOutput(); err != nil {
					t.Skipf("custom short names unavailable: %v %s", err, out)
				}
			}
			rows, err = f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
			if err != nil || core.GraveFailed(rows) != replaced || exists(b.Grave) != replaced || exists(filepath.Join(dir, "payload.txt")) == replaced {
				t.Fatalf("legacy alias without tilde bypassed identity check: %+v %v", rows, err)
			}
		})
	}
}

func TestResurrectRejectsMissingLegacyShortParent(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "DIRECT~1/payload.txt")
	if err := f.svc.Store.Update(func(st *core.State) error { st.Burials[0].LongNames = false; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Dir(b.Path)); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || !core.GraveFailed(got) || exists(filepath.Dir(b.Path)) || !exists(b.Grave) {
		t.Fatalf("legacy short parent was guessed: %+v %v", got, err)
	}
}

func TestResurrectLegacyShortAliasReuseIsRefused(t *testing.T) {
	f := newFixture(t)
	path := f.file("Directory With A Long Name/payload.txt")
	short := shortPath(t, path)
	f.add(path, time.Hour)
	f.sweepAt(time.Hour)
	b := f.burials()[0]
	if err := f.svc.Store.Update(func(st *core.State) error {
		current := &st.Burials[0]
		current.Path = short
		current.LongNames = false
		dir := filepath.Dir(short)
		for i := range current.Parents {
			current.Parents[i].Path = dir
			dir = filepath.Dir(dir)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Dir(path)); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(f.dir, "Directory With Another Name")
	if err := os.Mkdir(other, 0700); err != nil {
		t.Fatal(err)
	}
	if filepath.Base(shortPath(t, other)) != filepath.Base(filepath.Dir(short)) {
		t.Fatal("NTFS did not reuse short alias")
	}
	got, err := f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
	if err != nil || !core.GraveFailed(got) || exists(filepath.Join(other, "payload.txt")) || !exists(b.Grave) {
		t.Fatalf("legacy short alias restored into replacement: %+v %v", got, err)
	}
}

func TestResurrectExpandedLiteralShortShapeCanRebuild(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "DIRECT~1/payload.txt")
	if err := os.Remove(filepath.Dir(b.Path)); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || core.GraveFailed(got) || !exists(b.Path) {
		t.Fatalf("expanded literal short-shaped name was refused: %+v %v", got, err)
	}
}
