package project

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/testenv"
)

func snapshotRepo(t *testing.T, initArgs ...string) (Project, func(...string) string) {
	t.Helper()
	p := Project{Root: t.TempDir()}
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.Command("git", append([]string{"-C", p.Root}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %s %v", args, out, err)
		}
		return strings.TrimSpace(string(out))
	}
	if len(initArgs) == 0 {
		testenv.InitRepo(t, p.Root)
	} else {
		git(append([]string{"init", "-q"}, initArgs...)...)
	}
	return p, git
}

func snapshotFile(t *testing.T, root, name string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(name), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestIndexSnapshotTrackedModes(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, mode := range []string{"ordinary", "assume-unchanged", "skip-worktree", "split", "sparse", "sha256"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			var args []string
			if mode == "sha256" {
				args = []string{"--object-format=sha256"}
			}
			p, git := snapshotRepo(t, args...)
			kept := snapshotFile(t, p.Root, "kept/file.log")
			snapshotFile(t, p.Root, "omitted/keep.log")
			free := snapshotFile(t, p.Root, "free.log")
			git("add", "--", "kept/file.log", "omitted/keep.log")
			git("commit", "-qm", "seed")
			switch mode {
			case "assume-unchanged", "skip-worktree":
				git("update-index", "--"+mode, "kept/file.log")
			case "split":
				git("config", "core.splitIndex", "true")
				git("update-index", "--split-index")
			case "sparse":
				git("sparse-checkout", "init", "--cone", "--sparse-index")
				git("sparse-checkout", "set", "kept")
			}
			observer, err := p.ObserveIndex(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			paths := []string{kept, free, filepath.Join(p.Root, "omitted")}
			tracked, snapshot, err := observer.TrackedSnapshot(t.Context(), paths)
			if mode == "sparse" && errors.Is(err, ErrIndexSnapshotUnavailable) {
				tracked, err = p.TrackedBatchContext(t.Context(), paths)
				if err != nil || !reflect.DeepEqual(tracked, []bool{true, false, true}) {
					t.Fatalf("fresh sparse query changed membership: %v %v", tracked, err)
				}
				return
			}
			if err != nil || snapshot == nil || !reflect.DeepEqual(tracked, []bool{true, false, true}) {
				t.Fatalf("snapshot changed Git membership: %v %v", tracked, err)
			}
			if current, err := snapshot.Current(t.Context()); !current || err != nil {
				t.Fatalf("unchanged snapshot invalid: %v %v", current, err)
			}
		})
	}
}

func TestIndexRealSnapshotRechecksPublication(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	p, git := snapshotRepo(t)
	keep := snapshotFile(t, p.Root, "keep.log")
	free := snapshotFile(t, p.Root, "free.log")
	git("add", "--", "keep.log")
	observer, err := p.ObserveIndex(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	paths := []string{keep, free}
	tracked, snapshot, err := observer.TrackedRealSnapshot(t.Context(), paths)
	if err != nil || snapshot == nil || !reflect.DeepEqual(tracked, []bool{true, false}) {
		t.Fatalf("prepared names changed membership: %v %v", tracked, err)
	}
	git("add", "--", "free.log")
	if current, err := snapshot.Current(t.Context()); current || err != nil {
		t.Fatalf("prepared names hid index publication: %v %v", current, err)
	}
	tracked, _, err = observer.TrackedRealSnapshot(t.Context(), paths)
	if err != nil || !reflect.DeepEqual(tracked, []bool{true, true}) {
		t.Fatalf("prepared names reused stale membership: %v %v", tracked, err)
	}
}

func TestIndexSnapshotRejectsQueryPublication(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	p, git := snapshotRepo(t)
	snapshotFile(t, p.Root, "seed.txt")
	path := snapshotFile(t, p.Root, "keep.log")
	git("add", "--", "seed.txt")
	observer, err := p.ObserveIndex(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	observer.afterIndexQuery = func() { git("add", "--", "keep.log") }
	_, snapshot, err := observer.TrackedSnapshot(t.Context(), []string{path})
	if !errors.Is(err, errIndexChanged) || snapshot != nil {
		t.Fatalf("query publication retained stale authority: %v %v", snapshot, err)
	}
}

func TestIndexSnapshotRejectsSharedContentChange(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	p, git := snapshotRepo(t)
	snapshotFile(t, p.Root, "seed.txt")
	path := snapshotFile(t, p.Root, "keep.log")
	git("add", "--", "seed.txt")
	git("update-index", "--split-index")
	shared := git("rev-parse", "--path-format=absolute", "--shared-index-path")
	data, err := os.ReadFile(shared)
	if err != nil {
		t.Fatal(err)
	}
	observer, err := p.ObserveIndex(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	observer.afterIndexQuery = func() {
		data[len(data)-1] ^= 1
		if err := os.WriteFile(shared, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	_, snapshot, err := observer.TrackedSnapshot(t.Context(), []string{path})
	if !errors.Is(err, errIndexChanged) || snapshot != nil {
		t.Fatalf("changed shared bytes retained query authority: %v %v", snapshot, err)
	}
}

func TestIndexSnapshotGitfileRerouting(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	p, _ := snapshotRepo(t, "--separate-git-dir", filepath.Join(t.TempDir(), "metadata"))
	path := snapshotFile(t, p.Root, "keep.log")
	other, git := snapshotRepo(t)
	snapshotFile(t, other.Root, "keep.log")
	git("add", "--", "keep.log")
	observer, err := p.ObserveIndex(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	tracked, snapshot, err := observer.TrackedSnapshot(t.Context(), []string{path})
	if err != nil || snapshot == nil || tracked[0] {
		t.Fatalf("routing fixture: %v %v", tracked, err)
	}
	gitfile := filepath.Join(p.Root, ".git")
	if err := os.WriteFile(gitfile, []byte("gitdir: "+filepath.ToSlash(filepath.Join(other.Root, ".git"))+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if current, err := snapshot.Current(t.Context()); current && err == nil {
		t.Fatal("Gitfile rerouting left stale index authority")
	}
	tracked, _, err = observer.TrackedSnapshot(t.Context(), []string{path})
	if err != nil || len(tracked) != 1 || !tracked[0] {
		t.Fatalf("rerouted repository was not queried: %v %v", tracked, err)
	}
}

func TestIndexSnapshotHonorsCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	p, _ := snapshotRepo(t)
	observer, err := p.ObserveIndex(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	_, snapshot, err := observer.TrackedSnapshot(t.Context(), []string{snapshotFile(t, p.Root, "free.log")})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if current, err := snapshot.Current(ctx); current || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled snapshot retained move authority: %v %v", current, err)
	}
}

func TestIndexSnapshotSharedCaseAlias(t *testing.T) {
	testenv.WindowsOnly(t, "Windows split-index filenames are case insensitive")
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	p, git := snapshotRepo(t)
	snapshotFile(t, p.Root, "seed.txt")
	path := snapshotFile(t, p.Root, "free.log")
	git("add", "--", "seed.txt")
	git("update-index", "--split-index")
	shared := git("rev-parse", "--path-format=absolute", "--shared-index-path")
	upper := filepath.Join(filepath.Dir(shared), strings.ToUpper(filepath.Base(shared)))
	if err := os.Rename(shared, upper); err != nil {
		t.Fatal(err)
	}
	observer, err := p.ObserveIndex(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	tracked, snapshot, err := observer.TrackedSnapshot(t.Context(), []string{path})
	if err != nil || snapshot == nil || tracked[0] {
		t.Fatalf("case-alias Git fixture: %v %v", tracked, err)
	}
	if err := os.WriteFile(upper+".replacement", []byte("broken index"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(upper+".replacement", upper); err != nil {
		t.Fatal(err)
	}
	if current, err := snapshot.Current(t.Context()); current && err == nil {
		t.Fatal("case-alias shared index retained authority")
	}
}

func TestIndexSnapshotManySharedVersionsUseFreshQueries(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	p, git := snapshotRepo(t)
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("seed-%02d.txt", i)
		snapshotFile(t, p.Root, name)
		git("add", "--", name)
		git("update-index", "--split-index")
	}
	path := snapshotFile(t, p.Root, "free.log")
	observer, err := p.ObserveIndex(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	files, err := observer.sharedFiles(t.Context())
	if err != nil || len(files) < 9 {
		t.Fatalf("many-shared fixture: %d %v", len(files), err)
	}
	tracked, snapshot, err := observer.TrackedSnapshot(t.Context(), []string{path})
	if !errors.Is(err, ErrIndexSnapshotUnavailable) || snapshot != nil {
		t.Fatalf("large metadata set offered reusable authority: %v %v %v", tracked, snapshot, err)
	}
	tracked, err = p.TrackedBatchContext(t.Context(), []string{path})
	if err != nil || len(tracked) != 1 || tracked[0] {
		t.Fatalf("fresh query: %v %v", tracked, err)
	}
	git("add", "--", "free.log")
	tracked, err = p.TrackedBatchContext(t.Context(), []string{path})
	if err != nil || len(tracked) != 1 || !tracked[0] {
		t.Fatalf("uncached query missed Git publication: %v %v", tracked, err)
	}
}

func TestIndexSnapshotSparseTreeReplacement(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	p, git := snapshotRepo(t)
	snapshotFile(t, p.Root, "kept/file.log")
	snapshotFile(t, p.Root, "omitted/old.log")
	git("add", "--", "kept/file.log", "omitted/old.log")
	git("commit", "-qm", "seed")
	oldTree := git("rev-parse", "HEAD:omitted")
	blob := git("rev-parse", "HEAD:kept/file.log")
	cmd := exec.Command("git", "-C", p.Root, "mktree")
	cmd.Stdin = strings.NewReader("100644 blob " + blob + "\tkeep.log\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("replacement tree: %s %v", out, err)
	}
	newTree := strings.TrimSpace(string(out))
	git("sparse-checkout", "init", "--cone", "--sparse-index")
	git("sparse-checkout", "set", "kept")
	path := snapshotFile(t, p.Root, "omitted/keep.log")
	observer, err := p.ObserveIndex(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	tracked, snapshot, err := observer.TrackedSnapshot(t.Context(), []string{path})
	if errors.Is(err, ErrIndexSnapshotUnavailable) {
		tracked, err = p.TrackedBatchContext(t.Context(), []string{path})
	}
	if err != nil || len(tracked) != 1 || tracked[0] {
		t.Fatalf("sparse replacement fixture: %v %v", tracked, err)
	}
	git("replace", oldTree, newTree)
	var current bool
	var currentErr error
	if snapshot != nil {
		current, currentErr = snapshot.Current(t.Context())
	}
	tracked, err = p.TrackedBatchContext(t.Context(), []string{path})
	if err != nil || !tracked[0] {
		t.Fatalf("replacement did not change sparse membership: %v %v", tracked, err)
	}
	if current && currentErr == nil {
		t.Fatal("sparse object replacement retained stale authority")
	}
}

func TestIndexDigestBoundsSparseMarker(t *testing.T) {
	t.Parallel()
	for _, offset := range []int{64*1024 - 3, 64*1024 - 2, 64*1024 - 1} {
		path := filepath.Join(t.TempDir(), "index")
		data := append(make([]byte, offset), []byte("sdir")...)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := indexDigest(t.Context(), path); !errors.Is(err, ErrIndexSnapshotUnavailable) {
			t.Fatalf("split sparse marker retained cache authority: offset=%d error=%v", offset, err)
		}
	}
	path := snapshotFile(t, t.TempDir(), "index")
	metadata, err := indexMetadata(path, true)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := &IndexSnapshot{files: []indexFile{metadata}}
	if current, err := snapshot.Current(t.Context()); !current || err != nil {
		t.Fatalf("unchanged snapshot lost authority: %v %v", current, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if current, err := snapshot.Current(ctx); current || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled snapshot retained authority: %v %v", current, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if current, err := snapshot.Current(t.Context()); current {
		t.Fatalf("missing index retained authority: %v", err)
	}
}
