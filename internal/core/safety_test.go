package core_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func TestLinkedParentCannotReachAProtectedFolder(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	f := newFixture(t)
	root := filepath.Dir(f.dir)
	keep := filepath.Join(root, "keep")
	if err := os.MkdirAll(keep, 0o700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(f.dir, "alias")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", alias, root).CombinedOutput(); err != nil {
		t.Fatalf("mklink: %v %s", err, out)
	}
	f.svc.Guard = f.svc.Guard.WithCanonical(winfs.Canonical)

	results, err := f.svc.Add([]string{filepath.Join(alias, "keep")}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if err != nil || results[0].Error == "" {
		t.Fatalf("keep reached through a junctioned parent must be refused, got %+v %v", results, err)
	}
}
