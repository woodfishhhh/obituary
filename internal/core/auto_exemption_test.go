package core_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestAutomaticParentExpiryProtectsLaterResurrection(t *testing.T) {
	if testing.Short() {
		t.Skip("automatic expiry uses a real Git repository")
	}
	for _, mode := range []string{core.ModeGraveyard, core.ModeRecycle} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			testenv.InitRepo(t, f.dir)
			f.svc.Owner, f.svc.Mode = auto.Owner, mode
			path := f.file("test-results/keep.txt")
			f.svc.Mode = core.ModeGraveyard
			rows, err := f.svc.Bury([]string{path}, false, "")
			if err != nil || len(rows) != 1 || rows[0].Action != core.ActionBuried {
				t.Fatalf("bury: %+v %v", rows, err)
			}
			parent := filepath.Dir(path)
			added, err := f.svc.Add([]string{parent}, core.AddOptions{ExpiresAt: f.clock.Add(time.Second), OnlyNew: true, Source: "rule", Project: f.dir})
			if err != nil || core.Failed(added) {
				t.Fatalf("add: %+v %v", added, err)
			}
			res, err := f.svc.Resurrect([]string{path})
			if err != nil || core.GraveFailed(res) {
				t.Fatalf("resurrect: %+v %v", res, err)
			}
			f.svc.Mode = mode
			out := f.sweepAt(2 * time.Second)
			if !exists(path) || len(out) != 1 || out[0].Action == core.ActionBuried || out[0].Action == core.ActionRecycled {
				t.Fatalf("automatic parent moved later resurrected child: %+v", out)
			}
			f.svc.Mode = core.ModeGraveyard
			rows, err = f.svc.Bury([]string{parent}, false, "")
			if err != nil || rows[0].Action != core.ActionBuried {
				t.Fatalf("explicit burial lost authority: %+v %v", rows, err)
			}
		})
	}
}

func TestAutomaticDirectoryKeepEndsOnDescendantWrite(t *testing.T) {
	f := newFixture(t)
	path := f.file("test-results/nested/output.tmp")
	dir := filepath.Dir(filepath.Dir(path))
	f.add(dir, time.Hour)
	if rows, err := f.svc.Cancel([]string{dir}); err != nil || core.Failed(rows) {
		t.Fatalf("spare: %+v %v", rows, err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("changed descendant contents and size"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(dir, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	id, err := f.svc.FS.Identify(dir)
	if err != nil {
		t.Fatal(err)
	}
	exempt, err := f.svc.AutomaticExempt(&st, dir, id)
	if err != nil || exempt {
		t.Fatalf("descendant write did not release directory keep: %v %v", exempt, err)
	}
}

func TestAutomaticExemptionMaintenance(t *testing.T) {
	for _, change := range []string{"write", "delete", "uncertain", "unchanged"} {
		t.Run(change, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("kept.txt")
			f.add(path, time.Hour)
			if rows, err := f.svc.Cancel([]string{path}); err != nil || core.Failed(rows) {
				t.Fatalf("spare: %+v %v", rows, err)
			}
			switch change {
			case "write":
				if err := os.WriteFile(path, []byte("later write"), 0600); err != nil {
					t.Fatal(err)
				}
			case "delete":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "uncertain":
				f.fs.locateErr = os.ErrPermission
			}
			if _, err := f.svc.Sweep(false); err != nil {
				t.Fatal(err)
			}
			st, err := f.svc.Store.Read()
			want := 0
			if change == "uncertain" || change == "unchanged" {
				want = 1
			}
			if err != nil || len(st.AutoExemptions) != want {
				t.Fatalf("maintenance retained stale or discarded uncertain exemption: %+v %v", st.AutoExemptions, err)
			}
		})
	}
}

func TestLegacyDirectoryExemptionLearnsTree(t *testing.T) {
	f := newFixture(t)
	path := f.file("kept/nested/output.tmp")
	dir := filepath.Dir(filepath.Dir(path))
	f.add(dir, time.Hour)
	if rows, err := f.svc.Cancel([]string{dir}); err != nil || core.Failed(rows) {
		t.Fatalf("spare: %+v %v", rows, err)
	}
	// v11 recorded only the directory itself, without the descendant tree.
	snap, release, err := f.svc.FS.ObserveWatch(dir, false)
	if err != nil {
		t.Fatal(err)
	}
	release()
	if err := f.svc.Store.Update(func(st *core.State) error { st.AutoExemptions[0].Snapshot = &snap; return nil }); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.Read()
	if err != nil || len(st.AutoExemptions) != 1 {
		t.Fatalf("missing tree baseline discarded keep decision: %+v %v", st, err)
	}
	if err := os.WriteFile(path, []byte("later external descendant write"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(dir, snap.Modified, snap.Modified); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	st, err = f.svc.Store.Read()
	if err != nil || len(st.AutoExemptions) != 0 {
		t.Fatalf("legacy directory ignored a later descendant write: %+v %v", st.AutoExemptions, err)
	}
}

func TestAutomaticExemptionIgnoresUnrelatedHardlink(t *testing.T) {
	f := newFixture(t)
	kept := f.file("kept.log")
	f.add(kept, time.Hour)
	if rows, err := f.svc.Cancel([]string{kept}); err != nil || core.Failed(rows) {
		t.Fatalf("spare: %+v %v", rows, err)
	}
	if err := os.Link(kept, filepath.Join(f.dir, "kept-alias.log")); err != nil {
		t.Fatal(err)
	}
	path := f.file("unrelated.log")
	id, err := f.svc.FS.Identify(path)
	if err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	exempt, err := f.svc.AutomaticExempt(&st, path, id)
	if err != nil || exempt {
		t.Fatalf("unrelated hardlink blocked candidate: %v %v", exempt, err)
	}
	parentID, err := f.svc.FS.Identify(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	exempt, err = f.svc.AutomaticExempt(&st, f.dir, parentID)
	if err == nil && !exempt {
		t.Fatal("related hardlink bypassed keep decision")
	}
}

func TestAutomaticKeepRequiresLaterWrite(t *testing.T) {
	f := newFixture(t)
	path := f.file("keep.log")
	f.add(path, time.Hour)
	old, err := f.svc.CaptureObservations([]string{path}, "old-write")
	if err != nil || len(old) != 1 {
		t.Fatalf("capture: %+v %v", old, err)
	}
	f.clock = f.clock.Add(time.Second)
	if rows, err := f.svc.Cancel([]string{path}); err != nil || core.Failed(rows) {
		t.Fatalf("cancel: %+v %v", rows, err)
	}
	if err := f.svc.ObserveAutomaticWrite(old[0]); err != nil {
		t.Fatal(err)
	}
	options := core.AddOptions{ExpiresAt: f.clock.Add(time.Hour), Source: "rule", OnlyNew: true}
	if rows, err := f.svc.Add([]string{path}, options); err != nil || !core.Failed(rows) {
		t.Fatalf("old write discarded keep decision: %+v %v", rows, err)
	}
	f.clock = f.clock.Add(time.Second)
	fresh, err := f.svc.CaptureObservations([]string{path}, "new-write")
	if err != nil || len(fresh) != 1 {
		t.Fatalf("capture: %+v %v", fresh, err)
	}
	if err := f.svc.ObserveAutomaticWrite(fresh[0]); err != nil {
		t.Fatal(err)
	}
	if rows, err := f.svc.Add([]string{path}, options); err != nil || core.Failed(rows) {
		t.Fatalf("new write did not release keep decision: %+v %v", rows, err)
	}
}

func TestAutomaticKeepIdentityAndMetadata(t *testing.T) {
	for _, change := range []string{"rename", "mtime", "size", "replacement"} {
		t.Run(change, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("keep.log")
			f.add(path, time.Hour)
			if rows, err := f.svc.Cancel([]string{path}); err != nil || core.Failed(rows) {
				t.Fatalf("cancel: %+v %v", rows, err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "rename", "replacement":
				moved := filepath.Join(f.dir, "moved.log")
				if err := os.Rename(path, moved); err != nil {
					t.Fatal(err)
				}
				if change == "rename" {
					path = moved
				} else {
					f.file("keep.log")
				}
			case "mtime":
				if err := os.Chtimes(path, info.ModTime(), info.ModTime().Add(time.Second)); err != nil {
					t.Fatal(err)
				}
			case "size":
				if err := os.WriteFile(path, []byte("changed file size"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
					t.Fatal(err)
				}
			}
			rows, err := f.svc.Add([]string{path}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour), Source: "rule", OnlyNew: true})
			if err != nil || core.Failed(rows) != (change == "rename") {
				t.Fatalf("keep identity/metadata gate: %+v %v", rows, err)
			}
		})
	}
}
