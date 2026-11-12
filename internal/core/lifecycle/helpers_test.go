package lifecycle_test

import (
	"bytes"
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

	"github.com/woodfishhhh/obituary/internal/auto"
	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func delayedOwners(ctx context.Context, query core.OwnerBatchFunc, paths []string) []core.Ownership {
	timer := time.NewTimer(350 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
	// Always use the real Git provider, including its cancellation behavior.
	return query(ctx, paths)
}

func slowGitFixture(t *testing.T, names ...string) (*fixture, []string, func(...string)) {
	t.Helper()
	f := newFixture(t)
	git := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", f.dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %s %v", out, err)
		}
	}
	testenv.InitRepo(t, f.dir)
	var paths []string
	for _, name := range names {
		paths = append(paths, f.file(name))
	}
	f.svc.Owner, f.svc.OwnerBatch = auto.Owner, auto.OwnerBatch
	rows, err := f.svc.Add(paths, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour), Source: "rule", Project: f.dir})
	if err != nil || core.Failed(rows) {
		t.Fatalf("add: %+v %v", rows, err)
	}
	f.clock = f.clock.Add(time.Hour)
	return f, paths, git
}

// Probe the actual ledger lock once; scheduling delays cannot turn an unlocked
// ledger into a failed deadline. TryLockExclusive uses the platform lock backend.
func ledgerUnlocked(t *testing.T, f *fixture) bool {
	t.Helper()
	release, ok, err := core.TryLockExclusive(filepath.Join(f.svc.Store.Home, "store.lock"))
	if err != nil {
		t.Fatalf("probe store.lock: %v", err)
	}
	if ok {
		release()
	}
	return ok
}

func invalidatedIndex(t *testing.T, f *fixture, git func(...string), name string, afterMove bool) func() {
	t.Helper()
	git("add", "--", name)
	index, staged := filepath.Join(f.dir, ".git", "index"), filepath.Join(f.dir, "staged-index")
	if err := os.Rename(index, staged); err != nil {
		t.Fatal(err)
	}
	published := false
	publish := func() {
		if !published {
			if err := os.Rename(staged, index); err != nil {
				t.Fatal(err)
			}
			published = true
		}
	}
	if afterMove {
		t.Cleanup(core.SetBurialStep(f.svc, func(step string) error {
			if step == "moved" {
				publish()
			}
			return nil
		}))
	}
	refreshed := false
	f.svc.PrepareOwners = func(ctx context.Context, paths []string) core.OwnerBatchFunc {
		query := auto.PrepareOwners(ctx, paths)
		calls := 0
		return func(ctx context.Context, paths []string) []core.Ownership {
			calls++
			if calls == 2 {
				refreshed = true
				if !ledgerUnlocked(t, f) {
					t.Fatal("invalidated snapshot refresh held store.lock")
				}
			}
			owners := query(ctx, paths)
			if calls == 1 && !afterMove {
				publish()
			}
			return owners
		}
	}
	return func() {
		t.Helper()
		if !refreshed {
			t.Fatal("invalidated snapshot did not start another Git query")
		}
	}
}

func assertCommittedAudit(t *testing.T, f *fixture, action string, run func(context.Context) error, committed func(core.State) bool) {
	t.Helper()
	if err := os.MkdirAll(f.svc.Store.Home, 0o700); err != nil {
		t.Fatal(err)
	}
	release, err := core.LockFile(filepath.Join(f.svc.Store.Home, "audit.lock"), true)
	if err != nil {
		t.Fatal(err)
	}
	unlock := sync.OnceFunc(release)
	defer unlock()
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- run(ctx) }()
	finished := false
	defer func() {
		cancel()
		unlock()
		if !finished {
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Error("audit operation did not stop after cleanup")
			}
		}
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		// Read takes the ledger lock. Polling Inspect would open store.json
		// across MoveFileEx and can itself make a Windows commit fail sharing.
		st, err := f.svc.Store.Read()
		if err == nil && committed(st) {
			break
		}
		select {
		case err := <-done:
			finished = true
			t.Fatalf("operation ended before expected ledger commit: %v", err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("ledger did not commit while audit was locked")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	// A canceled Record must not abandon the already committed event while
	// waiting for this lock; let its cancellation waiter run before releasing.
	select {
	case err := <-done:
		finished = true
		t.Errorf("committed audit abandoned on cancellation: %v", err)
	case <-time.After(50 * time.Millisecond):
		unlock()
		select {
		case err := <-done:
			finished = true
			if err != nil {
				t.Errorf("committed audit failed: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("audit did not resume after unlock")
		}
	}
	unlock()
	events, err := f.svc.Store.History(20)
	if err != nil {
		t.Fatal(err)
	}
	for _, event := range events {
		if event.Action == action {
			return
		}
	}
	t.Errorf("committed audit missing after cancellation: %s", action)
}

func approveWatch(t *testing.T, svc *core.Service) {
	t.Helper()
	st, err := svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range st.Proposals {
		if p.Kind != core.ProposalWatch {
			continue
		}
		rows, err := (&auto.Engine{Svc: svc}).Approve([]string{fmt.Sprint(p.ID)})
		if err != nil || len(rows) != 1 || rows[0].Error != "" {
			t.Fatalf("approve policy: %+v %v", rows, err)
		}
	}
}

func watchFixture(t *testing.T, name, age string, lifespan ...string) (*fixture, string, core.WatchPolicy) {
	t.Helper()
	if testing.Short() {
		t.Skip("folder policy evidence uses a real Git repository")
	}
	f := newFixture(t)
	if out, err := exec.Command("git", "-C", f.dir, "init", "-q").CombinedOutput(); err != nil {
		t.Fatalf("git init: %s %v", out, err)
	}
	f.svc.Owner, f.svc.OwnerBatch, f.svc.PrepareOwners = auto.Owner, auto.OwnerBatch, auto.PrepareOwners
	path := f.file("downloads/" + name)
	if err := os.Chtimes(path, f.clock, f.clock); err != nil {
		t.Fatal(err)
	}
	duration := "1h"
	if len(lifespan) > 0 {
		duration = lifespan[0]
	}
	p, err := f.svc.SetWatch(filepath.Join(f.dir, "downloads"), duration, age)
	if err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Proposals) != 0 {
		rows, err := (&auto.Engine{Svc: f.svc}).Approve([]string{fmt.Sprint(st.Proposals[0].ID)})
		if err != nil || len(rows) != 1 || rows[0].Error != "" {
			t.Fatalf("approve: %+v %v", rows, err)
		}
	}
	return f, path, p
}

type watchObserveCallbackFS struct {
	core.FS
	onHold func()
}

func (fs *watchObserveCallbackFS) ObserveWatch(path string, hold bool) (core.WatchSnapshot, func(), error) {
	if hold && fs.onHold != nil {
		call := fs.onHold
		fs.onHold = nil
		call()
	}
	return fs.FS.ObserveWatch(path, hold)
}

func spareWatchFixture(t *testing.T, f *fixture, path string, restore bool) {
	t.Helper()
	if !restore {
		rows, err := f.svc.Cancel([]string{path})
		if err != nil || core.Failed(rows) {
			t.Fatalf("spare: %+v %v", rows, err)
		}
		return
	}
	out := f.sweepAt(2 * time.Hour)
	if len(out) != 1 || out[0].Action != core.ActionBuried {
		t.Fatalf("bury: %+v", out)
	}
	rows, err := f.svc.Resurrect([]string{fmt.Sprint(out[0].Item.ID)})
	if err != nil || core.GraveFailed(rows) {
		t.Fatalf("restore: %+v %v", rows, err)
	}
}

func (f *fixture) burials() []core.Burial {
	f.t.Helper()
	state, err := f.svc.Store.Read()
	if err != nil {
		f.t.Fatal(err)
	}
	return state.Burials
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func errorsAs(err error, target **core.RefusedError) bool { return errors.As(err, target) }

func buryOne(t *testing.T, f *fixture, name string) core.Burial {
	t.Helper()
	path := f.file(name)
	f.add(path, time.Hour)
	outcomes := f.sweepAt(time.Hour)
	graves := f.burials()
	if len(graves) == 0 {
		t.Fatalf("fixture burial failed: %+v", outcomes)
	}
	return graves[len(graves)-1]
}

func deferTestObservations(t *testing.T, store core.Store, paths []string, session string) error {
	t.Helper()
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("observed"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	svc := &core.Service{Store: store, FS: &testFS{}}
	rows, err := svc.CaptureObservations(paths, session)
	if err != nil {
		return err
	}
	return store.DeferObservations(rows)
}

// testFS uses real Windows file IDs and real burials (a rename inside the
// test's temporary directory). Recycling deletes instead, so tests never
// touch the user's Recycle Bin. fail and onRecycle apply to both.
type testFS struct {
	winfs.FS
	fail                 error
	locateErr            error
	identifyFollowingErr func(path string) error
	onRecycle            func(path string) // runs while deleting, before the outcome
	recycled             []string
}

func (f *testFS) IdentifyFollowing(path string) (core.Identity, error) {
	if f.identifyFollowingErr != nil {
		if err := f.identifyFollowingErr(path); err != nil {
			return core.Identity{}, err
		}
	}
	return f.FS.IdentifyFollowing(path)
}

func (f *testFS) MovePinned(path string, want core.Identity, to string) error {
	if f.onRecycle != nil {
		f.onRecycle(path)
	}
	if f.fail != nil {
		return f.fail
	}
	f.recycled = append(f.recycled, path)
	return f.FS.MovePinned(path, want, to)
}

func (f *testFS) MovePinnedInto(path string, want core.Identity, parent core.RestoreParent, name string) (string, error) {
	if f.onRecycle != nil {
		f.onRecycle(path)
	}
	if f.fail != nil {
		return "", f.fail
	}
	f.recycled = append(f.recycled, path)
	return f.FS.MovePinnedInto(path, want, parent, name)
}

func (f *testFS) Locate(id core.Identity) (string, error) {
	if f.locateErr != nil {
		return "", f.locateErr
	}
	return f.FS.Locate(id)
}

func (f *testFS) Recycle(path string, want core.Identity) error {
	if f.onRecycle != nil {
		f.onRecycle(path)
	}
	if f.fail != nil {
		return f.fail
	}
	if got, err := f.Identify(path); err != nil || got != want {
		return core.ErrUnsafe
	}
	f.recycled = append(f.recycled, path)
	return os.RemoveAll(path)
}

type fixture struct {
	t     *testing.T
	dir   string
	svc   *core.Service
	fs    *testFS
	clock time.Time
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dir := t.TempDir()
	f := &fixture{t: t, dir: filepath.Join(dir, "work"), fs: &testFS{},
		clock: time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)}
	if err := os.MkdirAll(f.dir, 0o700); err != nil {
		t.Fatal(err)
	}
	f.svc = &core.Service{
		Store: core.Store{Home: filepath.Join(dir, "home")},
		Guard: core.Guard{Keep: []string{filepath.Join(dir, "keep")}, System: []string{filepath.Join(dir, "home")}},
		FS:    f.fs,
		Now:   func() time.Time { return f.clock },
	}
	return f
}

func (f *fixture) file(name string) string {
	f.t.Helper()
	path := filepath.Join(f.dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
		f.t.Fatal(err)
	}
	return path
}

func (f *fixture) add(path string, d time.Duration) core.Item {
	f.t.Helper()
	results, err := f.svc.Add([]string{path}, core.AddOptions{ExpiresAt: f.clock.Add(d)})
	if err != nil || len(results) != 1 || results[0].Error != "" {
		f.t.Fatalf("add %s: %v %+v", path, err, results)
	}
	return *results[0].Item
}

func (f *fixture) sweepAt(after time.Duration) []core.Outcome {
	f.t.Helper()
	f.clock = f.clock.Add(after)
	outcomes, err := f.svc.Sweep(false)
	if err != nil {
		f.t.Fatal(err)
	}
	return outcomes
}

func (f *fixture) items() []core.Item {
	f.t.Helper()
	state, err := f.svc.Store.Read()
	if err != nil {
		f.t.Fatal(err)
	}
	return state.Items
}

func exists(path string) bool { _, err := os.Lstat(path); return err == nil }

func unreadableSnapshot(t *testing.T, store core.Store) (core.State, func()) {
	t.Helper()
	st, err := store.ReadSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.Home, st.Graveyard.File)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("unreadable snapshot"), 0600); err != nil {
		t.Fatal(err)
	}
	restore := func() {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Error(err)
		}
	}
	t.Cleanup(restore)
	return st, restore
}

func samePath(a, b string) bool { return strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) }

func rejectPath(t *testing.T, f *fixture, operation, input string) bool {
	t.Helper()
	switch operation {
	case "purge":
		rows, err := f.svc.PurgeRefs([]string{input}, false)
		return err != nil || core.GraveFailed(rows)
	case "resurrect":
		rows, err := f.svc.Resurrect([]string{input})
		return err != nil || core.GraveFailed(rows)
	case "bury":
		rows, err := f.svc.Bury([]string{input}, false, "")
		return err != nil || core.GraveFailed(rows)
	case "add":
		rows, err := f.svc.Add([]string{input}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
		return err != nil || core.Failed(rows)
	case "propose":
		_, _, _, err := f.svc.Propose(core.ProposalLeftovers, f.dir, "session", []core.Candidate{{Path: input}})
		return err != nil
	default:
		t.Fatal("unknown operation", operation)
		return false
	}
}

func benchmarkGraveyard(b *testing.B, n int) (*core.Service, core.State) {
	b.Helper()
	base := b.TempDir()
	svc := &core.Service{Store: core.Store{Home: filepath.Join(base, "home")}, FS: winfs.FS{}}
	root := filepath.Join(svc.Store.Home, "graveyard")
	if err := os.MkdirAll(root, 0700); err != nil {
		b.Fatal(err)
	}
	now := time.Now()
	st := core.State{NextID: int64(2*n + 1)}
	for i := range n {
		path := filepath.Join(base, fmt.Sprintf("live-%d", i))
		if err := os.WriteFile(path, nil, 0600); err != nil {
			b.Fatal(err)
		}
		id, err := svc.FS.Identify(path)
		if err != nil {
			b.Fatal(err)
		}
		st.Items = append(st.Items, core.Item{ID: int64(i + 1), Rev: 1, Path: path, Identity: &id, ExpiresAt: now.Add(time.Hour)})
		grave := filepath.Join(root, fmt.Sprintf("%d-%s-abcdef", n+i+1, now.UTC().Format("20060102T150405Z")), "buried")
		if err := os.MkdirAll(filepath.Dir(grave), 0700); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(grave, nil, 0600); err != nil {
			b.Fatal(err)
		}
		graveID, err := svc.FS.Identify(filepath.Dir(grave))
		if err != nil {
			b.Fatal(err)
		}
		objectID, err := svc.FS.Identify(grave)
		if err != nil {
			b.Fatal(err)
		}
		st.Burials = append(st.Burials, core.Burial{ID: int64(n + i + 1), Path: filepath.Join(base, fmt.Sprintf("old-%d", i), "buried"), Grave: grave, Identity: &objectID, GraveID: &graveID, BuriedAt: now, PurgeAt: now.Add(24 * time.Hour), Reason: core.ReasonBury})
	}
	if err := svc.Store.Update(func(current *core.State) error { *current = st; return nil }); err != nil {
		b.Fatal(err)
	}
	return svc, st
}

type pausedRestoreFS struct {
	core.FS
	entered chan struct{}
	release chan struct{}
}

func (fs pausedRestoreFS) FindRestored(parent, id core.Identity) (string, error) {
	close(fs.entered)
	<-fs.release
	return fs.FS.FindRestored(parent, id)
}

type unreadableGraveFS struct {
	core.FS
	dir string
}

func (f unreadableGraveFS) Identify(path string) (core.Identity, error) {
	if samePath(path, f.dir) {
		return core.Identity{}, os.ErrPermission
	}
	return f.FS.Identify(path)
}

type moveHookFS struct {
	*testFS
	before  func()
	after   func(string)
	located func(string)
}

func (f *moveHookFS) Locate(id core.Identity) (string, error) {
	path, err := f.testFS.Locate(id)
	if err == nil && f.located != nil {
		fn := f.located
		f.located = nil
		fn(path)
	}
	return path, err
}

func (f *moveHookFS) MovePinned(path string, want core.Identity, to string) error {
	if f.before != nil {
		fn := f.before
		f.before = nil
		fn()
		// Model a handle opened before the interleaved operation: it still
		// refers to this object even if another move changed its name.
		var err error
		path, err = f.FS.Locate(want)
		if err != nil {
			return err
		}
	}
	if err := f.testFS.MovePinned(path, want, to); err != nil {
		return err
	}
	if f.after != nil {
		fn := f.after
		f.after = nil
		fn(to)
	}
	return nil
}

// MakeGrave is outside the ledger transaction, after the due item was claimed.
type prepareHookFS struct {
	*testFS
	before func()
}

func (f *prepareHookFS) MakeGrave(dir string) error {
	if f.before != nil {
		fn := f.before
		f.before = nil
		fn()
	}
	return f.testFS.MakeGrave(dir)
}

type identifyHookFS struct {
	*testFS
	after func(string)
}

func (f *identifyHookFS) Identify(path string) (core.Identity, error) {
	id, err := f.testFS.Identify(path)
	if f.after != nil {
		f.after(path)
	}
	return id, err
}

type rootFailureFS struct {
	*testFS
	root string
}

func (f rootFailureFS) RealPath(path string) (string, error) {
	if samePath(path, f.root) {
		return "", errors.New("graveyard root temporarily inaccessible")
	}
	return f.testFS.RealPath(path)
}

type blockingMoveFS struct {
	*testFS
	reached chan struct{}
	release chan struct{}
}

func (f blockingMoveFS) MovePinned(path string, want core.Identity, to string) error {
	close(f.reached)
	<-f.release
	return f.testFS.MovePinned(path, want, to)
}

type offlineGraveFS struct {
	*testFS
	root string
}

func (f offlineGraveFS) RealPath(path string) (string, error) {
	if samePath(path, f.root) || samePath(path, filepath.VolumeName(f.root)+`\`) {
		return "", core.ErrNotFound
	}
	return f.testFS.RealPath(path)
}

type scanFailureFS struct {
	*testFS
	root   string
	failed bool
}

func (f *scanFailureFS) RealPath(path string) (string, error) {
	if samePath(path, f.root) && !f.failed {
		f.failed = true
		return "", errors.New("root enumeration temporarily unavailable")
	}
	return f.testFS.RealPath(path)
}

type replacedVolumeFS struct {
	*testFS
	missing string
}

func (f replacedVolumeFS) RealPath(path string) (string, error) {
	if samePath(path, f.missing) {
		return "", core.ErrNotFound
	}
	return f.testFS.RealPath(path)
}

func (f replacedVolumeFS) Identify(path string) (core.Identity, error) {
	id, err := f.testFS.Identify(path)
	if samePath(path, filepath.VolumeName(path)+`\`) {
		id.Serial++ // same letter, another volume
	}
	return id, err
}

func conflictingManifest(t *testing.T, f *fixture, b core.Burial) string {
	t.Helper()
	path := filepath.Join(core.GraveyardRoot(f.svc, b.Path), "99-conflict.json")
	data, err := json.Marshal(struct {
		Version int `json:"version"`
		core.Burial
	}{1, b})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func interruptBurial(t *testing.T, f *fixture, path, operation string) (core.Item, string) {
	t.Helper()
	blocked := filepath.Join(f.svc.Store.Home, "store.json.tmp")
	var grave string
	f.svc.FS = &moveHookFS{testFS: f.fs, after: func(to string) {
		grave = to
		if err := os.Mkdir(blocked, 0o700); err != nil {
			t.Fatal(err)
		}
	}}
	var err error
	if operation == "sweep" {
		_, err = f.svc.Sweep(false)
	} else {
		_, err = f.svc.Bury([]string{path}, false, "")
	}
	if err == nil || grave == "" || exists(path) {
		t.Fatalf("failed to inject a post-move commit failure: %s, %v", grave, err)
	}
	if err := os.Remove(blocked); err != nil {
		t.Fatal(err)
	}
	f.svc.FS = f.fs
	if items := f.items(); len(items) > 0 {
		return items[0], grave
	}
	data, err := os.ReadFile(filepath.Dir(grave) + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var b core.Burial
	if err := json.Unmarshal(data, &b); err != nil {
		t.Fatal(err)
	}
	return core.Item{ID: b.ID, Path: b.Path, Identity: b.Identity}, grave
}

func rewriteManifest(t *testing.T, path string, change func(map[string]any)) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber() // volume serials are uint64; float64 would change identity
	if err := decoder.Decode(&m); err != nil {
		t.Fatal(err)
	}
	change(m)
	data, err = json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func containsIDs(t *testing.T, value any) []int64 {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Contains []int64 `json:"contains"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded.Contains
}

func committedManifest(t *testing.T, f *fixture) core.Burial {
	t.Helper()
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
		t.Fatalf("interruption: %v", err)
	}
	return f.burials()[0]
}
