//go:build windows

package auto

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/project"
	"github.com/woodfishhhh/obituary/internal/rules"
	"golang.org/x/sys/windows"
)

func TestSweepPreservesNestedRepositoryPublishedDuringDiscovery(t *testing.T) {
	if testing.Short() {
		t.Skip("real nested Git repositories and scheduled publication")
	}
	r := newRepo(t)
	now := time.Now()
	r.engine.Svc.Now = func() time.Time { return now }
	path := r.write("nested/keep.log", "keep")
	nested := &repo{t: t, root: filepath.Dir(path)}
	nested.git("init", "-q")
	nested.git("add", "--", "keep.log")
	gitdir, staged := filepath.Join(nested.root, ".git"), filepath.Join(r.root, "staged-git")
	if err := os.Rename(gitdir, staged); err != nil {
		t.Fatal(err)
	}
	if rows, err := r.engine.Svc.Add([]string{path}, core.AddOptions{ExpiresAt: now.Add(time.Hour), Source: "rule", Project: r.root}); err != nil || core.Failed(rows) {
		t.Fatalf("add: %+v %v", rows, err)
	}
	now = now.Add(time.Hour)
	published := false
	r.engine.Svc.PrepareOwners = func(ctx context.Context, _ []string) core.OwnerBatchFunc {
		observer, err := (project.Project{Root: r.root}).ObserveIndex(ctx)
		if err != nil {
			t.Fatal(err)
		}
		observers := map[string]preparedIndex{r.root: {observer: observer, afterFind: func() {
			if !published {
				if err := os.Rename(staged, gitdir); err != nil {
					t.Fatal(err)
				}
				published = true
				time.Sleep(32 * time.Millisecond)
			}
		}}}
		known := make(map[string]discoveredOwner)
		return func(ctx context.Context, paths []string) []core.Ownership {
			return queryOwners(ctx, paths, observers, known, nil)
		}
	}
	rows, err := r.engine.Svc.Sweep(false)
	root, tracked, gitErr := Owner(path)
	if !published || gitErr != nil || !tracked || root != nested.root {
		t.Fatalf("nested repository publication not reached: root=%s tracked=%v %v", root, tracked, gitErr)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil || string(data) != "keep" {
		t.Fatalf("project discovery stall moved newly tracked content: %+v %v", rows, readErr)
	}
	st, readErr := r.engine.Svc.Store.ReadSnapshot()
	if err != nil || readErr != nil || len(rows) != 1 || rows[0].Action != core.ActionRetry || len(st.Burials) != 0 || len(st.Items) != 1 || st.Items[0].Attempts != 1 {
		t.Fatalf("stale project discovery did not preserve a retry: %+v %+v %v %v", rows, st, err, readErr)
	}
}

func TestOwnerShortPathsMatchLongPaths(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	r := newRepo(t)
	root, err := filepath.EvalSymlinks(r.root)
	if err != nil {
		t.Fatal(err)
	}
	r.root = root
	r.write("App.log", "keep")
	r.commit("App.log")
	r.write("debug.log", "scratch")
	short := shortPath(t, root)
	for _, base := range []string{root, short} {
		paths := []string{filepath.Join(base, "app.log"), filepath.Join(base, "debug.log"), filepath.Join(base, "absent", "nested", "debug.log")}
		batch := OwnerBatch(t.Context(), paths)
		for i, path := range paths {
			got, tracked, err := Owner(path)
			if err != nil || got != root || tracked != (i == 0) {
				t.Errorf("Owner(%q) = %q, %v, %v", path, got, tracked, err)
			}
			if batch[i].Err != nil || batch[i].Root != root || batch[i].Tracked != (i == 0) {
				t.Errorf("OwnerBatch(%q) = %+v", path, batch[i])
			}
		}
	}
}

func TestOwnerBatchKeepsProjectFailuresSeparate(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r, other := newRepo(t), newRepo(t)
	paths := []string{r.write("debug.log", "untracked"), other.write("debug.log", "tracked")}
	other.git("add", "--", "debug.log")
	owners := OwnerBatch(t.Context(), paths)
	if len(owners) != 2 || owners[0].Err != nil || owners[0].Root != r.root || owners[0].Tracked || owners[1].Err != nil || !owners[1].Tracked || owners[1].Root != other.root {
		t.Fatalf("batch confused project ownership: %+v", owners)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	owners = OwnerBatch(ctx, paths)
	for _, owner := range owners {
		if owner.Err == nil || !owner.Tracked {
			t.Fatalf("canceled Git exposed untracked result: %+v", owners)
		}
	}
}

func TestOwnerBatchRefreshesAbsentWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	r := &repo{t: t, root: t.TempDir()}
	path := r.write("file.txt", "not yet in Git")
	query := PrepareOwners(t.Context(), []string{path})
	before := query(t.Context(), []string{path})
	if before[0].Err != nil || before[0].Root != "" || before[0].CheckedAt.IsZero() {
		t.Fatalf("missing worktree did not produce a fresh answer: %+v", before)
	}
	r.git("init", "-q")
	r.git("add", "--", "file.txt")
	after := query(t.Context(), []string{path})
	if after[0].Err != nil || !after[0].Tracked || after[0].Root != r.root {
		t.Fatalf("prepared absence hid new Git tracking: %+v", after)
	}
}

func TestSweepPreservesFileStagedBetweenMoves(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, batch := range []bool{true, false} {
		t.Run(fmt.Sprint(batch), func(t *testing.T) {
			r := newRepo(t)
			now := time.Now()
			r.engine.Svc.Now = func() time.Time { return now }
			paths := []string{r.write("first.log", "first"), r.write("second.log", "keep"), r.write("third.log", "third")}
			if rows, err := r.engine.Svc.Add(paths, core.AddOptions{ExpiresAt: now.Add(time.Hour), Source: "rule", Project: r.root}); err != nil || core.Failed(rows) {
				t.Fatalf("add: %+v %v", rows, err)
			}
			now = now.Add(time.Hour)
			r.git("add", "--", "second.log")
			index, staged := filepath.Join(r.root, ".git", "index"), filepath.Join(r.root, "staged-index")
			if err := os.Rename(index, staged); err != nil {
				t.Fatal(err)
			}
			if batch {
				r.engine.Svc.OwnerBatch = OwnerBatch
			}
			published := false
			r.engine.Svc.FS = &batchRuleChangeFS{FS: r.engine.Svc.FS, afterMove: func() {
				if err := os.Rename(staged, index); err != nil {
					t.Fatal(err)
				}
				published = true
			}}
			out, err := r.engine.Svc.Sweep(false)
			root, tracked, gitErr := Owner(paths[1])
			if err != nil || !published || gitErr != nil || !tracked || root != r.root {
				t.Fatalf("invalid staging fixture: %+v %v %v", out, err, gitErr)
			}
			data, readErr := os.ReadFile(paths[1])
			if readErr != nil || string(data) != "keep" {
				t.Fatalf("file staged after first move was buried: %+v %v", out, readErr)
			}
			st, ledgerErr := r.engine.Svc.Store.ReadSnapshot()
			if ledgerErr != nil || len(out) != 3 || out[0].Action != core.ActionBuried || out[1].Action != core.ActionRefused || out[2].Action != core.ActionBuried || len(st.Burials) != 2 || len(st.Items) != 0 {
				t.Fatalf("new Git protection blocked unrelated siblings: %+v %v", out, ledgerErr)
			}
		})
	}
}

func TestSweepQueriesGitAfterFinalDirectoryGuard(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	r := newRepo(t)
	now := time.Now()
	r.engine.Svc.Now = func() time.Time { return now }
	keep := r.write("scratch/keep.txt", "keep")
	dir := filepath.Dir(keep)
	for i := 0; i < 1000; i++ {
		r.write(fmt.Sprintf("scratch/file-%04d", i), "data")
	}
	if rows, err := r.engine.Svc.Add([]string{dir}, core.AddOptions{ExpiresAt: now.Add(time.Hour), Source: "rule", Project: r.root}); err != nil || core.Failed(rows) {
		t.Fatalf("add: %+v %v", rows, err)
	}
	now = now.Add(time.Hour)
	r.git("add", "--", "scratch/keep.txt")
	index, staged := filepath.Join(r.root, ".git", "index"), filepath.Join(r.root, "staged-index")
	if err := os.Rename(index, staged); err != nil {
		t.Fatal(err)
	}
	published := false
	r.engine.Svc.Guard.Canonical = func(path string) (string, error) {
		if strings.EqualFold(path, dir) && !published {
			if err := os.Rename(staged, index); err != nil {
				t.Fatal(err)
			}
			published = true
		}
		return path, nil
	}
	r.engine.Svc.OwnerBatch = OwnerBatch
	out, err := r.engine.Svc.Sweep(false)
	_, tracked, gitErr := Owner(keep)
	if err != nil || !published || gitErr != nil || !tracked {
		t.Fatalf("invalid directory fixture: %+v %v %v", out, err, gitErr)
	}
	data, readErr := os.ReadFile(keep)
	if readErr != nil || string(data) != "keep" {
		t.Fatalf("single directory used Git answer predating Guard: %+v %v", out, readErr)
	}
	if len(out) != 1 || out[0].Action != core.ActionRefused {
		t.Fatalf("directory Git recheck was not reached: %+v", out)
	}
}

func TestOwnerBatchDeadlineStopsRepositoryDiscovery(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	t.Run("expired", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		owners := OwnerBatch(ctx, []string{r.root + string(rune(0))})
		if len(owners) != 1 || !owners[0].Tracked || !errors.Is(owners[0].Err, context.Canceled) {
			t.Fatalf("expired discovery performed filesystem work: %+v", owners)
		}
	})
	t.Run("deep", func(t *testing.T) {
		t.Parallel()
		r := newRepo(t)
		paths := make([]string, 32)
		for i := range paths {
			paths[i] = r.write(strings.Repeat("d/", 70)+fmt.Sprintf("file-%02d.log", i), "keep")
		}
		ctx, cancel := context.WithTimeout(t.Context(), 250*time.Millisecond)
		defer cancel()
		readDone := make(chan error, 1)
		start := time.Now()
		err := r.engine.Svc.Store.Update(func(*core.State) error {
			go func() {
				_, err := r.engine.Svc.Store.WithDeadline(time.Now().Add(time.Second)).ReadSnapshot()
				readDone <- err
			}()
			owners := OwnerBatch(ctx, paths)
			t.Logf("discovery elapsed=%s answers=%d", time.Since(start), len(owners))
			for _, owner := range owners {
				if owner.Err != nil && !owner.Tracked {
					t.Errorf("failed discovery exposed untracked answer: %+v", owner)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := <-readDone; err != nil {
			t.Fatalf("repository discovery exceeded hook lock budget: %s %v", time.Since(start), err)
		}
	})
}

func TestSweepBatchRechecksGitAndRules(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, protect := range []string{"git", "rules"} {
		t.Run(protect, func(t *testing.T) {
			r := newRepo(t)
			now := time.Now()
			r.engine.Svc.Now = func() time.Time { return now }
			paths := []string{r.write("first.log", "first"), r.write("second.log", "keep"), r.write("third.log", "third")}
			for _, path := range paths {
				if d := r.apply(path)[path]; d.Action != Registered {
					t.Fatalf("register: %+v", d)
				}
			}
			st, err := r.engine.Svc.Store.ReadSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			now = st.Items[len(st.Items)-1].ExpiresAt.Add(time.Second)
			if protect == "rules" {
				file := r.write(rules.FileName, "!second.log\n")
				if _, err := r.engine.Trust.Trust(file, ""); err != nil {
					t.Fatal(err)
				}
			} else {
				// Build an actual Git index before the timed transaction, then
				// publish it during the rule check. Starting an extra git add in
				// the seam consumes the production query budget under machine load.
				r.git("add", "--", "second.log")
				index, staged := filepath.Join(r.root, ".git", "index"), filepath.Join(r.root, "staged-index")
				if err := os.Rename(index, staged); err != nil {
					t.Fatal(err)
				}
				published := false
				r.engine.Svc.Recheck = func(root, path string, isDir bool) error {
					if path == paths[1] && !published {
						if err := os.Rename(staged, index); err != nil {
							t.Fatal(err)
						}
						published = true
					}
					return r.engine.RecheckRules(root, path, isDir)
				}
			}
			calls := 0
			r.engine.Svc.OwnerBatch = func(ctx context.Context, paths []string) []core.Ownership {
				calls++
				// Prefetch measures startup outside the lock. Without Current,
				// each subsequent answer must still come from the moving transaction.
				_, readErr := r.engine.Svc.Store.WithDeadline(time.Now().Add(5 * time.Millisecond)).ReadSnapshot()
				if (calls == 1) != (readErr == nil) {
					t.Error("Git prefetch or final query used the wrong ledger lock boundary")
				}
				return OwnerBatch(ctx, paths)
			}
			out, err := r.engine.Svc.Sweep(false)
			st, readErr := r.engine.Svc.Store.ReadSnapshot()
			data, fileErr := os.ReadFile(paths[1])
			if fileErr != nil || string(data) != "keep" {
				t.Fatalf("batch automatic recheck moved protected item: %+v %v", out, fileErr)
			}
			if err != nil || readErr != nil || calls == 0 || len(st.Burials) != 2 || len(st.Items) != 0 {
				t.Fatalf("batch automatic recheck blocked siblings: %+v calls=%d %v %v", out, calls, err, readErr)
			}
		})
	}
}

type batchRuleChangeFS struct {
	core.FS
	afterMove   func()
	gitAnswered map[string]time.Time
	maxGitGap   time.Duration
	moves       int
}

func (fs *batchRuleChangeFS) MovePinned(path string, want core.Identity, to string) error {
	if fs.gitAnswered != nil {
		answered := fs.gitAnswered[path]
		if answered.IsZero() {
			return fmt.Errorf("missing Git answer timestamp for %s", path)
		}
		fs.maxGitGap = max(fs.maxGitGap, time.Since(answered))
		fs.moves++
	}
	if err := fs.FS.MovePinned(path, want, to); err != nil {
		return err
	}
	if fs.afterMove != nil {
		change := fs.afterMove
		fs.afterMove = nil
		change()
	}
	return nil
}

func TestSweepFinalGitToMoveWindow(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	r := newRepo(t)
	now := time.Now()
	r.engine.Svc.Now = func() time.Time { return now }
	paths := make([]string, 32)
	for i := range paths {
		paths[i] = r.write(fmt.Sprintf("timed-%02d.log", i), "payload")
	}
	if rows, err := r.engine.Svc.Add(paths, core.AddOptions{ExpiresAt: now.Add(time.Hour), Source: "rule", Project: r.root}); err != nil || core.Failed(rows) {
		t.Fatalf("add: %+v %v", rows, err)
	}
	now = now.Add(time.Hour)
	answered := make(map[string]time.Time)
	calls := 0
	r.engine.Svc.OwnerBatch = func(ctx context.Context, paths []string) []core.Ownership {
		owners := OwnerBatch(ctx, paths)
		calls++
		for i, path := range paths {
			answered[path] = owners[i].CheckedAt
		}
		return owners
	}
	fs := &batchRuleChangeFS{FS: r.engine.Svc.FS, gitAnswered: answered}
	r.engine.Svc.FS = fs
	out, err := r.engine.Svc.Sweep(false)
	if err != nil || fs.moves != len(paths) {
		t.Fatalf("timed sweep incomplete: moves=%d %+v %v", fs.moves, out, err)
	}
	if fs.maxGitGap > 10*time.Millisecond {
		t.Fatalf("final Git answer exceeded move window: %s", fs.maxGitGap)
	}
	t.Logf("real Git samples=%d queries=%d maximum answer-to-MovePinned-entry=%s", fs.moves, calls, fs.maxGitGap)
}

func TestSweepBatchRechecksRulesBeforeEachMove(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	r := newRepo(t)
	now := time.Now()
	r.engine.Svc.Now = func() time.Time { return now }
	paths := []string{r.write("first.log", "first"), r.write("second.log", "keep"), r.write("third.log", "third")}
	for _, path := range paths {
		if d := r.apply(path)[path]; d.Action != Registered {
			t.Fatalf("register: %+v", d)
		}
	}
	st, err := r.engine.Svc.Store.ReadSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	now = st.Items[len(st.Items)-1].ExpiresAt.Add(time.Second)
	file := r.write(rules.FileName, "!second.log\n")
	if _, err := r.engine.Trust.Trust(file, ""); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(file, file+".pending"); err != nil {
		t.Fatal(err)
	}
	if was, err := r.engine.Trust.Untrust(file); err != nil || !was {
		t.Fatalf("untrust staged rules: %v %v", was, err)
	}
	r.engine.Svc.OwnerBatch = OwnerBatch
	r.engine.Svc.FS = &batchRuleChangeFS{FS: r.engine.Svc.FS, afterMove: func() {
		if err := os.Rename(file+".pending", file); err != nil {
			t.Fatal(err)
		}
		if _, err := r.engine.Trust.Trust(file, ""); err != nil {
			t.Fatal(err)
		}
	}}
	out, err := r.engine.Svc.Sweep(false)
	data, readErr := os.ReadFile(paths[1])
	st, ledgerErr := r.engine.Svc.Store.ReadSnapshot()
	if readErr != nil || string(data) != "keep" {
		t.Fatalf("batch ignored a rule published after its first move: %+v %v %v", out, err, readErr)
	}
	if err != nil || ledgerErr != nil || len(st.Burials) != 2 || len(st.Items) != 0 {
		t.Fatalf("new rule blocked unrelated siblings: %+v %v %v", out, err, ledgerErr)
	}
}

func TestMkTempRejectsNamedStreamLocation(t *testing.T) {
	t.Parallel()
	for _, extended := range []bool{false, true} {
		t.Run(fmt.Sprint(extended), func(t *testing.T) {
			t.Parallel()
			r := newEngineFixture(t)
			cwd := r.root + ":review"
			if extended {
				cwd = `\\?\` + cwd
			}
			path, item, err := r.engine.MkTemp(time.Hour, "stream", cwd, "session")
			if err == nil || !strings.Contains(err.Error(), "whole files or directories") || path != "" || item != nil {
				t.Fatalf("stream location created temporary directory: %s %+v %v", path, item, err)
			}
		})
	}
}

func TestMkTempRequiresDirectoryCwd(t *testing.T) {
	t.Parallel()
	for _, kind := range []string{"file", "file-with-separator", "missing", "directory"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			r := newEngineFixture(t)
			cwd := r.write("payload.txt", "keep")
			switch kind {
			case "file-with-separator":
				cwd += `\`
			case "missing":
				cwd = filepath.Join(r.root, "missing")
			case "directory":
				cwd = r.root + `\`
			}
			path, item, err := r.engine.MkTemp(time.Hour, "scratch", cwd, "s")
			if kind == "directory" {
				if err != nil || item == nil || path == "" {
					t.Fatalf("directory cwd rejected: %s %+v %v", path, item, err)
				}
				return
			}
			if err == nil || path != "" || item != nil {
				t.Fatalf("non-directory cwd created temporary directory: %s %+v %v", path, item, err)
			}
			if _, err := os.Stat(filepath.Join(r.root, ".obit")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("rejected cwd changed project: %v", err)
			}
		})
	}
}

type cancelTempFS struct {
	core.FS
	cancel context.CancelFunc
	fill   bool
}

func (f cancelTempFS) Identify(path string) (core.Identity, error) {
	id, err := f.FS.Identify(path)
	if err == nil && strings.Contains(path, "rollback-") {
		if f.fill {
			err = os.WriteFile(filepath.Join(path, "concurrent.txt"), []byte("keep"), 0o600)
		}
		f.cancel()
	}
	return id, err
}

func TestMkTempReportsFailedRollback(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, fill := range []bool{false, true} {
		r := newRepo(t)
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		r.engine.Svc = r.engine.Svc.WithContext(ctx)
		r.engine.Svc.FS = cancelTempFS{FS: r.engine.Svc.FS, cancel: cancel, fill: fill}
		path, item, err := r.engine.MkTemp(time.Hour, "rollback", r.root, "s")
		if !errors.Is(err, context.Canceled) || item != nil {
			t.Fatalf("canceled registration: %s %+v %v", path, item, err)
		}
		if fill {
			if path == "" || !strings.Contains(err.Error(), path) || !strings.Contains(err.Error(), "rollback") {
				t.Fatalf("failed rollback lost leftover path: %q %v", path, err)
			}
			if data, err := os.ReadFile(filepath.Join(path, "concurrent.txt")); err != nil || string(data) != "keep" {
				t.Fatalf("rollback removed concurrent content: %s %v", data, err)
			}
		} else {
			if path != "" {
				t.Fatalf("successful rollback returned leftover: %s", path)
			}
			entries, err := os.ReadDir(filepath.Join(r.root, ".obit", "tmp"))
			if err != nil || len(entries) != 0 {
				t.Fatalf("rollback left directory: %v %v", entries, err)
			}
		}
		st, err := r.engine.Svc.Store.Inspect()
		if err != nil || len(st.Items) != 0 {
			t.Fatalf("canceled registration committed: %+v %v", st, err)
		}
	}
}

func TestMkTempExpandsShortNamesBeforeRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	p, _ := windows.UTF16PtrFromString(r.root)
	buf := make([]uint16, 32768)
	n, err := windows.GetShortPathName(p, &buf[0], uint32(len(buf)))
	if err != nil {
		t.Fatal(err)
	}
	short := windows.UTF16ToString(buf[:n])
	if strings.EqualFold(short, r.root) {
		t.Skip("8.3 aliases unavailable")
	}
	path, item, err := r.engine.MkTemp(time.Hour, "longnames", short, "s")
	if err != nil {
		t.Fatal(err)
	}
	want, _, err := r.engine.Svc.FS.LongPath(path)
	if err != nil || path != want || item.Path != want || item.Project != r.root || !strings.HasPrefix(item.Path, filepath.Join(r.root, ".obit", "tmp")+string(os.PathSeparator)) {
		t.Fatalf("mktemp registered short spelling: %+v %v", item, err)
	}
	if err := os.WriteFile(filepath.Join(path, "probe.txt"), []byte("scratch"), 0600); err != nil {
		t.Fatal(err)
	}
	if status := r.git("status", "--porcelain", "--untracked-files=all"); status != "" {
		t.Fatalf("short cwd exposed mktemp to Git: %q", status)
	}
}
