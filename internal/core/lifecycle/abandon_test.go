//go:build windows

package lifecycle_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

func TestPurgeExplicitlyAbandonsUnverifiableRestore(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, mode := range []string{"moved", "replaced", "nonempty", "identity", "found", "all"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "payload.txt")
			_, release := interruptResurrection(t, f, b)
			release()
			outside := filepath.Join(f.dir, "elsewhere", "payload.txt")
			if err := os.MkdirAll(filepath.Dir(outside), 0700); err != nil {
				t.Fatal(err)
			}
			if mode != "found" {
				if err := os.Rename(b.Path, outside); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "replaced" {
				if err := os.WriteFile(b.Path, []byte("replacement"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			dir := filepath.Dir(b.Grave)
			if mode == "nonempty" {
				if err := os.WriteFile(filepath.Join(dir, "foreign"), []byte("keep"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "identity" {
				if err := os.Rename(dir, dir+"-old"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(dir, 0700); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "moved" || mode == "replaced" {
				hint := fmt.Sprintf("obit purge %d", b.ID)
				res, _ := f.svc.Resurrect([]string{b.Path})
				spare, _ := f.svc.Cancel([]string{fmt.Sprint(b.ID)})
				path := outside
				if mode == "replaced" {
					path = b.Path
				}
				mark, _ := f.svc.Add([]string{path}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
				all, _ := f.svc.Purge(true, false)
				for _, msg := range []string{res[0].Error, spare[0].Error, mark[0].Error, all[0].Error} {
					if !strings.Contains(msg, hint) {
						t.Fatalf("unverifiable restore lacks explicit purge advice: %s", msg)
					}
				}
			}
			before, _ := f.svc.Store.Read()
			var got []core.GraveResult
			var err error
			if mode == "all" {
				got, err = f.svc.Purge(true, false)
			} else {
				got, err = f.svc.PurgeRefs([]string{fmt.Sprint(b.ID)}, false)
			}
			if err != nil || len(got) != 1 {
				t.Fatalf("explicit purge result: %+v %v", got, err)
			}
			st, _ := f.svc.Store.Read()
			switch mode {
			case "moved", "replaced":
				if len(st.Burials) != 0 || got[0].Action != "abandoned" || st.Stats != before.Stats {
					t.Fatalf("unverifiable restore was not abandoned without accounting: %+v %+v", got, st.Stats)
				}
				if read(t, outside) != "payload.txt" {
					t.Fatal("abandon changed external object")
				}
				if mode == "replaced" && read(t, b.Path) != "replacement" {
					t.Fatal("abandon changed replacement")
				}
				events, _ := f.svc.Store.History(10)
				count := 0
				for _, e := range events {
					if e.Action == "abandoned" {
						count++
						if e.Detail == "" {
							t.Fatal("abandon lost intent detail")
						}
					}
				}
				if count != 1 {
					t.Fatal("abandon audit missing")
				}
				f.add(outside, time.Hour)
			case "found":
				if len(st.Burials) != 0 || got[0].Action != core.ActionResurrected || st.Stats.Resurrected != 1 {
					t.Fatalf("found restored object was abandoned: %+v", got)
				}
			default:
				if len(st.Burials) != 1 || got[0].Action == "abandoned" || !exists(dir+".json") {
					t.Fatalf("unsafe restore abandonment: %+v", got)
				}
			}
		})
	}
}

func TestAbandonCleanupWarningKeepsOutcome(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	_, unlock := interruptResurrection(t, f, b)
	unlock()
	outside := filepath.Join(f.dir, "other")
	if err := os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(b.Path, filepath.Join(outside, "payload.txt")); err != nil {
		t.Fatal(err)
	}
	manifest := filepath.Dir(b.Grave) + ".json"
	release := holdManifest(t, manifest)
	got, err := f.svc.PurgeRefs([]string{b.Path}, false)
	if err != nil || got[0].Action != core.ActionAbandoned || got[0].Warning == "" || len(f.burials()) != 0 {
		t.Fatalf("abandon cleanup changed committed result: %+v %v", got, err)
	}
	view, err := f.svc.ReadGraveyard()
	if err != nil || len(view.Restored) != 0 || len(view.Pending) != 1 || view.Pending[0].Status != "restore-abandoned" {
		t.Fatalf("abandon cleanup reported resurrection: %+v %v", view, err)
	}
	release()
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	st, _ := f.svc.Store.Read()
	if len(st.RestoreCleanups) != 0 || st.Stats.Resurrected != 0 || st.Stats.Purged != 0 || exists(manifest) {
		t.Fatalf("abandon cleanup not idempotent: %+v", st)
	}
}

type returnRestoredFS struct {
	core.FS
	before func()
}

func (fs returnRestoredFS) FindRestored(p, id core.Identity) (string, error) {
	fs.before()
	return fs.FS.FindRestored(p, id)
}

func TestAbandonRechecksRestoredObject(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	_, release := interruptResurrection(t, f, b)
	release()
	outside := filepath.Join(f.dir, "other")
	if err := os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(outside, "payload.txt")
	if err := os.Rename(b.Path, moved); err != nil {
		t.Fatal(err)
	}
	calls := 0
	f.svc.FS = returnRestoredFS{FS: f.fs, before: func() {
		calls++
		if calls == 2 {
			if err := os.Rename(moved, b.Path); err != nil {
				t.Fatal(err)
			}
		}
	}}
	got, err := f.svc.PurgeRefs([]string{b.Path}, false)
	if err != nil || len(got) != 1 || got[0].Action != core.ActionResurrected || !exists(b.Path) {
		t.Fatalf("abandon skipped final restored identity check: %+v %v", got, err)
	}
}
