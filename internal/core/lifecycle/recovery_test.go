package lifecycle_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

func TestRecoveryCommitFailureStopsBatch(t *testing.T) {
	for _, mode := range []string{"pending-write", "recorded-write", "restore-write", "pending-flush", "recorded-flush", "restore-flush"} {
		t.Run(mode, func(t *testing.T) {
			phase, flush := strings.CutSuffix(mode, "-flush")
			phase = strings.TrimSuffix(phase, "-write")
			if flush {
				core.RequireFsync(t)
			}
			f := newFixture(t)
			path := f.file("payload.txt")
			var manifest string
			if phase == "restore" {
				b := buryOne(t, f, "payload.txt")
				interruptRecoveryRestore(t, f, b)
				manifest = filepath.Dir(b.Grave) + ".json"
			} else {
				step := "moved"
				if phase == "recorded" {
					step = "committed"
				}
				stop := core.SetBurialStep(f.svc, func(at string) error {
					if at == step {
						return errors.New("interrupted")
					}
					return nil
				})
				t.Cleanup(stop)
				_, _ = f.svc.Bury([]string{path}, false, "")
				stop()
				rows, err := core.ScanManifests(f.svc, core.State{})
				if err != nil || len(rows) != 1 {
					t.Fatalf("pending fixture: %+v %v", rows, err)
				}
				manifest = rows[0].Manifest
			}
			due := f.file("due.txt")
			f.add(due, time.Hour)
			f.clock = f.clock.Add(time.Hour)
			if flush {
				failed := false
				t.Cleanup(core.SetReplacementSync(func(file *os.File) error {
					if !failed && filepath.Base(file.Name()) == "store.json.tmp" {
						failed = true
						return errors.New("one recovery flush failed")
					}
					return nil
				}))
			} else if err := os.Mkdir(filepath.Join(f.svc.Store.Home, "store.json.tmp"), 0o700); err != nil {
				t.Fatal(err)
			}
			out, err := f.svc.Sweep(false)
			if err == nil || !exists(due) || !exists(manifest) {
				t.Fatalf("recovery commit failure did not stop batch with evidence: %+v %v", out, err)
			}
		})
	}
}

func TestBurialRecoveryCleanupFailureIsPerRecord(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows open handles deny manifest deletion")
	}
	for _, phase := range []string{"manifest", "moved", "committed"} {
		t.Run(phase, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("payload.txt")
			stop := core.SetBurialStep(f.svc, func(at string) error {
				if at == phase {
					return errors.New("interrupted")
				}
				return nil
			})
			t.Cleanup(stop)
			_, _ = f.svc.Bury([]string{path}, false, "")
			stop()
			rows, err := core.ScanManifests(f.svc, core.State{})
			if err != nil || len(rows) != 1 {
				t.Fatalf("pending fixture: %+v %v", rows, err)
			}
			p := rows[0]
			held, err := os.Open(p.Manifest)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { held.Close() })
			due := f.file("due.txt")
			f.add(due, time.Hour)
			f.clock = f.clock.Add(time.Hour)
			out, err := f.svc.Sweep(false)
			blocked := false
			for _, row := range out {
				blocked = blocked || row.Item.ID == p.Burial.ID && row.Action == core.ActionBlocked && row.Detail != ""
			}
			if err != nil || exists(due) || !exists(p.Manifest) || !blocked {
				t.Fatalf("burial cleanup failure lost warning or stopped batch: %+v %v", out, err)
			}
			payload := p.Burial.Grave
			if phase == "manifest" {
				payload = path
			}
			if read(t, payload) != "payload.txt" {
				t.Fatal("cleanup failure changed payload")
			}
		})
	}
}

func interruptRecoveryRestore(t *testing.T, f *fixture, b core.Burial) {
	t.Helper()
	blocked := filepath.Join(f.svc.Store.Home, "store.json.tmp")
	f.fs.onRecycle = func(string) {
		if err := os.Mkdir(blocked, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	_, err := f.svc.Resurrect([]string{b.Path})
	f.fs.onRecycle = nil
	if err == nil || !exists(b.Path) {
		t.Fatalf("restore did not reach the failed commit: %v", err)
	}
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
}

func TestRecoverySchedulesBlockedPurge(t *testing.T) {
	f := newFixture(t)
	b := buryOne(t, f, "payload.txt")
	f.clock = b.PurgeAt
	conflictingManifest(t, f, b)
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || len(st.Burials) != 1 || !st.Burials[0].NextPurgeAttempt.Equal(f.clock.Add(time.Minute)) || st.Burials[0].LastPurgeError == "" {
		t.Fatalf("recovery omitted blocked purge retry: %+v %v", st.Burials, err)
	}
	if read(t, b.Grave) != "payload.txt" {
		t.Fatal("blocked purge recovery changed payload")
	}
}

func TestRecoverBurialAfterCommitFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, operation := range []string{"sweep", "sweep-renamed", "bury-tracked", "bury-new"} {
		for _, retry := range []string{"sweep", "bury"} {
			t.Run(operation+"/"+retry, func(t *testing.T) {
				f := newFixture(t)
				path := f.file("debug.log")
				if operation != "bury-new" {
					f.add(path, time.Hour)
				}
				if operation == "sweep-renamed" {
					current := filepath.Join(f.dir, "renamed.log")
					if err := os.Rename(path, current); err != nil {
						t.Fatal(err)
					}
					path = current
				}
				if strings.HasPrefix(operation, "sweep") {
					f.clock = f.clock.Add(time.Hour)
				}
				blocked := filepath.Join(f.svc.Store.Home, "store.json.tmp")
				var grave string
				f.svc.FS = &moveHookFS{testFS: f.fs, after: func(to string) {
					grave = to
					if err := os.Mkdir(blocked, 0o700); err != nil {
						t.Fatal(err)
					}
				}}
				var err error
				if strings.HasPrefix(operation, "sweep") {
					_, err = f.svc.Sweep(false)
				} else {
					_, err = f.svc.Bury([]string{path}, false, "")
				}
				if err == nil || grave == "" || exists(path) || read(t, grave) != "debug.log" {
					t.Fatalf("need a failed commit after a successful move: grave=%s, %v", grave, err)
				}
				if err := os.Remove(blocked); err != nil {
					t.Fatal(err)
				}
				if retry == "sweep" {
					_, err = f.svc.Sweep(false)
				} else {
					var got []core.GraveResult
					got, err = f.svc.Bury([]string{path}, false, "")
					if core.GraveFailed(got) {
						t.Fatalf("bury retry failed: %+v", got)
					}
				}
				st, readErr := f.svc.Store.Read()
				if err != nil || readErr != nil || len(st.Burials) != 1 || len(st.Items) != 0 || st.Burials[0].Grave != grave || st.Burials[0].Size != int64(len("debug.log")) || st.Stats.Buried != 1 {
					t.Fatalf("orphan not recovered: %+v, %v, %v", st, err, readErr)
				}
				got, err := f.svc.Resurrect([]string{fmt.Sprint(st.Burials[0].ID)})
				if err != nil || core.GraveFailed(got) || read(t, path) != "debug.log" {
					t.Fatalf("recovered burial not restorable: %+v, %v", got, err)
				}
			})
		}
	}
}

func TestRecoveryWithJunctionHome(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	f := newFixture(t)
	root := filepath.Dir(f.dir)
	alias := filepath.Join(root, "alias")
	real := filepath.Join(root, "real")
	if err := os.Mkdir(real, 0o700); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", alias, real).CombinedOutput(); err != nil {
		t.Fatalf("junction: %s, %v", out, err)
	}
	f.svc.Store.Home = filepath.Join(alias, "home")
	t.Setenv("OBIT_HOME", f.svc.Store.Home)
	f.svc.Guard = (core.Guard{System: []string{filepath.Join(real, "home")}}).WithCanonical(f.fs.RealPath)
	path := f.file("original.txt")
	item, _ := interruptBurial(t, f, path, "bury")
	got, err := f.svc.Sweep(false)
	graves := f.burials()
	if err != nil || len(graves) != 1 || len(f.items()) != 0 {
		t.Fatalf("junction home prevented recovery: %+v, %+v, %v", got, graves, err)
	}
	restored, err := f.svc.Resurrect([]string{fmt.Sprint(item.ID)})
	if err != nil || core.GraveFailed(restored) || read(t, path) != "original.txt" {
		t.Fatalf("canonical grave was not restorable: %+v, %v", restored, err)
	}
}

func TestRecoveryUsesIndependentOriginalPath(t *testing.T) {
	f := newFixture(t)
	path := f.file("original.txt")
	f.add(path, time.Hour)
	item, grave := interruptBurial(t, f, path, "bury")
	if err := f.svc.Store.Update(func(st *core.State) error {
		st.Items[st.Index(item.ID)].Path = grave
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	f.sweepAt(0)
	graves := f.burials()
	if len(graves) != 1 || graves[0].Path != path || graves[0].Reason != core.ReasonBury {
		t.Fatalf("recovery used the mutable path or lost the burial reason: %+v", graves)
	}
	got, err := f.svc.Resurrect([]string{fmt.Sprint(item.ID)})
	if err != nil || core.GraveFailed(got) || read(t, path) != "original.txt" {
		t.Fatalf("original location not restored: %+v, %v", got, err)
	}
}

func TestGraveRootResolutionFailureRetainsRecovery(t *testing.T) {
	f := newFixture(t)
	path := f.file("original.txt")
	item, grave := interruptBurial(t, f, path, "bury")
	f.svc.FS = rootFailureFS{testFS: f.fs, root: core.GraveyardRoot(f.svc, path)}
	out, scanErr := f.svc.Sweep(false)
	if scanErr != nil || len(out) != 1 || out[0].Action != core.ActionBlocked || out[0].Detail == "" || len(f.items()) != 0 || !exists(grave) {
		t.Fatalf("resolution failure discarded recovery: %+v", out)
	}
	f.svc.FS = f.fs
	f.sweepAt(time.Minute)
	f.svc.FS = rootFailureFS{testFS: f.fs, root: core.GraveyardRoot(f.svc, path)}
	got, err := f.svc.Resurrect([]string{fmt.Sprint(item.ID)})
	if err != nil || !core.GraveFailed(got) || len(f.burials()) != 1 || !exists(grave) {
		t.Fatalf("resolution failure must preserve the grave: %+v, %v", got, err)
	}
}

func TestRecoveryDoesNotDependOnItem(t *testing.T) {
	f := newFixture(t)
	path := f.file("keep.txt")
	item := f.add(path, time.Hour)
	f.clock = f.clock.Add(time.Hour)
	_, grave := interruptBurial(t, f, path, "sweep")
	// Simulate an ordinary cancellation after the failed commit. Recovery must
	// have an independent durable source, even when there is no Item at all.
	if err := f.svc.Store.Update(func(st *core.State) error { st.Items = nil; return nil }); err != nil {
		t.Fatal(err)
	}
	_, _, err := f.svc.Tick()
	graves := f.burials()
	if err != nil || len(graves) != 1 || graves[0].ID != item.ID || graves[0].Grave != grave || read(t, grave) != "keep.txt" {
		t.Fatalf("lost burial without Item: %+v, %v", graves, err)
	}
}

func TestGraveWithoutManifestKeepsTrackedItem(t *testing.T) {
	f := newFixture(t)
	path := f.file("keep.txt")
	item := f.add(path, time.Hour)
	_, grave := interruptBurial(t, f, path, "bury")
	if err := os.Remove(filepath.Dir(grave) + ".json"); err != nil {
		t.Fatal(err)
	}
	out := f.sweepAt(time.Hour)
	if len(out) != 1 || out[0].Action != core.ActionRetry || len(f.items()) != 1 || f.items()[0].ID != item.ID || read(t, grave) != "keep.txt" {
		t.Fatalf("unindexed grave lost its tracked entry: %+v, items=%+v", out, f.items())
	}
	if rows, err := f.svc.PendingGraves(); err != nil || len(rows) != 1 || rows[0].Status != "unverifiable" {
		t.Fatalf("missing manifest not diagnosable: %+v, %v", rows, err)
	}
	events, err := f.svc.Store.History(20)
	if err != nil || !strings.Contains(fmt.Sprint(events), "warning") {
		t.Fatalf("missing warning: %+v, %v", events, err)
	}
}

func TestSweepReportsRecoveryAndBlockedManifests(t *testing.T) {
	for _, corrupt := range []bool{false, true} {
		t.Run(fmt.Sprint(corrupt), func(t *testing.T) {
			f := newFixture(t)
			path := f.file("keep.txt")
			item, grave := interruptBurial(t, f, path, "bury")
			if corrupt {
				if err := os.WriteFile(filepath.Dir(grave)+".json", []byte("{broken"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			out, err := f.svc.Sweep(false)
			want := core.ActionBuried
			if corrupt {
				want = "blocked"
			}
			if err != nil || len(out) != 1 || out[0].Action != want || out[0].Item.ID != item.ID || out[0].Detail == "" {
				t.Fatalf("recovery receipt missing: %+v, %v", out, err)
			}
			if !corrupt && (out[0].Grave != grave || out[0].GraveID == nil) {
				t.Fatalf("recovered grave missing from receipt: %+v", out)
			}
		})
	}
}

func TestGraveCommandsCleanCommittedManifest(t *testing.T) {
	for _, operation := range []string{"resurrect", "purge"} {
		t.Run(operation, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("keep.txt")
			stop := errors.New("interrupted after commit")
			restore := core.SetBurialStep(f.svc, func(step string) error {
				if step == "committed" {
					return stop
				}
				return nil
			})
			t.Cleanup(restore)
			_, err := f.svc.Bury([]string{path}, false, "")
			restore()
			if !errors.Is(err, stop) {
				t.Fatal("interruption not reached", err)
			}
			b := f.burials()[0]
			var rows []core.GraveResult
			if operation == "purge" {
				rows, err = f.svc.Purge(true, false)
			} else {
				rows, err = f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
			}
			if err != nil || core.GraveFailed(rows) || len(f.burials()) != 0 || exists(filepath.Dir(b.Grave)+".json") {
				t.Fatalf("completed command stranded manifest: %+v, %v", rows, err)
			}
			pending, err := f.svc.PendingGraves()
			if err != nil || len(pending) != 0 {
				t.Fatalf("completed grave remains unverifiable: %+v, %v", pending, err)
			}
		})
	}
}

func TestRecoveryReconcilesDepartedChildren(t *testing.T) {
	for _, deleted := range []bool{false, true} {
		for _, removed := range []bool{false, true} {
			t.Run(fmt.Sprintf("deleted=%v/entryRemoved=%v", deleted, removed), func(t *testing.T) {
				f := newFixture(t)
				path := f.file("output/child.txt")
				parent := f.add(filepath.Dir(path), time.Hour)
				child := f.add(path, time.Hour)
				f.clock = f.clock.Add(time.Hour)
				stop := errors.New("stop after move")
				restore := core.SetBurialStep(f.svc, func(step string) error {
					if step == "moved" {
						return stop
					}
					return nil
				})
				t.Cleanup(restore)
				_, err := f.svc.Bury([]string{parent.Path}, false, "")
				restore()
				if !errors.Is(err, stop) {
					t.Fatal(err)
				}
				pending, err := f.svc.PendingGraves()
				if err != nil || len(pending) != 1 {
					t.Fatalf("pending: %+v %v", pending, err)
				}
				inside := filepath.Join(pending[0].Burial.Grave, "child.txt")
				outside := filepath.Join(f.dir, "escaped.txt")
				if deleted {
					err = os.Remove(inside)
				} else {
					err = os.Rename(inside, outside)
				}
				if err != nil {
					t.Fatal(err)
				}
				if removed {
					if err := f.svc.Store.Update(func(st *core.State) error {
						st.Items = st.Items[:1]
						return nil
					}); err != nil {
						t.Fatal(err)
					}
				}
				// Recovery must retire the old lifespan even when the child left.
				_, err = f.svc.Bury([]string{parent.Path}, false, "")
				if err != nil || len(f.burials()) != 1 || len(f.burials()[0].Contains) != 0 {
					t.Fatalf("departed child blocks recovery: burials=%+v err=%v", f.burials(), err)
				}
				st, err := f.svc.Store.Read()
				if err != nil {
					t.Fatal(err)
				}
				idx := st.Index(child.ID)
				if idx >= 0 || !deleted && read(t, outside) != "output/child.txt" {
					t.Fatalf("departed child retained a lifespan: %+v", st.Items)
				}
				if _, err := f.svc.Sweep(false); err != nil || len(f.burials()) != 1 || !deleted && !exists(outside) {
					t.Fatalf("departed child buried again: %v", err)
				}
				if exists(pending[0].Manifest) {
					t.Fatal("recovered manifest still pending")
				}
			})
		}
	}
}

func TestChildRecoveryRetriesTemporaryLocateFailure(t *testing.T) {
	f := newFixture(t)
	path := f.file("output/child.txt")
	parent := f.add(filepath.Dir(path), time.Hour)
	child := f.add(path, time.Hour)
	f.clock = f.clock.Add(time.Hour)
	stop := errors.New("interrupted")
	restore := core.SetBurialStep(f.svc, func(step string) error {
		if step == "moved" {
			return stop
		}
		return nil
	})
	t.Cleanup(restore)
	if _, err := f.svc.Bury([]string{parent.Path}, false, ""); !errors.Is(err, stop) {
		t.Fatal(err)
	}
	restore()
	f.fs.locateErr = errors.New("temporary child locate failure")
	if rows, err := f.svc.Bury([]string{parent.Path}, false, ""); err != nil || !core.GraveFailed(rows) {
		t.Fatalf("uncertain child recovery must be a per-record failure: %+v %v", rows, err)
	}
	if len(f.items()) != 2 || len(f.burials()) != 0 {
		t.Fatal("uncertain child retired")
	}
	f.fs.locateErr = nil
	if _, err := f.svc.Bury([]string{parent.Path}, false, ""); err != nil {
		t.Fatal(err)
	}
	if len(f.items()) != 0 || len(f.burials()) != 1 || len(f.burials()[0].Contains) != 1 || f.burials()[0].Contains[0] != child.ID {
		t.Fatal("recovery did not absorb verified child")
	}
}

func TestRecordedChildRecoveryLeavesEscapedChildUntracked(t *testing.T) {
	f := newFixture(t)
	path := f.file("output/child.txt")
	parent := f.add(filepath.Dir(path), time.Hour)
	f.add(path, time.Hour)
	f.clock = f.clock.Add(time.Hour)
	stop := errors.New("interrupted after commit")
	restore := core.SetBurialStep(f.svc, func(step string) error {
		if step == "committed" {
			return stop
		}
		return nil
	})
	t.Cleanup(restore)
	if _, err := f.svc.Bury([]string{parent.Path}, false, ""); !errors.Is(err, stop) {
		t.Fatal(err)
	}
	restore()
	b := f.burials()[0]
	outside := filepath.Join(f.dir, "escaped.txt")
	if err := os.Rename(filepath.Join(b.Grave, "child.txt"), outside); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Bury([]string{parent.Path}, false, ""); err != nil {
		t.Fatal(err)
	}
	items := f.items()
	if len(items) != 0 || read(t, outside) != "output/child.txt" {
		t.Fatalf("committed manifest rebuilt escaped lifespan: %+v", items)
	}
	if _, err := f.svc.Sweep(false); err != nil || len(f.burials()) != 1 || !exists(outside) {
		t.Fatalf("escaped child buried again: %v", err)
	}
	if len(f.burials()[0].Contains) != 0 || exists(filepath.Dir(b.Grave)+".json") {
		t.Fatal("recorded recovery did not reconcile and retire manifest")
	}
}

func TestRecoveryBlocksDepartedChildrenUntilManifestVerified(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, fault := range []string{"grave-unreadable", "invalid-version", "unknown-field", "trailing-json", "contains-only"} {
		for _, byIdentity := range []bool{false, true} {
			if fault == "contains-only" && byIdentity {
				continue // This damaged manifest has only the child's number.
			}
			t.Run(fmt.Sprintf("%s/identity=%v", fault, byIdentity), func(t *testing.T) {
				f := newFixture(t)
				path := f.file("output/child.txt")
				parent := f.add(filepath.Dir(path), time.Hour)
				child := f.add(path, time.Hour)
				f.clock = parent.ExpiresAt
				stop := errors.New("interrupted after parent move")
				restore := core.SetBurialStep(f.svc, func(step string) error {
					if step == "moved" {
						return stop
					}
					return nil
				})
				t.Cleanup(restore)
				_, err := f.svc.Bury([]string{parent.Path}, false, "")
				restore()
				if !errors.Is(err, stop) {
					t.Fatal(err)
				}
				pending, err := f.svc.PendingGraves()
				if err != nil || len(pending) != 1 || pending[0].Burial == nil {
					t.Fatalf("pending parent: %+v %v", pending, err)
				}
				p := pending[0]
				if err := os.MkdirAll(parent.Path, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Rename(filepath.Join(p.Burial.Grave, "child.txt"), path); err != nil {
					t.Fatal(err)
				}
				if byIdentity {
					if err := f.svc.Store.Update(func(st *core.State) error {
						idx := st.Index(child.ID)
						child.ID = st.NextID
						st.NextID++
						st.Items[idx].ID = child.ID
						return nil
					}); err != nil {
						t.Fatal(err)
					}
				}
				original := read(t, p.Manifest)
				switch fault {
				case "grave-unreadable":
					f.svc.FS = unreadableGraveFS{FS: f.fs, dir: filepath.Dir(p.Burial.Grave)}
				case "trailing-json":
					if err := os.WriteFile(p.Manifest, []byte(original+"{}"), 0o600); err != nil {
						t.Fatal(err)
					}
				default:
					var doc map[string]json.RawMessage
					if err := json.Unmarshal([]byte(original), &doc); err != nil {
						t.Fatal(err)
					}
					switch fault {
					case "invalid-version":
						doc["version"] = json.RawMessage("2")
					case "unknown-field":
						doc["unknown"] = json.RawMessage("true")
					case "contains-only":
						delete(doc, "children")
					}
					data, err := json.Marshal(doc)
					if err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(p.Manifest, data, 0o600); err != nil {
						t.Fatal(err)
					}
				}
				for range 2 {
					out, err := f.svc.Sweep(false)
					if err != nil || !exists(path) || len(f.burials()) != 0 || !exists(p.Manifest) {
						t.Fatalf("blocked child was buried again: %+v %v", out, err)
					}
					st, err := f.svc.Store.Read()
					if err != nil || st.Index(child.ID) < 0 || st.Index(parent.ID) < 0 {
						t.Fatalf("blocked lifespans lost: %+v %v", st.Items, err)
					}
					for _, id := range []int64{parent.ID, child.ID} {
						it := st.Items[st.Index(id)]
						if !it.NextAttempt.After(f.clock) || it.LastError == "" {
							t.Fatalf("blocked #%d lacks retry: %+v", id, it)
						}
					}
					for _, row := range out {
						if row.Item.ID == child.ID && row.Action != core.ActionBlocked {
							t.Fatalf("blocked child processed by Sweep: %+v", row)
						}
					}
					f.clock = f.clock.Add(time.Hour)
				}
				if got := read(t, path); got != "output/child.txt" {
					t.Fatalf("child content changed: %q", got)
				}
				// Remove the fault; recovery retires the departed child's old expiry.
				f.svc.FS = f.fs
				if err := os.WriteFile(p.Manifest, []byte(original), 0o600); err != nil {
					t.Fatal(err)
				}
				if !byIdentity {
					if out, err := f.svc.Sweep(false); err != nil || len(f.items()) != 0 || len(f.burials()) != 1 || !exists(path) || exists(p.Manifest) {
						t.Fatalf("verified recovery did not retire child: %+v %v", out, err)
					}
				}
			})
		}
	}
}

func TestOfflineRootDoesNotBlockOtherRoots(t *testing.T) {
	for _, operation := range []string{"sweep", "bury", "tick"} {
		t.Run(operation, func(t *testing.T) {
			f := newFixture(t)
			pending := f.file("pending.txt")
			_, grave := interruptBurial(t, f, pending, "bury")
			path := f.file("due.txt")
			f.add(path, time.Hour)
			f.clock = f.clock.Add(time.Hour)
			// Pick a genuinely absent drive; never create anything on it.
			volume := ""
			for drive := 'Z'; drive >= 'E'; drive-- {
				candidate := fmt.Sprintf("%c:\\", drive)
				if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
					volume = candidate
					break
				}
			}
			if volume == "" {
				t.Skip("no absent drive available")
			}
			offline := core.Item{ID: 99, Rev: 1, Path: volume + "offline.txt", ExpiresAt: f.clock,
				Identity: &core.Identity{Volume: volume, Serial: 123, FileID: strings.Repeat("a", 32)}}
			if err := f.svc.Store.Update(func(st *core.State) error { st.Items = append(st.Items, offline); return nil }); err != nil {
				t.Fatal(err)
			}
			var err error
			switch operation {
			case "sweep":
				_, err = f.svc.Sweep(false)
			case "tick":
				_, _, err = f.svc.Tick()
			case "bury":
				_, err = f.svc.Bury([]string{path}, false, "")
			}
			if err != nil || exists(path) || read(t, grave) != "pending.txt" || len(f.burials()) != 2 || len(f.items()) != 1 {
				t.Fatalf("offline root stopped online work: burials=%+v items=%+v err=%v", f.burials(), f.items(), err)
			}
			got := f.items()[0]
			if got.Attempts != 1 || got.LastError == "" || !got.NextAttempt.Equal(f.clock.Add(time.Minute)) {
				t.Fatalf("offline record lacks retry backoff: %+v", got)
			}
			got.Attempts, got.NextAttempt, got.LastError, got.Rev = offline.Attempts, offline.NextAttempt, offline.LastError, offline.Rev
			if !reflect.DeepEqual(got, offline) {
				t.Fatalf("offline record changed beyond retry state: %+v", got)
			}
		})
	}
}
