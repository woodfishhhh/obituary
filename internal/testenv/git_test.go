package testenv_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/woodfishhhh/obituary/internal/testenv"
)

// Called by the existing isolated Git process test, retaining its test name
// and keeping real Git verification out of the short tier.
func assertGitTemplateCopies(t *testing.T) {
	t.Helper()
	fresh, first, second := t.TempDir(), t.TempDir(), t.TempDir()
	fixtureGit(t, fresh, "init", "-q")
	testenv.InitRepo(t, first)
	testenv.InitRepo(t, second)
	for _, path := range []string{first, second} {
		for _, args := range [][]string{{"symbolic-ref", "HEAD"}, {"status", "--porcelain=v1"}, {"ls-files", "-z"}} {
			if got, want := fixtureGit(t, path, args...), fixtureGit(t, fresh, args...); got != want {
				t.Fatalf("copied Git %v = %q, fresh = %q", args, got, want)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(first, "keep.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, first, "add", "keep.txt")
	fixtureGit(t, first, "commit", "-qm", "independent")
	fixtureGit(t, first, "update-index", "--refresh")
	if got := fixtureGit(t, first, "status", "--porcelain=v1"); got != "" {
		t.Fatalf("copied index reports a false modification: %q", got)
	}
	if got := fixtureGit(t, second, "ls-files", "-z"); got != "" {
		t.Fatalf("copied repositories share an index: %q", got)
	}
	if got := fixtureGit(t, second, "status", "--porcelain=v1"); got != "" {
		t.Fatalf("other copy changed: %q", got)
	}
}

func fixtureGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s %v", args, out, err)
	}
	return string(out)
}
