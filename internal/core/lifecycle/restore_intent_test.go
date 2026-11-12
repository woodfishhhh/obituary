//go:build windows

package lifecycle_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/watch"
	"golang.org/x/sys/windows"
)

func TestRestoreIntentRejectsInvalidNameBeforeWrite(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	parent, err := f.fs.OpenRestoreParent(filepath.Dir(b.Path))
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	for _, name := range []string{strings.Repeat("a", 256), "../outside", "nul", "trailing.", "control\n.txt"} {
		if err := core.WriteRestoreIntent(b, parent, name); err == nil || exists(filepath.Dir(b.Grave)+".json") {
			t.Fatalf("invalid destination was written to manifest: %q %v", name, err)
		}
	}
}

func TestInvalidRestoreNameRetriesOnlyWithIntactGrave(t *testing.T) {
	for _, name := range []string{strings.Repeat("a", 243) + ".txt", strings.Repeat("😀", 121) + ".txt"} {
		for _, intact := range []bool{true, false} {
			t.Run(fmt.Sprintf("%d/%v", len(name), intact), func(t *testing.T) {
				f := newFixture(t)
				b := buryOne(t, f, name)
				parent, err := f.fs.OpenRestoreParent(filepath.Dir(b.Path))
				if err != nil {
					t.Fatal(err)
				}
				defer parent.Close()
				if err := core.WriteRestoreIntent(b, parent, name); err != nil {
					t.Fatal(err)
				}
				rewriteManifest(t, filepath.Dir(b.Grave)+".json", func(m map[string]any) {
					m["resurrect"].(map[string]any)["name"] = strings.TrimSuffix(name, ".txt") + " (resurrected).txt"
				})
				if err := os.WriteFile(b.Path, []byte("occupied"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(b.Path, filepath.Join(f.dir, "placeholder.txt")); err != nil {
					t.Fatal(err)
				}
				if !intact {
					if err := os.Rename(b.Grave, filepath.Join(f.dir, "elsewhere.txt")); err != nil {
						t.Fatal(err)
					}
				}
				rows, err := f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
				if intact {
					if err != nil || core.GraveFailed(rows) || rows[0].Action != core.ActionResurrected || read(t, b.Path) != name {
						t.Fatalf("intact grave could not discard invalid old name: %+v %v", rows, err)
					}
				} else if err == nil && !core.GraveFailed(rows) || !exists(filepath.Dir(b.Grave)+".json") || len(f.burials()) != 1 {
					t.Fatalf("invalid moved intent accepted: %+v %v", rows, err)
				}
			})
		}
	}
}

func TestRestoreIntentWriteFailurePreservesGrave(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	blocked := filepath.Dir(b.Grave) + ".json.tmp"
	if err := os.Mkdir(blocked, 0o700); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || !core.GraveFailed(got) || !exists(b.Grave) || exists(b.Path) || len(f.burials()) != 1 {
		t.Fatalf("restore moved without durable intent: %+v %v", got, err)
	}
}

func TestAbandonConfirmationRejectsOversizedAuditIntent(t *testing.T) {
	for _, receiptOnly := range []bool{false, true} {
		t.Run(fmt.Sprint(receiptOnly), func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "payload.txt")
			_, unlock := interruptResurrection(t, f, b)
			unlock()
			manifest := filepath.Dir(b.Grave) + ".json"
			rewriteManifest(t, manifest, func(m map[string]any) {
				m["resurrect"].(map[string]any)["name"] = strings.Repeat("\x7f", 300000)
			})
			var receipt core.RestoreCleanup
			if err := json.Unmarshal([]byte(read(t, manifest)), &receipt); err != nil {
				t.Fatal(err)
			}
			receipt.Kind, receipt.Burial, receipt.CommittedAt = core.ActionAbandoned, b, f.clock
			if err := f.svc.Store.Update(func(st *core.State) error {
				st.Burials = nil
				st.RestoreCleanups = []core.RestoreCleanup{receipt}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if receiptOnly {
				if err := os.Remove(manifest); err != nil {
					t.Fatal(err)
				}
			}
			before := read(t, filepath.Join(f.svc.Store.Home, "audit.jsonl"))
			view, err := f.svc.ReadGraveyard()
			if err != nil || len(view.Pending) != 1 || view.Pending[0].Status != "unverifiable" || !strings.Contains(view.Pending[0].Error, "invalid destination identity or name") || read(t, filepath.Join(f.svc.Store.Home, "audit.jsonl")) != before {
				t.Fatalf("oversized restore name reached confirmation audit: pending=%+v err=%v", view.Pending, err)
			}
			if _, err := f.svc.Store.History(20); err != nil {
				t.Fatalf("oversized name corrupted log reading: %v", err)
			}
		})
	}
}

func TestAbandonConfirmationRetainsAudit(t *testing.T) {
	core.RequireFsync(t)
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	_, unlock := interruptResurrection(t, f, b)
	unlock()
	outside := filepath.Join(f.dir, "elsewhere")
	if err := os.Mkdir(outside, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(b.Path, filepath.Join(outside, "payload.txt")); err != nil {
		t.Fatal(err)
	}
	reset := core.SetReplacementSync(func(file *os.File) error {
		if filepath.Base(file.Name()) == "store.json.tmp" {
			return errors.New("abandon commit unconfirmed")
		}
		return file.Sync()
	})
	defer reset()
	if _, err := f.svc.PurgeRefs([]string{fmt.Sprint(b.ID)}, false); err == nil {
		t.Fatal("fixture did not interrupt abandon")
	}
	reset()
	before, err := f.svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(before.RestoreCleanups) != 1 {
		t.Fatal("fixture lacks abandonment evidence")
	}
	view, err := f.svc.ReadGraveyard()
	if err != nil || len(view.Restored) != 0 || len(view.Pending) != 0 {
		t.Fatalf("abandon confirmation: %+v %v", view, err)
	}
	after, err := f.svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if after.Stats != before.Stats || len(after.RestoreCleanups) != 0 {
		t.Fatal("abandon confirmation changed statistics or retained evidence")
	}
	events, err := f.svc.Store.History(20)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range events {
		if e.Action == core.ActionResurrected {
			t.Fatal("abandon recorded as resurrection")
		}
		if e.Action == core.ActionAbandoned && e.ID == b.ID && strings.Contains(e.Detail, "restore parent=") && strings.Contains(e.Detail, "planned name=\"payload.txt\"") && strings.Contains(e.Detail, "reason=") {
			found = true
		}
	}
	if !found {
		t.Fatalf("abandon confirmation lost intent audit: %+v", events)
	}
}

func TestRestoreIntentBeforeMoveCanRetry(t *testing.T) {
	for _, retry := range []string{"resurrect", "sweep"} {
		t.Run(retry, func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "payload.txt")
			f.fs.fail = errors.New("temporarily busy")
			got, err := f.svc.Resurrect([]string{b.Path})
			if err != nil || !core.GraveFailed(got) {
				t.Fatal(got, err)
			}
			pending, err := f.svc.PendingGraves()
			if err != nil || len(pending) != 1 || pending[0].Status != "restore-ready" {
				t.Fatalf("pre-move intent not diagnosable: %+v %v", pending, err)
			}
			f.fs.fail = nil
			if retry == "sweep" {
				if _, err := f.svc.Sweep(false); err != nil {
					t.Fatal(err)
				}
			}
			got, err = f.svc.Resurrect([]string{b.Path})
			if err != nil || core.GraveFailed(got) || read(t, b.Path) != "payload.txt" {
				t.Fatalf("pre-move intent blocked retry: %+v %v", got, err)
			}
		})
	}
}

func TestRestoreIntentUsesCurrentAccounting(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	_, unlock := interruptResurrection(t, f, b)
	unlock()
	if err := f.svc.Store.Update(func(st *core.State) error { st.Burials[0].Size = 123; return nil }); err != nil {
		t.Fatal(err)
	}
	got, err := f.svc.Resurrect([]string{b.Path})
	st, readErr := f.svc.Store.Read()
	if err != nil || readErr != nil || core.GraveFailed(got) || len(st.Burials) != 0 || st.Stats.ResurrectedBytes != 123 {
		t.Fatalf("concurrent accounting blocked recovery: %+v %+v %v %v", got, st, err, readErr)
	}
}

func TestRestoreIntentTamperingRetainsRecord(t *testing.T) {
	for _, field := range []string{"parent", "name", "size", "version", "grave"} {
		t.Run(field, func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "payload.txt")
			_, unlock := interruptResurrection(t, f, b)
			unlock()
			manifest := filepath.Dir(b.Grave) + ".json"
			rewriteManifest(t, manifest, func(m map[string]any) {
				switch field {
				case "parent":
					m["resurrect"].(map[string]any)["parent"] = b.Identity
				case "name":
					m["resurrect"].(map[string]any)["name"] = `..\elsewhere`
				case "size":
					m["size"] = 12345
				case "version":
					m["version"] = 1
				case "grave":
					m["grave"] = b.Path
				}
			})
			view, err := f.svc.ReadGraveyard()
			if err != nil || len(view.Pending) != 1 || view.Pending[0].Status != "unverifiable" || len(f.burials()) != 1 || read(t, b.Path) != "payload.txt" {
				t.Fatalf("damaged restore intent was accepted: %+v %v", view, err)
			}
		})
	}
}

func holdManifest(t *testing.T, path string) func() {
	t.Helper()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	release := func() {
		if h != 0 {
			windows.CloseHandle(h)
			h = 0
		}
	}
	t.Cleanup(release)
	return release
}

func TestRestoreCleanupFailureIsPerRecord(t *testing.T) {
	if testing.Short() {
		t.Skip("exercises the full batch of per-record recovery failures")
	}
	for _, phase := range []string{"restore-ready", "resurrected", "restore-committed"} {
		for _, entry := range []string{"sweep", "bury", "tick", "graveyard", "resurrect"} {
			t.Run(phase+"/"+entry, func(t *testing.T) {
				f := newFixture(t)
				b := buryOne(t, f, "payload.txt")
				other := buryOne(t, f, "purge.txt")
				manifest := filepath.Dir(b.Grave) + ".json"
				if phase == "restore-ready" {
					f.fs.fail = errors.New("busy")
					f.svc.Resurrect([]string{b.Path})
					f.fs.fail = nil
				} else {
					_, release := interruptResurrection(t, f, b)
					release()
				}
				release := holdManifest(t, manifest)
				if phase == "restore-committed" {
					f.svc.ReadGraveyard()
				}
				due := f.file("due.txt")
				f.add(due, time.Minute)
				f.clock = f.clock.Add(8 * 24 * time.Hour)
				var got []core.GraveResult
				var outcomes []core.Outcome
				var err error
				switch entry {
				case "sweep":
					outcomes, err = f.svc.Sweep(false)
				case "bury":
					got, err = f.svc.Bury([]string{due}, false, "")
				case "tick":
					outcomes, got, err = f.svc.Tick()
				case "graveyard":
					var view *core.GraveyardView
					view, err = f.svc.ReadGraveyard()
					if phase != "restore-ready" {
						for _, row := range view.Burials {
							if row.ID == b.ID {
								t.Fatal("committed restore remained in burials")
							}
						}
						if len(view.Restored) != 1 || view.Restored[0].Action != core.ActionResurrected || !samePath(view.Restored[0].Path, b.Path) {
							t.Fatalf("committed restore receipt lost: %+v", view)
						}
					}
				case "resurrect":
					got, err = f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
				}
				if err != nil {
					t.Fatalf("cleanup failure aborted operation: %v", err)
				}
				if entry == "sweep" || entry == "bury" || entry == "tick" {
					if exists(due) {
						t.Fatal("cleanup failure blocked unrelated burial")
					}
				}
				if entry == "tick" && exists(other.Grave) {
					t.Fatalf("cleanup failure blocked unrelated purge: %+v", got)
				}
				if phase != "restore-ready" && (entry == "sweep" || entry == "tick") {
					found := false
					for _, o := range outcomes {
						found = found || o.Item.ID == b.ID && o.Action == core.ActionResurrected && samePath(o.Path, b.Path)
					}
					if !found {
						t.Fatalf("committed restore outcome lost: %+v", outcomes)
					}
				}
				if phase != "restore-ready" && entry == "bury" {
					found := false
					for _, r := range got {
						found = found || r.Action == core.ActionResurrected && r.Burial.ID == b.ID && samePath(r.Path, b.Path)
					}
					if !found {
						t.Fatalf("bury recovery receipt lost: %+v", got)
					}
				}
				if phase != "restore-ready" && entry == "resurrect" && (len(got) != 1 || got[0].Action != core.ActionResurrected || !samePath(got[0].Path, b.Path)) {
					t.Fatalf("committed restore retry receipt lost: %+v", got)
				}
				release()
				if phase == "restore-ready" {
					if _, err := f.svc.Resurrect([]string{b.Path}); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := f.svc.Sweep(false); err != nil {
					t.Fatal(err)
				}
				st, err := f.svc.Store.Read()
				if err != nil || st.Stats.Resurrected != 1 {
					t.Fatalf("restore counted more than once: %+v %v", st.Stats, err)
				}
				events, err := f.svc.Store.History(100)
				count := 0
				confirmations := 0
				for _, e := range events {
					if e.Action == core.ActionResurrected && e.ID == b.ID {
						if e.Detail == "confirmed after an unconfirmed commit" {
							confirmations++
							if !samePath(e.Path, b.Path) {
								t.Fatalf("confirmation audit lost restored path: %+v", e)
							}
							continue // confirmation delivery is at least once
						}
						count++
					}
				}
				if err != nil || count != 1 || phase != "restore-ready" && confirmations == 0 || exists(manifest) {
					t.Fatalf("restore cleanup/audit not idempotent: original=%d confirmations=%d %v", count, confirmations, err)
				}
			})
		}
	}
}

func TestRestoreCommittedRequiresAllocatedUnusedID(t *testing.T) {
	for _, conflict := range []string{"item", "proposal", "unallocated"} {
		t.Run(conflict, func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "payload.txt")
			_, release := interruptResurrection(t, f, b)
			release()
			if err := f.svc.Store.Update(func(st *core.State) error {
				st.Burials = nil
				switch conflict {
				case "item":
					st.Items = append(st.Items, core.Item{ID: b.ID, Path: b.Path})
				case "proposal":
					st.Proposals = append(st.Proposals, core.Proposal{ID: b.ID})
				case "unallocated":
					st.NextID = b.ID
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			view, err := f.svc.ReadGraveyard()
			if err != nil || len(view.Pending) != 1 || view.Pending[0].Status != "unverifiable" || !strings.Contains(view.Pending[0].Error, "not a committed burial") || !exists(filepath.Dir(b.Grave)+".json") {
				t.Fatalf("unallocated or occupied ID treated as committed: %+v %v", view, err)
			}
		})
	}
}

func TestRestoreCleanupPreservesReplacementGrave(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	_, release := interruptResurrection(t, f, b)
	release()
	dir := filepath.Dir(b.Grave)
	if err := f.svc.Store.Update(func(st *core.State) error { st.Burials = nil; return nil }); err != nil {
		t.Fatal(err)
	}
	// Replace it after committed-state inspection, immediately before cleanup,
	// so this test isolates cleanup's own identity check.
	f.svc.FS = returnRestoredFS{FS: f.fs, before: func() {
		if err := os.Rename(dir, dir+"-old"); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
	}}
	f.svc.ReadGraveyard()
	if !exists(dir) || !exists(dir+".json") {
		t.Fatal("restore cleanup deleted replacement grave")
	}
}

type restoreRecheckFaultFS struct {
	core.FS
	path  string
	calls int
}

func (fs *restoreRecheckFaultFS) Identify(path string) (core.Identity, error) {
	if samePath(path, fs.path) {
		fs.calls++
		if fs.calls == 2 {
			return core.Identity{}, errors.New("second identity probe unavailable")
		}
	}
	return fs.FS.Identify(path)
}

func TestRestoreReadyRecheckFailureDoesNotAbortUnrelatedSweep(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	f.fs.fail = errors.New("move unavailable")
	if rows, err := f.svc.Resurrect([]string{b.Path}); err != nil || !core.GraveFailed(rows) {
		t.Fatal(rows, err)
	}
	f.fs.fail = nil
	due := f.file("unrelated.txt")
	f.add(due, time.Minute)
	f.clock = f.clock.Add(time.Hour)
	fault := &restoreRecheckFaultFS{FS: f.fs, path: b.Grave}
	f.svc.FS = fault
	rows, err := f.svc.Sweep(false)
	if err != nil || exists(due) || !exists(b.Grave) || !exists(filepath.Dir(b.Grave)+".json") || fault.calls < 2 {
		t.Fatalf("restore recheck aborted unrelated sweep: %+v %v; probes=%d", rows, err, fault.calls)
	}
	found := false
	for _, r := range rows {
		found = found || r.Item.ID == b.ID && r.Action == core.ActionBlocked && strings.Contains(r.Detail, "second identity probe")
	}
	if !found {
		t.Fatalf("restore recheck lost per-record warning: %+v", rows)
	}
}

func TestMissingBurialDoesNotClaimUnmovedRestore(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	f.fs.fail = errors.New("move unavailable")
	if rows, err := f.svc.Resurrect([]string{b.Path}); err != nil || !core.GraveFailed(rows) {
		t.Fatal(rows, err)
	}
	f.fs.fail = nil
	if err := f.svc.Store.Update(func(st *core.State) error { st.Burials = nil; return nil }); err != nil {
		t.Fatal(err)
	}
	view, err := f.svc.ReadGraveyard()
	if err != nil || len(view.Restored) != 0 || len(view.Pending) != 1 || view.Pending[0].Status != "unverifiable" || !strings.Contains(view.Pending[0].Error, "ledger contradicts") {
		t.Fatalf("missing burial claimed an unmoved restore: %+v %v", view, err)
	}
	rows, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || !core.GraveFailed(rows) || rows[0].Action == core.ActionResurrected || !exists(b.Grave) || !exists(filepath.Dir(b.Grave)+".json") {
		t.Fatalf("missing burial retry claimed an unmoved restore: %+v %v", rows, err)
	}
}

func TestWatchRestoreRecoveryCommitsExemption(t *testing.T) {
	f, path, _ := watchFixture(t, "keep.txt", core.WatchModified)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	f.sweepAt(2 * time.Hour)
	b := f.burials()[0]
	_, unlock := interruptResurrection(t, f, b)
	unlock()
	if _, err := f.svc.ReadGraveyard(); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.Read()
	if err != nil || len(st.Burials) != 0 || len(st.WatchExemptions) != 1 {
		t.Fatalf("restore recovery lost exemption: %+v %v", st, err)
	}
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	if len(f.items()) != 0 || !exists(path) {
		t.Fatal("recovered restore registered again")
	}
}

func TestWatchRestoreExemptionAtomicWithReceipt(t *testing.T) {
	f, _, _ := watchFixture(t, "keep.txt", core.WatchFirstSeen)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	f.sweepAt(2 * time.Hour)
	b := f.burials()[0]
	sawReceipt := false
	reset := core.SetReplacementSync(func(file *os.File) error {
		if filepath.Base(file.Name()) != "store.json.tmp" {
			return file.Sync()
		}
		st, err := f.svc.Store.Inspect()
		if err != nil {
			return err
		}
		if len(st.RestoreCleanups) > 0 {
			sawReceipt = true
			if len(st.WatchExemptions) != 1 || len(st.Burials) != 0 {
				t.Fatal("restore receipt committed without exemption")
			}
		}
		return file.Sync()
	})
	defer reset()
	rows, err := f.svc.Resurrect([]string{b.Path})
	if err != nil || core.GraveFailed(rows) || !sawReceipt {
		t.Fatalf("restore receipt: %+v %v", rows, err)
	}
}
