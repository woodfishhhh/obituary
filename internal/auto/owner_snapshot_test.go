package auto

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPreparedOwnersShortPaths(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	r := newRepo(t)
	r.write("keep.log", "keep")
	r.git("add", "--", "keep.log")
	r.write("debug.log", "scratch")
	short := shortPath(t, r.root)
	paths := []string{filepath.Join(short, "keep.log"), filepath.Join(short, "debug.log")}
	query := PrepareOwners(t.Context(), paths)
	owners := query(t.Context(), paths)
	for i, owner := range owners {
		if owner.Err != nil || owner.Root != r.root || owner.Tracked != (i == 0) || owner.Current == nil {
			t.Fatalf("prepared alias lost index authority: %+v", owners)
		}
		if current, _, err := owner.Current(t.Context()); !current || err != nil {
			t.Fatalf("prepared alias snapshot: %v %v", current, err)
		}
	}
}

func TestPreparedOwnersSeparateMetadataFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r, other := newRepo(t), newRepo(t)
	paths := []string{r.write("free.log", "free"), other.write("keep.log", "keep")}
	other.git("add", "--", "keep.log")
	query := PrepareOwners(t.Context(), []string{r.root, other.root})
	if err := os.WriteFile(filepath.Join(other.root, ".git", "index"), []byte("broken index"), 0600); err != nil {
		t.Fatal(err)
	}
	owners := query(t.Context(), paths)
	if len(owners) != 2 || owners[0].Err != nil || owners[0].Tracked || owners[0].Current == nil || owners[1].Err == nil || !owners[1].Tracked {
		t.Fatalf("snapshot metadata failure crossed projects or failed open: %+v", owners)
	}
	if current, _, err := owners[0].Current(t.Context()); err != nil || !current {
		t.Fatalf("independent project lost authority: %v %v", current, err)
	}
}

func TestPreparedOwnersNoticeNestedRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	path := r.write("nested/keep.log", "keep")
	query := PrepareOwners(t.Context(), []string{r.root})
	owners := query(t.Context(), []string{path})
	if owners[0].Err != nil || owners[0].Tracked || owners[0].Current == nil {
		t.Fatalf("nested fixture: %+v", owners)
	}
	nested := filepath.Dir(path)
	for _, args := range [][]string{{"init", "-q"}, {"add", "--", "keep.log"}} {
		if out, err := exec.Command("git", append([]string{"-C", nested}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("nested git: %s %v", out, err)
		}
	}
	if current, _, err := owners[0].Current(t.Context()); current && err == nil {
		t.Fatal("nested repository reused outer index authority")
	}
	owners = query(t.Context(), []string{path})
	if owners[0].Err != nil || !owners[0].Tracked || owners[0].Root != nested {
		t.Fatalf("new nested repository was not queried: %+v", owners)
	}
}

func TestPreparedOwnersMissingIncludeUsesLiveGit(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	path := r.write("keep.log", "keep")
	r.write("repo/keep.log", "tracked")
	r.git("add", "--", "repo/keep.log")
	include := filepath.Join(r.root, "optional.config")
	r.git("config", "include.path", include)
	query := PrepareOwners(t.Context(), []string{path})
	owners := query(t.Context(), []string{path})
	if owners[0].Err != nil || owners[0].Tracked {
		t.Fatalf("missing include fixture: %+v", owners)
	}
	r.git("config", "--file", include, "core.worktree", filepath.Dir(r.root))
	if owners[0].Current != nil {
		if current, _, err := owners[0].Current(t.Context()); current && err == nil {
			t.Fatal("missing include created stale authority")
		}
	}
	owners = query(t.Context(), []string{path})
	root, tracked, err := Owner(path)
	if err != nil || owners[0].Err != nil || owners[0].Root != root || owners[0].Tracked != tracked {
		t.Fatalf("live query missed new include: %+v", owners)
	}
}

func TestPreparedOwnersVirtualConfigUsesLiveGit(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", "NUL")
	r := newRepo(t)
	path := r.write("keep.log", "keep")
	query := PrepareOwners(t.Context(), []string{path})
	owners := query(t.Context(), []string{path})
	if owners[0].Err != nil || owners[0].Tracked {
		t.Fatalf("virtual configuration blocked live Git: %+v", owners)
	}
	r.git("add", "--", "keep.log")
	owners = query(t.Context(), []string{path})
	if owners[0].Err != nil || !owners[0].Tracked {
		t.Fatalf("virtual config query missed publication: %+v", owners)
	}
}
