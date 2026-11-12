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

// Leftover creates name/index.js in the repository at root and lists /name/
// in its info/exclude, so Git ignores it. With a regenerable name such as
// node_modules or dist it is the kind of session leftover Obituary proposes.
// It returns the directory.
func Leftover(t testing.TB, root, name string) string {
	t.Helper()
	exclude := filepath.Join(root, ".git", "info", "exclude")
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(exclude), 0o700); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(exclude, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = fmt.Fprintf(f, "/%s/\n", filepath.ToSlash(name))
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = os.MkdirAll(dir, 0o700)
	}
	if err == nil {
		err = os.WriteFile(filepath.Join(dir, "index.js"), []byte("generated"), 0o600)
	}
	if err != nil {
		t.Fatal(err)
	}
	return dir
}
