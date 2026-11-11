package project

import (
	"path/filepath"
	"testing"
)

func TestIndexConfigurationNoticesAbsentGlobalFile(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(t.TempDir(), "new.config"))
	p, git := snapshotRepo(t)
	path := snapshotFile(t, p.Root, "keep.log")
	trackedName := filepath.ToSlash(filepath.Join(filepath.Base(p.Root), "keep.log"))
	snapshotFile(t, p.Root, trackedName)
	git("add", "--", trackedName)
	observer, err := p.ObserveIndex(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	tracked, snapshot, err := observer.TrackedSnapshot(t.Context(), []string{path})
	if err != nil || snapshot == nil || tracked[0] {
		t.Fatalf("missing global config fixture: %v %v", tracked, err)
	}
	git("config", "--global", "core.worktree", filepath.Dir(p.Root))
	if current, err := snapshot.Current(t.Context()); current && err == nil {
		t.Fatal("new global config reused stale authority")
	}
}
