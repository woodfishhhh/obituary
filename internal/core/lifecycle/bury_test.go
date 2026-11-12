//go:build windows

package lifecycle_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

func TestBuryReservationUsesHotTransaction(t *testing.T) {
	f := newFixture(t)
	tracked := f.add(f.file("tracked"), time.Hour)
	newID, err := f.fs.Identify(f.file("untracked"))
	if err != nil {
		t.Fatal(err)
	}
	before, restore := unreadableSnapshot(t, f.svc.Store)
	old, err := core.ReserveBuryIdentity(f.svc, *tracked.Identity)
	if err != nil || old.ID != tracked.ID || old.Rev != tracked.Rev {
		t.Fatal("burial reservation read cold evidence or changed tracked revision", old, err)
	}
	reserved, err := core.ReserveBuryIdentity(f.svc, newID)
	if err != nil || reserved.ID != before.NextID || reserved.Rev != 0 {
		t.Fatal("burial reservation failed to allocate hot ID", reserved, err)
	}
	restore()
	after, err := f.svc.Store.ReadSnapshot()
	if err != nil || after.NextID != before.NextID+1 || !reflect.DeepEqual(after.Items, before.Items) || !reflect.DeepEqual(after.Graveyard, before.Graveyard) {
		t.Fatal("reservation changed evidence or lost allocation", after, err)
	}
}

func TestBuryBatchFailureBoundaries(t *testing.T) {
	for _, stage := range []string{"inspect", "reserve", "settle", "recycle-commit"} {
		t.Run(stage, func(t *testing.T) {
			f := newFixture(t)
			paths := []string{f.file("first.txt"), f.file("second.txt"), f.file("third.txt")}
			blocked := filepath.Join(f.svc.Store.Home, "store.json.tmp")
			blockCommit := func() {
				if err := os.Mkdir(blocked, 0700); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Remove(blocked); err != nil {
						t.Error(err)
					}
				})
			}
			switch stage {
			case "inspect":
				f.svc.Guard.Keep = append(f.svc.Guard.Keep, paths[1])
			case "reserve":
				once := false
				f.svc.FS = &identifyHookFS{testFS: f.fs, after: func(path string) {
					if !once && samePath(path, paths[1]) {
						once = true
						blockCommit()
					}
				}}
			case "settle":
				starts := 0
				t.Cleanup(core.SetBurialStep(f.svc, func(step string) error {
					if step == "reserved" {
						starts++
						if starts == 2 {
							return errors.New("second burial interrupted")
						}
					}
					return nil
				}))
			case "recycle-commit":
				f.svc.Mode = core.ModeRecycle
				for _, path := range paths {
					f.add(path, time.Hour)
				}
				f.fs.onRecycle = func(path string) {
					if samePath(path, paths[1]) {
						blockCommit()
					}
				}
			}
			rows, err := f.svc.Bury(paths, false, "")
			if stage == "reserve" {
				if err == nil || len(rows) != len(paths) || len(f.burials()) != 0 {
					t.Fatalf("failed batch reservation changed the ledger: %+v %v", rows, err)
				}
				for i, row := range rows {
					if row.Ref != paths[i] || row.Action != core.ActionNotExecuted || row.Error == "" || !exists(paths[i]) {
						t.Fatalf("failed batch reservation lost an untouched operand: %+v", row)
					}
				}
				return
			}
			if len(rows) != len(paths) || rows[0].Error != "" || rows[1].Error == "" || exists(paths[0]) {
				t.Fatalf("burial batch lost per-input result: %+v %v", rows, err)
			}
			for i, path := range paths {
				if rows[i].Ref != path {
					t.Fatalf("burial batch reordered inputs: %+v", rows)
				}
			}
			if stage == "inspect" {
				if err != nil || !exists(paths[1]) || exists(paths[2]) || rows[2].Action != core.ActionBuried {
					t.Fatalf("inspection failure stopped later burial: %+v %v", rows, err)
				}
				return
			}
			if err == nil || !exists(paths[2]) || rows[2].Action != core.ActionNotExecuted || rows[2].Error != "not executed: an earlier operation failed" {
				t.Fatalf("ledger failure did not stop later burial: %+v %v", rows, err)
			}
			if stage == "recycle-commit" && (exists(paths[1]) || rows[1].Action != core.ActionRecycled) {
				t.Fatalf("recycle commit failure lost completed move: %+v %v", rows, err)
			}
			if stage == "settle" {
				events, historyErr := f.svc.Store.History(10)
				if historyErr != nil || len(events) != 1 || events[0].Action != core.ActionBuried || events[0].Path != paths[0] {
					t.Fatalf("ledger failure recorded an uncommitted outcome: %+v %v", events, historyErr)
				}
			}
		})
	}
}

func TestBuryCancellationFinishesReservedItem(t *testing.T) {
	for _, mode := range []string{core.ModeGraveyard, core.ModeRecycle} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			f.svc.Mode = mode
			first, later := f.file("first.txt"), f.file("later.txt")
			f.add(first, time.Hour)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			f.fs.onRecycle = func(string) { cancel() }
			rows, err := f.svc.WithContext(ctx).Bury([]string{first, later}, false, "")
			if !errors.Is(err, context.Canceled) || len(rows) != 2 || rows[0].Error != "" || rows[1].Action != core.ActionNotExecuted || exists(first) || !exists(later) || len(f.items()) != 0 {
				t.Fatalf("cancellation lost reserved burial or moved later item: %+v %v", rows, err)
			}
		})
	}
}

func TestBuryCanceledWaitKeepsAllReceipts(t *testing.T) {
	f := newFixture(t)
	paths := []string{f.file("first.txt"), f.file("later.txt")}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	rows, err := f.svc.WithContext(ctx).Bury(paths, false, "")
	if !errors.Is(err, context.Canceled) || len(rows) != len(paths) {
		t.Fatalf("canceled burial lost batch error: %+v %v", rows, err)
	}
	for i, row := range rows {
		if row.Ref != paths[i] || row.Action != core.ActionNotExecuted || row.Error != "not executed: context canceled" || !exists(paths[i]) {
			t.Fatalf("canceled burial lost unexecuted receipt: %+v", row)
		}
	}
}

func TestBuryBlocksUnverifiedRoot(t *testing.T) {
	f := newFixture(t)
	buryOne(t, f, "old.txt")
	path := f.file("new.txt")
	before, err := f.svc.Store.ReadSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	f.svc.FS = rootFailureFS{testFS: f.fs, root: core.GraveyardRoot(f.svc, path)}
	rows, err := f.svc.Bury([]string{path}, false, "")
	after, readErr := f.svc.Store.ReadSnapshot()
	if err != nil || readErr != nil || len(rows) != 1 || rows[0].Action != core.ActionBlocked || !strings.Contains(rows[0].Error, "temporarily inaccessible") || !exists(path) || before.NextID != after.NextID {
		t.Fatalf("unverified root allowed burial reservation: %+v %v %v", rows, err, readErr)
	}
}

func TestBuryBlocksUnverifiedManifestIdentity(t *testing.T) {
	f := newFixture(t)
	path := f.file("pending.txt")
	f.add(path, time.Hour)
	_, grave := interruptBurial(t, f, path, "bury")
	manifest := filepath.Dir(grave) + ".json"
	data := read(t, manifest)
	if err := os.WriteFile(manifest, []byte(data+"{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(grave, path); err != nil {
		t.Fatal(err)
	}
	later := f.file("later.txt")
	rows, err := f.svc.Bury([]string{path, later}, false, "")
	if err != nil || len(rows) != 2 || rows[0].Action != core.ActionRetry || rows[0].Error != "unverifiable burial manifest; see obit doctor" || !exists(path) || !exists(manifest) || rows[1].Action != core.ActionBuried || exists(later) {
		t.Fatalf("unverified manifest identity allowed burial or stopped later item: %+v %v", rows, err)
	}
}

func TestBuryOmittedRecoveryPointsToAudit(t *testing.T) {
	testRecoveryLogAdvice(t, 0)
}

func TestRecoveryLogAdviceWithOldOutbox(t *testing.T) {
	testRecoveryLogAdvice(t, 30)
}

func testRecoveryLogAdvice(t *testing.T, backlog int) {
	t.Helper()
	f := newFixture(t)
	parent, err := f.fs.IdentifyFollowing(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	var receipts []map[string]any
	for i := range 21 {
		path := f.file(fmt.Sprintf("restored-%d.txt", i))
		id, err := f.fs.Identify(path)
		if err != nil {
			t.Fatal(err)
		}
		dir := filepath.Join(f.svc.Store.Home, "graveyard", fmt.Sprintf("%d-20261107T120000Z-abcdef", i+1))
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		graveID, err := f.fs.Identify(dir)
		if err != nil {
			t.Fatal(err)
		}
		b := core.Burial{ID: int64(i + 1), Path: path, Grave: filepath.Join(dir, filepath.Base(path)), Identity: &id, GraveID: &graveID}
		receipts = append(receipts, map[string]any{"kind": core.ActionResurrected, "burial": b, "path": path, "resurrect": map[string]any{"parent": parent, "name": filepath.Base(path)}})
	}
	encoded, err := json.Marshal(receipts)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Store.Update(func(st *core.State) error { st.NextID = 22; return json.Unmarshal(encoded, &st.RestoreCleanups) }); err != nil {
		t.Fatal(err)
	}
	// Stage an old committed outbox without delivering it before this batch.
	if err := f.svc.Store.UpdateHot(func(st *core.State) error {
		for i := range backlog {
			st.PendingAudit = append(st.PendingAudit, core.Event{Time: f.clock, Action: core.ActionPurged, ID: int64(1000 + i), Path: "old operation"})
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.Bury([]string{f.file("new.txt")}, false, "")
	if err != nil || len(rows) != 21 || rows[0].RecoveryOmitted != 1 || !strings.Contains(rows[0].Warning, "obit log --lines ") {
		t.Fatalf("omitted recovery points to vanished evidence: %+v %v", rows, err)
	}
	_, advice, _ := strings.Cut(rows[0].Warning, "obit log --lines ")
	var lines int
	if _, err := fmt.Sscanf(advice, "%d", &lines); err != nil {
		t.Fatal(err)
	}
	events, err := f.svc.Store.History(lines)
	recovered := map[int64]bool{}
	for _, event := range events {
		if event.Action == core.ActionResurrected {
			recovered[event.ID] = true
		}
	}
	if err != nil || len(recovered) != 21 {
		t.Fatalf("recovery log advice missed receipts behind old outbox: lines=%d recovered=%d error=%v", lines, len(recovered), err)
	}
	view, err := f.svc.ReadGraveyard()
	if err != nil || len(view.Restored) != 0 || len(view.Pending) != 0 {
		t.Fatalf("fixture recovery did not finish: %+v %v", view, err)
	}
	events, err = f.svc.Store.History(lines)
	if err != nil || len(events) != 22+backlog {
		t.Fatalf("recommended audit count cannot find the recovery: %+v %v", events, err)
	}
	for _, event := range events[:21] {
		if event.Action != core.ActionResurrected {
			t.Fatal("recovery audit missing", event)
		}
	}
}

func TestBuryAndResurrectUseSameVolumeSpelling(t *testing.T) {
	f := newFixture(t)
	path := f.file("payload.txt")
	unc := `\\localhost\` + strings.TrimSuffix(filepath.VolumeName(path), ":") + "$" + path[len(filepath.VolumeName(path)):]
	if _, err := os.Stat(unc); err != nil {
		t.Skipf("existing administrative share unavailable: %v", err)
	}
	want := filepath.Join(f.svc.Store.Home, "graveyard")
	f.svc.Store.Home = `\\localhost\` + strings.TrimSuffix(filepath.VolumeName(path), ":") + "$" + f.svc.Store.Home[2:]
	// Even accidental execution under a location mutant must never write a
	// volume-root grave. The mutation table uses the separate location test.
	alias := `\\localhost\` + strings.TrimSuffix(filepath.VolumeName(path), ":") + "$" + want[2:]
	if got := core.GraveyardRoot(f.svc, unc); !samePath(got, alias) {
		t.Fatalf("refusing unsafe test graveyard: %s", got)
	}
	rows, err := f.svc.Bury([]string{unc}, false, "")
	if err != nil || core.GraveFailed(rows) || len(rows) != 1 || rows[0].Burial == nil {
		t.Fatalf("volume alias burial did not use Home: %+v %v", rows, err)
	}
	actualID, actualErr := f.fs.IdentifyFollowing(filepath.Dir(filepath.Dir(rows[0].Burial.Grave)))
	wantID, wantErr := f.fs.IdentifyFollowing(want)
	if actualErr != nil || wantErr != nil || actualID.Serial != wantID.Serial || actualID.FileID != wantID.FileID {
		t.Fatalf("alias grave is outside physical Home: %v %v", actualErr, wantErr)
	}
	if exists(path) || read(t, rows[0].Burial.Grave) != "payload.txt" {
		t.Fatal("volume alias burial lost content")
	}
	if restored, err := f.svc.Resurrect([]string{fmt.Sprint(rows[0].Burial.ID)}); err != nil || core.GraveFailed(restored) || !exists(path) {
		t.Fatalf("alias grave could not be restored: %+v %v", restored, err)
	}
}

func TestBuryRejectsTrailingSpaceAncestor(t *testing.T) {
	f := newFixture(t)
	neighbor := filepath.Join(f.dir, "Long Directory Name")
	parent := neighbor + " "
	for _, dir := range []string{neighbor, `\\?\` + parent} {
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(parent, "payload.txt")
	if err := os.WriteFile(`\\?\`+path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.Bury([]string{path}, false, "")
	if err == nil && !core.GraveFailed(rows) {
		if len(rows) == 1 && rows[0].Burial != nil {
			restored, restoreErr := f.svc.Resurrect([]string{fmt.Sprint(rows[0].Burial.ID)})
			t.Fatalf("unstable ancestor allowed burial and restoration into neighbor: %+v %v", restored, restoreErr)
		}
		t.Fatalf("unstable ancestor allowed burial: %+v", rows)
	}
	if read(t, `\\?\`+path) != "keep" || exists(filepath.Join(neighbor, "payload.txt")) || len(f.burials()) != 0 {
		t.Fatal("rejected unstable path changed either directory")
	}
}

func TestConfirmedBurialReceiptSurvivesAccountingFailure(t *testing.T) {
	testConfirmedBurialReceipt(t, "exact")
}

func TestConfirmedBurialReceiptMatchesRequest(t *testing.T) {
	for _, mode := range []string{"replacement", "uppercase", "directory", "file-separator"} {
		t.Run(mode, func(t *testing.T) { testConfirmedBurialReceipt(t, mode) })
	}
}

func testConfirmedBurialReceipt(t *testing.T, mode string) {
	core.RequireFsync(t)
	t.Helper()
	f := newFixture(t)
	path := f.file("payload.txt")
	if mode == "directory" {
		path = filepath.Dir(f.file("directory/keep.txt"))
	}
	stop := core.SetBurialStep(f.svc, func(stage string) error {
		if stage == "committed" {
			return errors.New("interrupted before receipt")
		}
		return nil
	})
	defer stop()
	if _, err := f.svc.Bury([]string{path}, false, ""); err == nil {
		t.Fatal("fixture did not interrupt burial")
	}
	stop()
	b := f.burials()[0]
	ref := path
	switch mode {
	case "replacement":
		if err := os.WriteFile(path, []byte("new object"), 0600); err != nil {
			t.Fatal(err)
		}
	case "uppercase":
		ref = strings.ToUpper(path)
	case "directory", "file-separator":
		ref += `\`
	}
	accountingFailed := false
	fault := errors.New("accounting flush failed")
	reset := core.SetReplacementSync(func(file *os.File) error {
		if filepath.Base(file.Name()) == "store.json.tmp" {
			// Observe the new ledger contents at the durability boundary. An
			// outbox acknowledgement may also write the ledger, so ordinal
			// write counts do not identify the accounting operation.
			state, err := f.svc.Store.Inspect()
			if err != nil {
				t.Fatalf("read replacement: %v", err)
			}
			if len(state.Burials) == 1 && state.Burials[0].Size > 0 {
				accountingFailed = true
				return fault
			}
		}
		return file.Sync()
	})
	defer reset()
	rows, err := f.svc.Bury([]string{ref}, false, "")
	if !errors.Is(err, fault) || !accountingFailed {
		t.Fatalf("accounting failure was not reached: %v", err)
	}
	if mode == "replacement" || mode == "file-separator" {
		if len(rows) != 2 || rows[0].Action != core.ActionNotExecuted || rows[1].Action != core.ActionBuried || rows[1].Ref != fmt.Sprintf("#%d", b.ID) {
			t.Fatalf("recovery receipt falsely acknowledged current request: %+v", rows)
		}
		if mode == "replacement" && read(t, path) != "new object" {
			t.Fatal("replacement changed")
		}
	} else if len(rows) != 1 || rows[0].Burial == nil || rows[0].Burial.ID != b.ID || rows[0].Action != core.ActionBuried || rows[0].Error != "" {
		t.Fatalf("accounting failure swallowed confirmed burial receipt: %+v %v", rows, err)
	}
	if exists(filepath.Dir(b.Grave)+".json") || !exists(b.Grave) || len(f.burials()) != 1 {
		t.Fatal("confirmed recovery lost the burial or retained its manifest")
	}
}

func TestBuryRetainsManifestAfterPostRenameSyncFailure(t *testing.T) {
	core.RequireFsync(t)
	f := newFixture(t)
	path := f.file("payload.txt")
	moved := false
	step := core.SetBurialStep(f.svc, func(stage string) error {
		if stage == "moved" {
			moved = true
		}
		return nil
	})
	defer step()
	stop := core.SetReplacementSync(func(file *os.File) error {
		if moved && filepath.Base(file.Name()) == "store.json.tmp" {
			return errors.New("injected post-rename sync failure")
		}
		return file.Sync()
	})
	defer stop()
	rows, err := f.svc.Bury([]string{path}, false, "")
	if err == nil || !strings.Contains(err.Error(), "commit unconfirmed") || len(rows) != 1 || rows[0].Action == core.ActionBuried {
		t.Fatalf("burial claimed durable commit after sync failure: %+v %v", rows, err)
	}
	graves := f.burials()
	if len(graves) != 1 || !exists(graves[0].Grave) || !exists(filepath.Dir(graves[0].Grave)+".json") {
		t.Fatal("unconfirmed burial lost recovery manifest")
	}
	if _, err := f.svc.Sweep(false); err == nil || !exists(filepath.Dir(graves[0].Grave)+".json") {
		t.Fatal("recovery discarded unconfirmed commit evidence before a successful flush")
	}
	stop()
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	if !exists(graves[0].Grave) || exists(filepath.Dir(graves[0].Grave)+".json") {
		t.Fatal("confirmed recovery did not converge")
	}
	events, err := f.svc.Store.History(20)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range events {
		found = found || e.Action == core.ActionBuried && e.ID == graves[0].ID && strings.Contains(e.Detail, "confirmed")
	}
	if !found {
		t.Fatalf("recovery lost confirmed burial audit: %+v", events)
	}
}

func TestBuryRetryAcknowledgesPostRenameCommit(t *testing.T) {
	core.RequireFsync(t)
	f := newFixture(t)
	path := f.file("payload.txt")
	moved := false
	step := core.SetBurialStep(f.svc, func(stage string) error { moved = moved || stage == "moved"; return nil })
	defer step()
	stop := core.SetReplacementSync(func(file *os.File) error {
		if moved && filepath.Base(file.Name()) == "store.json.tmp" {
			return errors.New("injected post-rename sync failure")
		}
		return file.Sync()
	})
	defer stop()
	if _, err := f.svc.Bury([]string{path}, false, ""); err == nil {
		t.Fatal("fixture did not interrupt commit")
	}
	b := f.burials()[0]
	stop()
	rows, err := f.svc.Bury([]string{path}, false, "")
	st, readErr := f.svc.Store.ReadSnapshot()
	if err != nil || core.GraveFailed(rows) || len(rows) != 1 || rows[0].Burial == nil || rows[0].Burial.ID != b.ID || rows[0].Action != core.ActionBuried || readErr != nil || st.Stats.Buried != 1 || len(st.Burials) != 1 || !exists(b.Grave) {
		t.Fatalf("bury retry lost confirmed recovery receipt: %+v %v %+v", rows, err, st.Stats)
	}
}

func TestBurialRecoveryShortReference(t *testing.T) {
	f := newFixture(t)
	path := f.file("Directory With A Long Name/payload.txt")
	short := shortPath(t, path)
	stop := core.SetBurialStep(f.svc, func(step string) error {
		if step == "moved" {
			return errors.New("interrupted burial")
		}
		return nil
	})
	rows, err := f.svc.Bury([]string{path}, false, "")
	stop()
	if err == nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
	rows, err = f.svc.Bury([]string{short}, false, "")
	if err != nil || core.GraveFailed(rows) || rows[0].Action != core.ActionBuried || len(f.burials()) != 1 {
		t.Fatalf("short retry lost recovered burial receipt: %+v %v", rows, err)
	}
}

func TestBuryRejectsOversizedNote(t *testing.T) {
	f := newFixture(t)
	path := f.file("note.txt")
	before, err := f.svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Bury([]string{path}, false, strings.Repeat("n", 4097)); err == nil {
		t.Fatal("oversized burial note accepted")
	}
	after, err := f.svc.Store.Read()
	if err != nil || !reflect.DeepEqual(before, after) || !exists(path) {
		t.Fatalf("rejected note changed burial state: %v", err)
	}
}

func TestBuryTakesOverATrackedItem(t *testing.T) {
	f := newFixture(t)
	dir := filepath.Join(f.dir, "scratch")
	f.file("scratch/a/b.txt")
	item := f.add(dir, 24*time.Hour)

	results, err := f.svc.Bury([]string{dir}, false, "")
	if err != nil || results[0].Error != "" {
		t.Fatalf("bury: %v %+v", err, results)
	}
	graves := f.burials()
	if exists(dir) || len(f.items()) != 0 || len(graves) != 1 || graves[0].ID != item.ID || graves[0].Reason != core.ReasonBury {
		t.Fatalf("want the tracked folder buried under the same number, got %+v", graves)
	}
	if !exists(filepath.Join(graves[0].Grave, "a", "b.txt")) || graves[0].Size != int64(len("scratch/a/b.txt")) {
		t.Fatalf("the folder's contents must be buried intact, got %+v", graves[0])
	}
}

func TestBuryAndSweepShareLock(t *testing.T) {
	if testing.Short() {
		t.Skip("waits for the full sweep lock timeout")
	}
	for _, first := range []string{"bury", "sweep"} {
		t.Run(first, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("debug.log")
			item := f.add(path, time.Hour)
			f.clock = f.clock.Add(time.Hour)
			var overlap error
			f.svc.FS = &moveHookFS{testFS: f.fs, before: func() {
				other := *f.svc
				if first == "bury" {
					_, overlap = other.Sweep(false)
				} else {
					_, overlap = other.Bury([]string{path}, false, "")
				}
			}}
			var err error
			if first == "bury" {
				_, err = f.svc.Bury([]string{path}, false, "")
			} else {
				_, err = f.svc.Sweep(false)
			}
			if err != nil {
				t.Fatal(err)
			}
			if !errors.Is(overlap, core.ErrSweepBusy) {
				t.Errorf("overlapping operation must retry: %v", overlap)
			}
			st, err := f.svc.Store.Read()
			if err != nil || len(st.Burials) != 1 || len(st.Items) != 0 || st.Burials[0].ID != item.ID {
				t.Fatalf("unreadable or duplicate burial: %+v, %v", st, err)
			}
			entries, err := os.ReadDir(core.GraveyardRoot(f.svc, path))
			if err != nil || len(entries) != 1 || read(t, st.Burials[0].Grave) != "debug.log" {
				t.Fatalf("object must have one grave: %v, %v", entries, err)
			}
		})
	}
}

func TestBurialCommitNeverDuplicatesID(t *testing.T) {
	for _, operation := range []string{"bury", "sweep"} {
		t.Run(operation, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("debug.log")
			item := f.add(path, time.Hour)
			f.clock = f.clock.Add(time.Hour)
			done := make(chan struct{})
			f.svc.FS = &moveHookFS{testFS: f.fs, after: func(grave string) {
				// A second public burial waits behind the first transaction.
				// It must not create another grave or corrupt the shared ID.
				go func() {
					defer close(done)
					_, _ = f.svc.Bury([]string{path}, false, "")
				}()
			}}
			if operation == "bury" {
				_, _ = f.svc.Bury([]string{path}, false, "")
			} else {
				_, _ = f.svc.Sweep(false)
			}
			<-done
			st, err := f.svc.Store.Read()
			if err != nil || len(st.Burials) != 1 || len(st.Items) != 0 || st.Burials[0].ID != item.ID || read(t, st.Burials[0].Grave) != "debug.log" {
				t.Fatalf("duplicate commit corrupted ledger: %+v, %v", st, err)
			}
		})
	}
}

func TestClaimedBurialHonorsExtendAndSpare(t *testing.T) {
	for _, operation := range []string{"sweep", "bury"} {
		for _, mutation := range []string{"extend", "spare"} {
			t.Run(operation+"/"+mutation, func(t *testing.T) {
				f := newFixture(t)
				path := f.file("keep.log")
				item := f.add(path, time.Hour)
				f.clock = f.clock.Add(time.Hour)
				var changed core.Item
				f.svc.FS = &prepareHookFS{testFS: f.fs, before: func() {
					var rows []core.Result
					var err error
					ref := []string{fmt.Sprint(item.ID)}
					if mutation == "extend" {
						rows, err = f.svc.Extend(ref, 30*24*time.Hour)
					} else {
						rows, err = f.svc.Cancel(ref)
					}
					if err != nil || core.Failed(rows) {
						t.Fatalf("concurrent mutation: %+v, %v", rows, err)
					}
					changed = *rows[0].Item
				}}
				if operation == "sweep" {
					_, _ = f.svc.Sweep(false)
				} else {
					_, _ = f.svc.Bury([]string{path}, false, "")
				}
				if !exists(path) || len(f.burials()) != 0 {
					t.Fatal("a successful extend/spare must prevent the claimed move")
				}
				items := f.items()
				if mutation == "spare" {
					if len(items) != 0 {
						t.Fatalf("spared item reappeared: %+v", items)
					}
				} else if len(items) != 1 || !reflect.DeepEqual(items[0], changed) {
					t.Fatalf("new lifespan was overwritten: %+v; want %+v", items, changed)
				}
			})
		}
	}
}

func TestBuryFailurePreservesRegistration(t *testing.T) {
	for _, mode := range []string{core.ModeGraveyard, core.ModeRecycle} {
		for _, tracked := range []bool{false, true} {
			for _, kind := range []string{"retry", "unsafe", "refused"} {
				t.Run(fmt.Sprintf("%s/tracked=%v/%s", mode, tracked, kind), func(t *testing.T) {
					f := newFixture(t)
					f.svc.Mode = mode
					path := f.file("blocked.txt")
					var before core.Item
					if tracked {
						before = f.add(path, 30*24*time.Hour)
					}
					f.fs.fail = errors.New("sharing violation")
					switch kind {
					case "unsafe":
						f.fs.fail = core.ErrUnsafe
					case "refused":
						f.fs.fail = &core.RefusedError{Path: path, Reason: "protected"}
					}
					got, err := f.svc.Bury([]string{path}, false, "new note")
					items := f.items()
					if err != nil || !core.GraveFailed(got) || !exists(path) {
						t.Fatalf("burial failure: %+v, %v", got, err)
					}
					if tracked {
						if len(items) != 1 {
							t.Fatalf("lost preexisting registration: %+v", items)
						}
						if !reflect.DeepEqual(before, items[0]) {
							t.Fatalf("refusal shortened existing lifespan: %+v; want %+v", items[0], before)
						}
					} else if len(items) != 0 {
						t.Fatalf("refusal left a due registration: %+v", items)
					}
				})
			}
		}
	}
}

func TestBuryWaitsForActiveSweep(t *testing.T) {
	f := newFixture(t)
	path := f.file("waiting.txt")
	if err := os.MkdirAll(f.svc.Store.Home, 0o700); err != nil {
		t.Fatal(err)
	}
	release, ok, err := core.TryLockExclusive(filepath.Join(f.svc.Store.Home, "sweep.lock"))
	if err != nil || !ok {
		t.Fatalf("hold sweep lock: %v", err)
	}
	done := make(chan struct{})
	go func() {
		time.Sleep(200 * time.Millisecond)
		release()
		close(done)
	}()
	got, err := f.svc.Bury([]string{path}, false, "")
	<-done
	if err != nil || core.GraveFailed(got) || exists(path) || len(f.burials()) != 1 {
		t.Fatalf("bury must wait for a short sweep: %+v, %v", got, err)
	}
}

func TestBurialMoveSerializesLifespanMutation(t *testing.T) {
	f := newFixture(t)
	path := f.file("original.txt")
	item := f.add(path, time.Hour)
	f.clock = f.clock.Add(time.Hour)
	reached, release := make(chan struct{}), make(chan struct{})
	f.svc.FS = blockingMoveFS{testFS: f.fs, reached: reached, release: release}
	swept := make(chan error, 1)
	go func() {
		_, err := f.svc.Sweep(false)
		swept <- err
	}()
	<-reached
	type mutation struct {
		rows []core.Result
		err  error
	}
	mutated := make(chan mutation, 1)
	go func() {
		rows, err := f.svc.Extend([]string{fmt.Sprint(item.ID)}, 24*time.Hour)
		mutated <- mutation{rows, err}
	}()
	select {
	case got := <-mutated:
		t.Errorf("mutation completed inside the burial transaction: %+v", got)
		close(release)
		<-swept
		return
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	if err := <-swept; err != nil {
		t.Fatal(err)
	}
	got := <-mutated
	if got.err != nil || !core.Failed(got.rows) || len(f.burials()) != 1 || len(f.items()) != 0 {
		t.Fatalf("mutation must observe a completed burial: %+v", got)
	}
}

func TestUntrackedBuryHonorsConcurrentAdd(t *testing.T) {
	f := newFixture(t)
	path := f.file("keep.txt")
	var added core.Item
	t.Cleanup(core.SetAfterManifest(f.svc, func() { added = f.add(path, 30*24*time.Hour) }))
	got, err := f.svc.Bury([]string{path}, false, "")
	if err != nil || len(got) != 1 || got[0].Action != core.ActionSkipped || !core.GraveFailed(got) || read(t, path) != "keep.txt" || len(f.burials()) != 0 || len(f.items()) != 1 || !reflect.DeepEqual(f.items()[0], added) {
		t.Fatalf("concurrent registration did not supersede reserved burial: %+v, %v", got, err)
	}
}

func TestExplicitBuryIdentitySwapDoesNotRegister(t *testing.T) {
	for _, tracked := range []bool{false, true} {
		t.Run(fmt.Sprint(tracked), func(t *testing.T) {
			f := newFixture(t)
			path := f.file("keep.txt")
			if tracked {
				f.add(path, 30*24*time.Hour)
			}
			before := f.items()
			backup := path + ".saved"
			f.fs.onRecycle = func(string) {
				if err := os.Rename(path, backup); err != nil {
					t.Fatal(err)
				}
				f.file("keep.txt")
			}
			got, err := f.svc.Bury([]string{path}, false, "")
			after := f.items()
			if err != nil || !core.GraveFailed(got) || read(t, backup) != "keep.txt" || read(t, path) != "keep.txt" || len(f.burials()) != 0 || len(before) != len(after) || tracked && !reflect.DeepEqual(before, after) {
				t.Fatalf("identity swap moved a replacement or changed registration: %+v, %v", got, err)
			}
		})
	}
}

func TestExplicitBuryFailureLeavesLedgerUnchanged(t *testing.T) {
	for _, tracked := range []bool{false, true} {
		for _, cause := range []error{errors.New("sharing violation"), core.ErrUnsafe} {
			t.Run(fmt.Sprintf("tracked=%v/%v", tracked, cause), func(t *testing.T) {
				f := newFixture(t)
				path := f.file("keep.txt")
				if tracked {
					f.add(path, 30*24*time.Hour)
				}
				before := f.items()
				f.fs.fail = cause
				got, err := f.svc.Bury([]string{path}, false, "new note")
				if err == nil && !core.GraveFailed(got) {
					t.Fatalf("failure was hidden: %+v", got)
				}
				if !exists(path) || (len(before) != len(f.items()) || (tracked && !reflect.DeepEqual(before, f.items()))) || len(f.burials()) != 0 {
					t.Fatalf("failed explicit burial changed registration: before=%+v after=%+v", before, f.items())
				}
			})
		}
	}
}

func TestParentBurialAbsorbsChildLifespans(t *testing.T) {
	for _, earlier := range []bool{false, true} {
		for _, interrupted := range []bool{false, true} {
			t.Run(fmt.Sprintf("earlier=%v/interrupted=%v", earlier, interrupted), func(t *testing.T) {
				f := newFixture(t)
				path := f.file("output/keep.txt")
				parent := f.add(filepath.Dir(path), time.Hour)
				life := time.Hour
				if earlier {
					life = time.Minute
				}
				child := f.add(path, life)
				f.clock = f.clock.Add(time.Hour)
				stop := errors.New("interrupted after move")
				restore := core.SetBurialStep(f.svc, func(step string) error {
					if interrupted && step == "moved" {
						return stop
					}
					return nil
				})
				t.Cleanup(restore)
				_, err := f.svc.Bury([]string{parent.Path}, false, "")
				restore()
				if interrupted {
					if !errors.Is(err, stop) {
						t.Fatalf("interruption: %v", err)
					}
					pending, err := f.svc.PendingGraves()
					if err != nil || len(pending) != 1 || !reflect.DeepEqual(containsIDs(t, pending[0].Burial), []int64{child.ID}) {
						t.Fatalf("manifest lost contained child IDs: %+v, %v", pending, err)
					}
					_, err = f.svc.Sweep(false)
					if err != nil {
						t.Fatal(err)
					}
				} else if err != nil {
					t.Fatal(err)
				}
				burials := f.burials()
				if len(f.items()) != 0 || len(burials) != 1 || !reflect.DeepEqual(containsIDs(t, burials[0]), []int64{child.ID}) {
					t.Fatalf("child lifespan survived parent burial: items=%+v burials=%+v", f.items(), burials)
				}
				rows, err := f.svc.Resurrect([]string{fmt.Sprint(parent.ID)})
				if err != nil || core.GraveFailed(rows) || read(t, path) != "output/keep.txt" || len(f.items()) != 0 {
					t.Fatalf("resurrection must restore contents without rearming expired children: %+v, %v", rows, err)
				}
			})
		}
	}
}
