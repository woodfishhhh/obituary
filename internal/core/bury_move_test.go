//go:build windows

package core_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"golang.org/x/sys/windows"
)

func TestSingleBurialInvalidatedSnapshotReleasesLedger(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git index publication")
	}
	f, paths, git := slowGitFixture(t, "keep.log")
	if _, err := f.svc.Cancel(paths); err != nil {
		t.Fatal(err)
	}
	// A new write releases the keep decision before testing watch movement.
	info, err := os.Stat(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(paths[0], info.ModTime(), info.ModTime().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	policy, err := f.svc.SetWatch(f.dir, "1h", core.WatchFirstSeen)
	if err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, proposal := range st.Proposals {
		if err := f.svc.ApproveWatch(proposal); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.svc.RegisterWatchChild(policy, paths[0]); err != nil {
		t.Fatal(err)
	}
	f.svc.Mode = core.ModeRecycle
	f.clock = f.clock.Add(time.Hour)
	assertRefresh := invalidatedIndex(t, f, git, "keep.log", false)
	rows, err := f.svc.Sweep(false)
	assertRefresh()
	if err != nil || len(rows) != 1 || rows[0].Action != core.ActionRefused || read(t, paths[0]) != "keep.log" || len(f.burials()) != 0 {
		t.Fatalf("single refresh lost Git protection: %+v %v", rows, err)
	}
}

func TestSingleBurialSlowGit(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git repository and delayed single-item burial")
	}
	for _, tracked := range []bool{false, true} {
		t.Run(fmt.Sprint(tracked), func(t *testing.T) {
			f, paths, git := slowGitFixture(t, "scratch.log")
			if _, err := f.svc.Cancel(paths); err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(paths[0])
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Chtimes(paths[0], info.ModTime(), info.ModTime().Add(time.Second)); err != nil {
				t.Fatal(err)
			}
			policy, err := f.svc.SetWatch(f.dir, "1h", core.WatchFirstSeen)
			if err != nil {
				t.Fatal(err)
			}
			st, err := f.svc.Store.ReadSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			for _, proposal := range st.Proposals {
				if err := f.svc.ApproveWatch(proposal); err != nil {
					t.Fatal(err)
				}
			}
			if err := f.svc.RegisterWatchChild(policy, paths[0]); err != nil {
				t.Fatal(err)
			}
			f.svc.Mode = core.ModeRecycle
			f.clock = f.clock.Add(time.Hour)
			published := false
			f.svc.PrepareOwners = func(ctx context.Context, paths []string) core.OwnerBatchFunc {
				query := auto.PrepareOwners(ctx, paths)
				return func(ctx context.Context, paths []string) []core.Ownership {
					owners := delayedOwners(ctx, query, paths)
					if tracked && !published {
						git("add", "--", "scratch.log")
						published = true
					}
					return owners
				}
			}
			rows, err := f.svc.Sweep(false)
			want := core.ActionBuried
			if tracked {
				want = core.ActionRefused
			}
			if err != nil || len(rows) != 1 || rows[0].Action != want || exists(paths[0]) != tracked {
				t.Fatalf("single slow Git burial lost progress or protection: %+v %v", rows, err)
			}
		})
	}
}

func TestSweepBoundsDirectoryGuardTraversal(t *testing.T) {
	if testing.Short() {
		t.Skip("walks a large real directory tree to exhaust the guard budget")
	}
	f := newFixture(t)
	dir := filepath.Join(f.dir, "wide")
	f.file("wide/seed.txt")
	for i := 0; i < 20000; i++ {
		if err := os.Mkdir(filepath.Join(dir, fmt.Sprintf("d%05d", i)), 0700); err != nil {
			t.Fatal(err)
		}
	}
	id, err := f.fs.Identify(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Store.Update(func(st *core.State) error {
		st.Items = []core.Item{{ID: 1, Rev: 1, Path: dir, IsDir: true, Identity: &id, ExpiresAt: f.clock.Add(-time.Hour)}}
		st.NextID = 2
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	readDone := make(chan error, 1)
	started := false
	f.svc.Guard.Canonical = func(path string) (string, error) {
		if samePath(path, dir) && !started {
			started = true
			go func() {
				_, err := f.svc.Store.WithDeadline(time.Now().Add(time.Second)).ReadSnapshot()
				readDone <- err
			}()
		}
		return path, nil
	}
	out, err := f.svc.Sweep(false)
	if err != nil || !started {
		t.Fatalf("guard setup: %+v %v", out, err)
	}
	readErr := <-readDone
	if readErr != nil {
		t.Fatalf("directory Guard starved shared reader: %v", readErr)
	}
	if len(out) != 1 || out[0].Action != core.ActionRetry || !exists(dir) || len(f.burials()) != 0 || len(f.items()) != 1 {
		t.Fatalf("directory Guard ignored traversal budget: %+v", out)
	}
}

func TestSweepReportsMovedUncommittedBurial(t *testing.T) {
	f := newFixture(t)
	path := f.file("payload.txt")
	f.add(path, time.Hour)
	f.clock = f.clock.Add(time.Hour)
	var handle windows.Handle
	f.fs.onRecycle = func(string) {
		name, err := windows.UTF16PtrFromString(filepath.Join(f.svc.Store.Home, "store.json"))
		if err != nil {
			t.Fatal(err)
		}
		handle, err = windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, 0, 0)
		if err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		if handle != 0 {
			windows.CloseHandle(handle)
		}
	}()
	rows, err := f.svc.Sweep(false)
	if err == nil || len(rows) != 1 || rows[0].Action != core.ActionRecoveryPending || rows[0].Grave == "" || !strings.Contains(rows[0].Detail, rows[0].Grave) || !strings.Contains(rows[0].Detail, "obit reap") {
		t.Fatalf("moved burial lacked recovery pending receipt: %+v %v", rows, err)
	}
	if exists(path) || !exists(filepath.Dir(rows[0].Grave)+".json") || read(t, rows[0].Grave) != "payload.txt" {
		t.Fatal("pending burial lost object or manifest")
	}
	windows.CloseHandle(handle)
	handle = 0
	f.fs.onRecycle = nil
	if rows, err := f.svc.Sweep(false); err != nil || len(rows) != 1 || rows[0].Action != core.ActionBuried {
		t.Fatalf("released ledger did not recover: %+v %v", rows, err)
	}
}

type incompleteLongNamesFS struct{ core.FS }

func (fs incompleteLongNamesFS) LongPath(path string) (string, bool, error) {
	expanded, _, err := fs.FS.LongPath(path)
	return expanded, false, err
}

func TestBurialRecordsIncompleteLongNameExpansion(t *testing.T) {
	f := newFixture(t)
	f.svc.FS = incompleteLongNamesFS{f.fs}
	rows, err := f.svc.Bury([]string{f.file("payload.txt")}, false, "")
	if err != nil || core.GraveFailed(rows) || rows[0].Burial.LongNames {
		t.Fatalf("burial falsely certified unexpanded names: %+v %v", rows, err)
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || len(st.Burials) != 1 || st.Burials[0].LongNames {
		t.Fatalf("ledger falsely certified unexpanded names: %+v %v", st, err)
	}
}

func TestBurialProtocolInterruptions(t *testing.T) {
	if testing.Short() {
		t.Skip("exercises every durable burial interruption and recovery point")
	}
	core.RequireFsync(t)
	for _, operation := range []string{"sweep", "bury-tracked", "bury-new"} {
		for _, step := range []string{"reserved", "prepared", "manifest", "moved", "committed", "cleaned", "measured"} {
			for _, recovery := range []string{"tick", "sweep", "bury"} {
				t.Run(operation+"/"+step+"/"+recovery, func(t *testing.T) {
					f := newFixture(t)
					path := f.file("keep.txt")
					var original core.Item
					if operation != "bury-new" {
						rows, err := f.svc.Add([]string{path}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour),
							Note: "retain metadata", Source: "mktemp", Project: f.dir, Session: "session-7"})
						if err != nil || core.Failed(rows) {
							t.Fatalf("add: %+v, %v", rows, err)
						}
						original = *rows[0].Item
					}
					if operation == "sweep" {
						f.clock = f.clock.Add(time.Hour)
					}
					calls := 0
					stop := errors.New("process interrupted at " + step)
					restore := core.SetBurialStep(f.svc, func(at string) error {
						if at == step {
							calls++
							return stop
						}
						return nil
					})
					t.Cleanup(restore)
					var err error
					if operation == "sweep" {
						_, err = f.svc.Sweep(false)
					} else {
						_, err = f.svc.Bury([]string{path}, false, "")
					}
					restore()
					if calls != 1 || !errors.Is(err, stop) {
						t.Fatalf("interruption not reached: calls=%d, %v", calls, err)
					}
					// Use a fresh Service instance: only persisted state may recover.
					restarted := *f.svc
					f.svc = &restarted
					switch recovery {
					case "tick":
						_, _, err = f.svc.Tick()
					case "sweep":
						_, err = f.svc.Sweep(false)
					case "bury":
						_, err = f.svc.Bury([]string{path}, false, "")
					}
					if err != nil {
						t.Fatal(err)
					}
					st, err := f.svc.Store.Read()
					if err != nil {
						t.Fatal(err)
					}
					if exists(path) {
						if len(st.Burials) != 0 || read(t, path) != "keep.txt" {
							t.Fatalf("original and burial disagree: %+v", st)
						}
						if original.ID == 0 && len(st.Items) != 0 || original.ID != 0 && (len(st.Items) != 1 || !reflect.DeepEqual(st.Items[0], original)) {
							t.Fatalf("interrupted pre-move burial changed Item: %+v", st.Items)
						}
					} else {
						if len(st.Burials) != 1 || len(st.Items) != 0 || st.Stats.Buried != 1 || read(t, st.Burials[0].Grave) != "keep.txt" {
							t.Fatalf("orphan or duplicate burial: %+v", st)
						}
						b := st.Burials[0]
						if b.Path != path || original.ID != 0 && (b.ID != original.ID || b.Note != original.Note || b.Source != original.Source || b.Project != original.Project || b.Session != original.Session) {
							t.Fatalf("manifest lost original metadata: %+v", b)
						}
						if st.Stats.BuriedBytes != b.Size || b.Size > int64(len("keep.txt")) {
							t.Fatalf("incorrect statistics: %+v", st)
						}
					}
					manifests, err := filepath.Glob(filepath.Join(core.GraveyardRoot(f.svc, path), "*.json"))
					if err != nil || len(manifests) != 0 || len(f.fs.recycled) > 1 {
						t.Fatalf("manifest left or object moved twice: %v, moves=%v, %v", manifests, f.fs.recycled, err)
					}
				})
			}
		}
	}
}

func TestManifestChildLifespanRechecked(t *testing.T) {
	for _, operation := range []string{"sweep", "bury"} {
		t.Run(operation, func(t *testing.T) {
			f := newFixture(t)
			child := f.file("output/keep.txt")
			dir := filepath.Dir(child)
			f.add(dir, time.Hour)
			f.clock = f.clock.Add(time.Hour)
			var saved core.Item
			t.Cleanup(core.SetAfterManifest(f.svc, func() {
				saved = f.add(child, 30*24*time.Hour)
			}))
			if operation == "sweep" {
				_, _ = f.svc.Sweep(false)
			} else {
				_, _ = f.svc.Bury([]string{dir}, false, "")
			}
			st, err := f.svc.Store.Read()
			idx := st.Index(saved.ID)
			if err != nil || !exists(child) || len(st.Burials) != 0 || idx < 0 || !reflect.DeepEqual(st.Items[idx], saved) {
				t.Fatalf("later child lifespan must prevent moving parent: %+v, %v", st, err)
			}
		})
	}
}
