//go:build windows

package core_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"golang.org/x/sys/windows"
)

func TestPurgeAuditAfterTransientSharingViolation(t *testing.T) {
	for _, target := range []string{"store.json", "store.json.tmp"} {
		t.Run(target, func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "payload.txt")
			var h windows.Handle
			unlock := func() {
				if h != 0 {
					windows.CloseHandle(h)
					h = 0
				}
			}
			defer unlock()
			stop := core.SetBeforePurgeDelete(f.svc, func() {
				p, _ := windows.UTF16PtrFromString(filepath.Join(f.svc.Store.Home, target))
				var err error
				h, err = windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_ALWAYS, 0, 0)
				if err != nil {
					t.Fatal(err)
				}
			})
			defer stop()
			_, err := f.svc.PurgeRefs([]string{fmt.Sprint(b.ID)}, false)
			stop()
			if err == nil || exists(filepath.Dir(b.Grave)) {
				t.Fatalf("fixture must delete then fail ledger commit: %v grave=%v", err, exists(filepath.Dir(b.Grave)))
			}
			before, err := f.svc.Store.ReadSnapshot()
			if err != nil || len(before.Burials) != 1 || before.Burials[0].PurgeIntent == nil || before.Burials[0].PurgeIntent.Basis != "explicit selection" || before.Stats.Purged != 0 || len(before.PendingAudit) != 0 {
				t.Fatalf("purge intent missing or success published before commit: %+v %v", before, err)
			}
			unlock()
			// A new service instance must finish from durable evidence alone.
			restarted := *f.svc
			rows, err := restarted.PurgeRefs([]string{fmt.Sprint(b.ID)}, false)
			if err != nil || len(rows) != 1 || rows[0].Action != core.ActionPurged {
				t.Fatalf("purge retry lost verified deletion: %+v %v", rows, err)
			}
			after, err := restarted.Store.ReadSnapshot()
			if err != nil || len(after.Burials) != 0 || after.Stats.Purged != 1 || after.Stats.PurgedBytes != b.Size {
				t.Fatalf("purge retry lost accounting: %+v %v", after, err)
			}
			events, err := restarted.Store.History(100)
			purged, lost := 0, 0
			for _, e := range events {
				if e.ID == b.ID {
					if e.Action == core.ActionPurged {
						purged++
					}
					if e.Action == core.ActionLost {
						lost++
					}
				}
			}
			if err != nil || purged != 1 || lost != 0 {
				t.Fatalf("purge retry lost terminal audit: purged=%d lost=%d %v", purged, lost, err)
			}
		})
	}
}

func TestPurgeIntentMustCommitBeforeDeletion(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	tmp := filepath.Join(f.svc.Store.Home, "store.json.tmp")
	if err := os.Mkdir(tmp, 0700); err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.Purge(true, false)
	if err == nil || !exists(b.Grave) || len(rows) != 1 || rows[0].Action == core.ActionPurged {
		t.Fatalf("uncommitted intent deleted a grave: %+v %v", rows, err)
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || len(st.Burials) != 1 || st.Burials[0].PurgeIntent != nil || st.Stats.Purged != 0 {
		t.Fatalf("failed intent changed ledger: %+v %v", st, err)
	}
	if err := os.Remove(tmp); err != nil {
		t.Fatal(err)
	}
	if rows, err := f.svc.Purge(true, false); err != nil || core.GraveFailed(rows) {
		t.Fatalf("purge did not resume: %+v %v", rows, err)
	}
}

func TestPurgeRetryRequiresConfirmedIntent(t *testing.T) {
	core.RequireFsync(t)
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	restore := core.SetReplacementSync(func(*os.File) error { return errors.New("intent flush interrupted") })
	defer restore()
	for attempt := 1; attempt <= 2; attempt++ {
		rows, err := f.svc.PurgeRefs([]string{fmt.Sprint(b.ID)}, false)
		if !exists(b.Grave) {
			t.Fatalf("unconfirmed purge retry deleted content on attempt %d: %+v %v", attempt, rows, err)
		}
		if err == nil {
			t.Fatalf("attempt %d accepted unconfirmed intent: %+v", attempt, rows)
		}
		st, readErr := f.svc.Store.ReadSnapshot()
		if readErr != nil || len(st.Burials) != 1 || st.Burials[0].PurgeIntent == nil || len(st.PendingAudit) != 0 {
			t.Fatalf("invalid retry evidence: %+v %v", st, readErr)
		}
	}
	restore()
	rows, err := f.svc.PurgeRefs([]string{fmt.Sprint(b.ID)}, false)
	if err != nil || len(rows) != 1 || rows[0].Action != core.ActionPurged || exists(b.Grave) {
		t.Fatalf("confirmed retry failed: %+v %v", rows, err)
	}
}

func TestPurgeIntentProtectsInterruptedDeletion(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	var held windows.Handle
	unlock := func() {
		if held != 0 {
			windows.CloseHandle(held)
			held = 0
		}
	}
	defer unlock()
	stop := core.SetBeforePurgeDelete(f.svc, func() {
		name, _ := windows.UTF16PtrFromString(b.Grave)
		var err error
		held, err = windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
	})
	defer stop()
	rows, err := f.svc.Purge(true, false)
	stop()
	if err != nil || !core.GraveFailed(rows) || !exists(b.Grave) {
		t.Fatalf("fixture must stop deletion: %+v %v", rows, err)
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || st.Stats.Purged != 0 || len(st.PendingAudit) != 0 || len(st.Burials) != 1 || st.Burials[0].PurgeIntent == nil {
		t.Fatalf("incomplete purge published success: %+v %v", st, err)
	}
	unlock()
	rows, err = f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
	if err != nil || len(rows) != 1 || !strings.Contains(rows[0].Error, "purge is in progress") || !exists(b.Grave) {
		t.Fatalf("partly purged object was resurrected: %+v %v", rows, err)
	}
	f.clock = f.clock.Add(time.Hour)
	_, rows, err = f.svc.Tick()
	if err != nil || len(rows) != 1 || rows[0].Action != core.ActionPurged || exists(b.Grave) {
		t.Fatalf("background did not resume authorized intent: %+v %v", rows, err)
	}
}

func TestPurgeDetachesCancellation(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "purge.log")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	out, err := f.svc.WithContext(ctx).Purge(true, false)
	if err != nil || len(out) != 1 || out[0].Action != core.ActionPurged || exists(b.Grave) || len(f.burials()) != 0 {
		t.Fatalf("purge did not detach cancellation: %+v %v", out, err)
	}
}

func TestPurgeHonoursRetention(t *testing.T) {
	f := newFixture(t)
	path := f.file("big.bin")
	f.add(path, time.Hour)
	f.sweepAt(time.Hour)
	grave := f.burials()[0].Grave

	f.clock = f.clock.Add(core.DefaultRetention - time.Minute)
	if out, _ := f.svc.Purge(false, false); len(out) != 0 || !exists(grave) {
		t.Fatalf("purged before its retention: %+v", out)
	}
	f.clock = f.clock.Add(time.Minute)
	if out, _ := f.svc.Purge(false, true); len(out) != 1 || !exists(grave) {
		t.Fatalf("dry run must report and keep, got %+v", out)
	}
	out, err := f.svc.Purge(false, false)
	if err != nil || len(out) != 1 || out[0].Action != core.ActionPurged || exists(grave) || exists(filepath.Dir(grave)) {
		t.Fatalf("want the grave removed, got %+v %v", out, err)
	}
	if sum, _ := f.svc.Summarize(); sum.Graves != 0 || sum.Lifetime.Purged != 1 || sum.Lifetime.PurgedBytes != int64(len("big.bin")) {
		t.Fatalf("stats after purge %+v", sum)
	}
}

func TestPurgeRefusesAPathOutsideTheGraveyard(t *testing.T) {
	f := newFixture(t)
	path := f.file("a.log")
	f.add(path, time.Hour)
	f.sweepAt(time.Hour)
	precious := f.file("precious/thesis.docx")
	err := f.svc.Store.Update(func(st *core.State) error {
		st.Burials[0].Grave = precious // a corrupted or tampered ledger
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	out, err := f.svc.Purge(true, false)
	if err != nil || len(out) != 1 || out[0].Error == "" || !exists(precious) {
		t.Fatalf("purge must refuse a grave outside the graveyard, got %+v %v (exists=%v)", out, err, exists(precious))
	}
}

func TestPurgeRefusesAGraveRedirectedByAJunction(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	f := newFixture(t)
	b := buryOne(t, f, "a.log")
	graveDir := filepath.Dir(b.Grave)
	// Someone moves the real grave aside and leaves a junction to user files in its place.
	if err := os.Rename(graveDir, graveDir+"-moved"); err != nil {
		t.Fatal(err)
	}
	user := filepath.Join(f.dir, "user-docs")
	precious := filepath.Join(user, "a.log")
	if err := os.MkdirAll(user, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(precious, []byte("precious"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", graveDir, user).CombinedOutput(); err != nil {
		t.Fatalf("mklink: %v %s", err, out)
	}

	out, err := f.svc.Purge(true, false)
	if err != nil || len(out) != 1 || out[0].Error == "" || !exists(precious) {
		t.Fatalf("purge must refuse a redirected grave, got %+v %v (precious exists=%v)", out, err, exists(precious))
	}
	if _, err := f.svc.Resurrect([]string{"#1"}); err != nil || !exists(precious) {
		t.Fatal("resurrect must refuse it too")
	}
}

func TestPurgeRefusesAGraveHoldingSomethingElse(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "a.log")
	stray := filepath.Join(filepath.Dir(b.Grave), "stray.txt")
	if err := os.WriteFile(stray, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, _ := f.svc.Purge(true, false); len(out) != 1 || out[0].Error == "" || !exists(stray) || !exists(b.Grave) {
		t.Fatalf("a grave with extra content must not be purged: %+v", out)
	}

	f2 := newFixture(t)
	b2 := buryOne(t, f2, "a.log")
	if err := os.Remove(b2.Grave); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b2.Grave, []byte("swapped in"), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, _ := f2.svc.Purge(true, false); len(out) != 1 || out[0].Error == "" || !exists(b2.Grave) {
		t.Fatalf("a grave whose object was swapped must not be purged: %+v", out)
	}
}

func TestTickBusySkipsPurge(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "keep.txt")
	f.clock = b.PurgeAt
	release, ok, err := core.TryLockExclusive(filepath.Join(f.svc.Store.Home, "sweep.lock"))
	if err != nil || !ok {
		t.Fatalf("lock: %v, %v", ok, err)
	}
	defer release()
	out, purged, err := f.svc.Tick()
	if !errors.Is(err, core.ErrSweepBusy) || len(out) != 0 || len(purged) != 0 || !exists(b.Grave) || len(f.burials()) != 1 {
		t.Fatalf("busy Tick touched grave: outcomes=%+v purge=%+v err=%v", out, purged, err)
	}
}

func TestPurgeScansConflictingManifests(t *testing.T) {
	for _, dry := range []bool{false, true} {
		t.Run(fmt.Sprint(dry), func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "keep.txt")
			f.clock = b.PurgeAt
			manifest := conflictingManifest(t, f, b)
			rows, err := f.svc.Purge(true, dry)
			for _, r := range rows {
				if r.Action == core.ActionPurged {
					t.Fatalf("conflict authorized purge: %+v", rows)
				}
			}
			if err != nil || !exists(b.Grave) || !exists(manifest) || len(f.burials()) != 1 {
				t.Fatalf("conflict lost: %+v, %v", rows, err)
			}
		})
	}
}

func TestPurgeBlocksUnscannableRoot(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "keep.txt")
	f.clock = b.PurgeAt
	f.svc.FS = &scanFailureFS{testFS: f.fs, root: core.GraveyardRoot(f.svc, b.Path)}
	rows, err := f.svc.Purge(false, false)
	if err != nil || !exists(b.Grave) || len(f.burials()) != 1 {
		t.Fatalf("unscannable root purged: %+v, %v", rows, err)
	}
}

func TestPurgeScanFailureUsesOriginalVolumeSpelling(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "keep.txt")
	// Model an obsolete development record whose original spelling used an
	// alias but whose grave used Home's spelling. Never create an alias root.
	b.Path = `\\localhost\` + strings.TrimSuffix(filepath.VolumeName(b.Path), ":") + "$" + b.Path[2:]
	if err := f.svc.Store.Update(func(st *core.State) error { st.Burials[0] = b; return nil }); err != nil {
		t.Fatal(err)
	}
	f.svc.FS = &scanFailureFS{testFS: f.fs, root: core.GraveyardRoot(f.svc, b.Path)}
	rows, err := f.svc.PurgeRefs([]string{fmt.Sprint(b.ID)}, false)
	if err != nil || len(rows) != 1 || rows[0].Action != core.ActionBlocked || len(f.burials()) != 1 || read(t, b.Grave) != "keep.txt" {
		t.Fatalf("original-root scan failure did not block old-layout burial: %+v %v", rows, err)
	}
}

func TestPurgeMissingExpectedRootRetainsExistingGrave(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "keep.txt")
	// Model an unsupported old layout entirely inside scratch: the old grave
	// remains while the currently expected Home/graveyard no longer exists.
	oldRoot := filepath.Dir(filepath.Dir(b.Grave))
	legacyRoot := filepath.Join(filepath.Dir(f.dir), "legacy-graveyard")
	if err := os.Rename(oldRoot, legacyRoot); err != nil {
		t.Fatal(err)
	}
	b.Grave = filepath.Join(legacyRoot, filepath.Base(filepath.Dir(b.Grave)), filepath.Base(b.Grave))
	if err := f.svc.Store.Update(func(st *core.State) error { st.Burials[0] = b; return nil }); err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.PurgeRefs([]string{fmt.Sprint(b.ID)}, false)
	if err != nil || len(rows) != 1 || !core.GraveFailed(rows) || rows[0].Action == core.ActionLost || len(f.burials()) != 1 {
		t.Fatalf("missing expected root discarded an existing grave record: %+v %v", rows, err)
	}
	if read(t, b.Grave) != "keep.txt" {
		t.Fatal("unsupported layout lost its object")
	}
}

func TestBlockedBurialBackoffAndWarningDedup(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "keep.txt")
	f.clock = b.PurgeAt
	conflictingManifest(t, f, b)
	for i := 0; i < 2; i++ {
		if _, _, err := f.svc.Tick(); err != nil {
			t.Fatal(err)
		}
	}
	st, err := f.svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	next, ok := st.NextDue()
	if !ok || !next.Equal(f.clock.Add(time.Minute)) {
		t.Fatalf("blocked grave keeps waking daemon: next=%v", next)
	}
	events, err := f.svc.Store.History(100)
	if err != nil {
		t.Fatal(err)
	}
	warnings := 0
	for _, ev := range events {
		if ev.Action == "recovery warning" {
			warnings++
		}
	}
	if warnings != 1 {
		t.Fatalf("repeated recovery warnings during backoff: %d", warnings)
	}
}

func TestPurgeFailureBacksOff(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "keep.txt")
	f.clock = b.PurgeAt
	extra := filepath.Join(filepath.Dir(b.Grave), "extra.txt")
	if err := os.WriteFile(extra, []byte("unrelated"), 0o600); err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.Purge(false, false)
	if err != nil || !core.GraveFailed(rows) {
		t.Fatalf("unsafe grave accepted: %+v %v", rows, err)
	}
	st, err := f.svc.Store.Read()
	if err != nil || st.Burials[0].PurgeAttempts != 1 || !st.Burials[0].NextPurgeAttempt.Equal(f.clock.Add(time.Minute)) {
		t.Fatalf("failed purge has no backoff: %+v %v", st.Burials, err)
	}
	if err := os.Remove(extra); err != nil {
		t.Fatal(err)
	}
	if _, rows, err := f.svc.Tick(); err != nil || len(rows) != 0 || !exists(b.Grave) {
		t.Fatalf("retried early: %+v %v", rows, err)
	}
	f.clock = f.clock.Add(time.Minute)
	if rows, err := f.svc.Purge(false, false); err != nil || len(rows) != 1 || rows[0].Action != core.ActionPurged {
		t.Fatalf("retry never completed: %+v %v", rows, err)
	}
}

func TestTickBlocksManifestContentIDAndFilenameID(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "keep.txt")
	f.clock = b.PurgeAt
	// A mismatched filename must still protect the burial named in the JSON.
	manifest := filepath.Join(core.GraveyardRoot(f.svc, b.Path), "99-mismatch.json")
	data, err := json.Marshal(struct {
		Version int `json:"version"`
		core.Burial
	}{1, b})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifest, data, 0o600); err != nil {
		t.Fatal(err)
	}
	out, purged, err := f.svc.Tick()
	if err != nil || len(out) == 0 || len(purged) != 0 || len(f.burials()) != 1 || read(t, b.Grave) != "keep.txt" || !exists(manifest) {
		t.Fatalf("Tick lost content ID from recovery block: %+v, %+v, %v", out, purged, err)
	}
}
