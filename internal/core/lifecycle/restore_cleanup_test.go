//go:build windows

package lifecycle_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/pathutil"
)

type offlineCleanupFS struct {
	core.FS
	volume string
}

type cleanupPathFailureFS struct {
	core.FS
	path string
	err  error
}

func (fs cleanupPathFailureFS) RealPath(path string) (string, error) {
	if samePath(path, fs.path) {
		return "", fs.err
	}
	return fs.FS.RealPath(path)
}

func (fs offlineCleanupFS) RealPath(path string) (string, error) {
	if strings.EqualFold(filepath.VolumeName(path), fs.volume) {
		return "", os.ErrPermission
	}
	return fs.FS.RealPath(path)
}

func (fs offlineCleanupFS) IdentifyFollowing(path string) (core.Identity, error) {
	if strings.EqualFold(filepath.VolumeName(path), fs.volume) {
		return core.Identity{}, os.ErrPermission
	}
	return fs.FS.IdentifyFollowing(path)
}

func TestCleanupReceiptRetainsUnreferencedVolume(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	// No other record names this volume; FS models it being offline. No file
	// is created outside the fixture and no drive mapping is installed.
	b.Path = `Z:\original\payload.txt`
	root := core.GraveyardRoot(f.svc, b.Path)
	b.Grave = filepath.Join(root, filepath.Base(filepath.Dir(b.Grave)), "payload.txt")
	for _, id := range []*core.Identity{b.Identity, b.GraveID} {
		id.Volume = `Z:\`
	}
	f.svc.FS = offlineCleanupFS{FS: f.fs, volume: "Z:"}
	if err := f.svc.Store.Update(func(st *core.State) error {
		st.Burials = nil
		st.RestoreCleanups = []core.RestoreCleanup{{Kind: core.ActionAbandoned, Burial: b, CommittedAt: f.clock}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	pending, err := f.svc.PendingGraves()
	found := false
	for _, p := range pending {
		found = found || samePath(p.Manifest, root) && p.Status == "unverifiable"
	}
	if err == nil || !found {
		t.Fatalf("cleanup receipt root disappeared from scan: %+v %v", pending, err)
	}
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || len(st.RestoreCleanups) != 1 {
		t.Fatalf("offline cleanup receipt was discarded: %+v %v", st, err)
	}
}

func TestUnreleasedCleanupKeyIsIgnored(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(f.svc.Store.Home, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.svc.Store.Home, "store.json"), []byte(`{"version":7,"nextId":8,"abandonedRestores":[7]}`), 0600); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || len(st.RestoreCleanups) != 0 {
		t.Fatalf("unreleased key was not ignored: %+v %v", st, err)
	}
}

func TestCleanupReceiptSurvivesMissingManifest(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, kind := range []string{"resurrected", "abandoned"} {
		entries := []string{"sweep", "graveyard"}
		if kind == "resurrected" {
			entries = append(entries, "resurrect")
		}
		for _, entry := range entries {
			t.Run(kind+"/"+entry, func(t *testing.T) {
				f := newFixture(t)
				b := buryOne(t, f, "payload.txt")
				_, unlock := interruptResurrection(t, f, b)
				unlock()
				manifest := filepath.Dir(b.Grave) + ".json"
				release := holdManifest(t, manifest)
				if kind == "abandoned" {
					outside := filepath.Join(f.dir, "elsewhere")
					if err := os.Mkdir(outside, 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.Rename(b.Path, filepath.Join(outside, "payload.txt")); err != nil {
						t.Fatal(err)
					}
					if rows, err := f.svc.PurgeRefs([]string{b.Path}, false); err != nil || core.GraveFailed(rows) {
						t.Fatal(rows, err)
					}
				} else if _, err := f.svc.ReadGraveyard(); err != nil {
					t.Fatal(err)
				}
				var state map[string]json.RawMessage
				committed, err := f.svc.Store.ReadSnapshot()
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal([]byte(read(t, filepath.Join(f.svc.Store.Home, committed.Graveyard.File))), &state); err != nil {
					t.Fatal(err)
				}
				var receipts []map[string]any
				if err := json.Unmarshal(state["restoreCleanups"], &receipts); err != nil || len(receipts) != 1 || receipts[0]["kind"] != kind {
					t.Fatalf("committed cleanup lost locating receipt: %s %v", state["restoreCleanups"], err)
				}
				release()
				if err := os.Remove(manifest); err != nil {
					t.Fatal(err)
				}
				// Crash after metadata removal, before receipt retirement.
				if entry == "sweep" {
					if _, err := f.svc.Sweep(false); err != nil {
						t.Fatal(err)
					}
				} else if entry == "resurrect" {
					rows, err := f.svc.Resurrect([]string{b.Path})
					if err != nil || core.GraveFailed(rows) || len(rows) != 1 || rows[0].Action != core.ActionResurrected || !samePath(rows[0].Path, b.Path) {
						t.Fatalf("path retry lost committed cleanup receipt: %+v %v", rows, err)
					}
				} else if _, err := f.svc.ReadGraveyard(); err != nil {
					t.Fatal(err)
				}
				st, err := f.svc.Store.ReadSnapshot()
				if err != nil {
					t.Fatal(err)
				}
				data, _ := json.Marshal(st)
				state = nil
				if err := json.Unmarshal(data, &state); err != nil {
					t.Fatal(err)
				}
				var remaining []json.RawMessage
				if raw := state["restoreCleanups"]; raw != nil {
					if err := json.Unmarshal(raw, &remaining); err != nil {
						t.Fatal(err)
					}
				}
				if len(remaining) != 0 {
					t.Fatalf("cleanup receipt did not converge without manifest: %s", data)
				}
			})
		}
	}
}

func TestCleanupReceiptAcceptsExtendedGrave(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	_, unlock := interruptResurrection(t, f, b)
	unlock()
	manifest := filepath.Dir(b.Grave) + ".json"
	release := holdManifest(t, manifest)
	if _, err := f.svc.ReadGraveyard(); err != nil {
		t.Fatal(err)
	}
	release()
	if err := os.Remove(manifest); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Store.Update(func(st *core.State) error {
		if len(st.RestoreCleanups) != 1 {
			t.Fatal("fixture lacks committed cleanup receipt")
		}
		st.RestoreCleanups[0].Burial.Grave = `\\?\` + b.Grave
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	view, err := f.svc.ReadGraveyard()
	st, readErr := f.svc.Store.ReadSnapshot()
	if err != nil || readErr != nil || len(view.Pending) != 0 || len(st.RestoreCleanups) != 0 || len(view.Restored) != 1 || read(t, b.Path) != "payload.txt" {
		t.Fatalf("extended Grave cleanup receipt did not converge: %+v %v %v", view, err, readErr)
	}
}

func TestCleanupReceiptConvergesAfterRootRemoved(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, kind := range []string{core.ActionResurrected, core.ActionAbandoned} {
		for _, mode := range []string{"missing", "invalid-identity", "offline", "permission", "replaced-grave"} {
			t.Run(kind+"/"+mode, func(t *testing.T) {
				f := newFixture(t)
				b := buryOne(t, f, "payload.txt")
				_, unlock := interruptResurrection(t, f, b)
				unlock()
				actual := b.Path
				if kind == core.ActionAbandoned {
					actual = filepath.Join(f.dir, "elsewhere", "payload.txt")
					if err := os.Mkdir(filepath.Dir(actual), 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.Rename(b.Path, actual); err != nil {
						t.Fatal(err)
					}
				}
				manifest := filepath.Dir(b.Grave) + ".json"
				release := holdManifest(t, manifest)
				if kind == core.ActionAbandoned {
					if _, err := f.svc.PurgeRefs([]string{b.Path}, false); err != nil {
						t.Fatal(err)
					}
				} else if _, err := f.svc.ReadGraveyard(); err != nil {
					t.Fatal(err)
				}
				release()
				if err := os.Remove(manifest); err != nil {
					t.Fatal(err)
				}
				root := filepath.Dir(filepath.Dir(b.Grave))
				if err := os.Remove(root); err != nil {
					t.Fatal(err)
				}
				before, err := f.svc.Store.ReadSnapshot()
				if err != nil || len(before.RestoreCleanups) != 1 {
					t.Fatalf("cleanup fixture: %+v %v", before, err)
				}
				switch mode {
				case "invalid-identity":
					if err := f.svc.Store.Update(func(st *core.State) error { st.RestoreCleanups[0].Burial.GraveID = nil; return nil }); err != nil {
						t.Fatal(err)
					}
				case "offline":
					f.svc.FS = cleanupPathFailureFS{FS: f.fs, path: pathutil.VolumeRoot(root), err: core.ErrNotFound}
				case "permission":
					f.svc.FS = cleanupPathFailureFS{FS: f.fs, path: root, err: os.ErrPermission}
				case "replaced-grave":
					if err := os.MkdirAll(filepath.Dir(b.Grave), 0700); err != nil {
						t.Fatal(err)
					}
					if id, err := f.fs.Identify(filepath.Dir(b.Grave)); err != nil || id == *b.GraveID {
						t.Fatalf("replacement retained original grave identity: %+v %v", id, err)
					}
				}
				view, err := f.svc.ReadGraveyard()
				after, readErr := f.svc.Store.ReadSnapshot()
				if readErr != nil {
					t.Fatal(readErr)
				}
				if mode == "missing" {
					if err != nil || len(view.Pending) != 0 || len(after.RestoreCleanups) != 0 || exists(root) {
						t.Fatalf("cleanup receipt did not converge after root removal: %+v receipts=%d err=%v", view, len(after.RestoreCleanups), err)
					}
				} else if len(after.RestoreCleanups) != 1 || len(view.Pending) == 0 {
					t.Fatalf("unverified cleanup evidence retired: %+v %v", view, err)
				}
				if before.Stats != after.Stats || read(t, actual) != "payload.txt" {
					t.Fatal("cleanup changed committed content or statistics")
				}
				if mode == "replaced-grave" && !exists(filepath.Dir(b.Grave)) {
					t.Fatal("cleanup touched replacement grave")
				}
			})
		}
	}
}

func TestRestoreCleanupValidatesPlannedName(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, tc := range []struct {
		name  string
		valid bool
	}{
		{"payload.txt", true}, {strings.Repeat("a", 255), true}, {strings.Repeat("😀", 127) + "a", true},
		{strings.Repeat("a", 256), false}, {strings.Repeat("😀", 128), false},
		{"", false}, {".", false}, {"..", false}, {`a\b`, false}, {"a/b", false}, {"a:b", false},
		{"NUL", false}, {"name.", false}, {"name ", false}, {"a?", false}, {"a\x00", false},
	} {
		t.Run(fmt.Sprintf("%d-%t", len(tc.name), tc.valid), func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "payload.txt")
			_, unlock := interruptResurrection(t, f, b)
			unlock()
			manifest := filepath.Dir(b.Grave) + ".json"
			rewriteManifest(t, manifest, func(m map[string]any) { m["resurrect"].(map[string]any)["name"] = tc.name })
			pending, err := f.svc.PendingGraves()
			if err != nil || len(pending) != 1 || (pending[0].Status == "resurrected") != tc.valid {
				t.Fatalf("manifest name validation: %+v %v", pending, err)
			}
			var receipt core.RestoreCleanup
			if err := json.Unmarshal([]byte(read(t, manifest)), &receipt); err != nil {
				t.Fatal(err)
			}
			receipt.Kind, receipt.Burial, receipt.Path = core.ActionResurrected, b, b.Path
			if err := f.svc.Store.Update(func(st *core.State) error {
				st.Burials = nil
				st.RestoreCleanups = []core.RestoreCleanup{receipt}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(manifest); err != nil {
				t.Fatal(err)
			}
			pending, err = f.svc.PendingGraves()
			if err != nil || len(pending) != 1 || (pending[0].Status == "restore-committed") != tc.valid {
				t.Fatalf("cleanup name validation: %+v %v", pending, err)
			}
		})
	}
}
