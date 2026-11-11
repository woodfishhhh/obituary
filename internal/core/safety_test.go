package core_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func TestDefaultGuardProtectsSystemAndProfileLocations(t *testing.T) {
	guard := core.DefaultGuard(filepath.Join(os.Getenv("LOCALAPPDATA"), "Obituary"))
	for _, path := range []string{
		`C:\`,
		os.Getenv("SystemRoot"),
		filepath.Join(os.Getenv("SystemRoot"), "System32"),
		os.Getenv("USERPROFILE"),
		filepath.Dir(os.Getenv("USERPROFILE")),
		filepath.Join(os.Getenv("USERPROFILE"), "Downloads"),
		filepath.Join(os.Getenv("ProgramFiles"), "Git"),
	} {
		var refused *core.RefusedError
		if err := guard.Check(path, true); !errors.As(err, &refused) {
			t.Errorf("%s not refused: %v", path, err)
		}
	}
}

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

// cancelAfter reports cancellation once Err has been asked more than budget
// times, so a test can end a walk at an exact entry instead of a wall clock.
type cancelAfter struct {
	context.Context
	budget int
}

func (c *cancelAfter) Err() error {
	if c.budget--; c.budget < 0 {
		return context.Canceled
	}
	return nil
}

func TestGuardWalkStopsWhenItsBudgetEnds(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 50; i++ {
		if err := os.Mkdir(filepath.Join(dir, fmt.Sprintf("a%02d", i)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	// Lexical walk order puts this repository after every a* folder.
	if err := os.MkdirAll(filepath.Join(dir, "z", ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	ctx := &cancelAfter{Context: context.Background(), budget: 10}
	err := core.Guard{}.CheckContext(ctx, dir, false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("guard walk ignored its ended budget: %v", err)
	}
}
