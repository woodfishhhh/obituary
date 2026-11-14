package guard_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	core "github.com/woodfishhhh/obituary/internal/core/guard"
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

// repositoryTree holds a repository after 50 folders in lexical walk order.
func repositoryTree(t *testing.T) string {
	dir := t.TempDir()
	for i := 0; i < 50; i++ {
		if err := os.Mkdir(filepath.Join(dir, fmt.Sprintf("a%02d", i)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(dir, "z", ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestGuardWalkStopsWhenItsBudgetEnds(t *testing.T) {
	dir := repositoryTree(t)
	ctx := &cancelAfter{Context: context.Background(), budget: 10}
	err := core.Guard{}.CheckContext(ctx, dir, false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("guard walk ignored its ended budget: %v", err)
	}
}

// A forced check passes a contained repository anyway, so it must not pay for
// walking the tree; an unforced check still finds the repository.
func TestForcedCheckSkipsTheRepositoryWalk(t *testing.T) {
	dir := repositoryTree(t)
	if err := (core.Guard{}).CheckContext(&cancelAfter{Context: context.Background(), budget: 5}, dir, true); err != nil {
		t.Fatalf("forced check walked the tree: %v", err)
	}
	var refused *core.RefusedError
	if err := (core.Guard{}).Check(dir, false); !errors.As(err, &refused) || !refused.Forceable {
		t.Fatalf("unforced check missed the repository: %v", err)
	}
}
