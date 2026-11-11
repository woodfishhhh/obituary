package testenv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

var gitTemplateRoot string // owned and removed by Main

var gitTemplate = sync.OnceValues(func() (string, error) {
	path := filepath.Join(gitTemplateRoot, "git-template")
	// No initial commit/index: copying cannot preserve stale index stat data.
	// Use an empty template to avoid developer hooks and unused example hooks.
	cmd := exec.Command("git", "init", "-q", "--template=", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git template: %w: %s", err, out)
	}
	return path, nil
})

// InitRepo copies a package-local real Git repository. Go's os.CopyFS handles
// the tree copy; every test owns independent files, refs and object storage.
// The template stays under TestMain's scratch until all parallel tests finish.
func InitRepo(t testing.TB, path string) {
	t.Helper()
	template, err := gitTemplate()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(path, os.DirFS(template)); err != nil {
		t.Fatal(err)
	}
}
