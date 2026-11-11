package core

import (
	"os"
	"testing"
	"time"
)

func init() { syncFile = func(*os.File) error { return nil } }

// RequireFsync is only available in core's test binary. Like SetReplacementSync,
// it is for serial tests: callers and their subtests must not use t.Parallel.
// The core suite is serial, including all recovery and flush-failure tests.
func RequireFsync(t *testing.T) {
	t.Helper()
	previous := syncFile
	syncFile = (*os.File).Sync
	t.Cleanup(func() { syncFile = previous })
}

func SyncFileForTesting(file *os.File) error { return syncFile(file) }

func CheckWatchDownload(path string) error { return checkWatchDownload(path) }

func ObserveWatchTree(s *Service, path string, id Identity, isDir, hold bool) (WatchSnapshot, func(), error) {
	return s.observeWatch(path, id, isDir, hold)
}

func SpareRestoredWatch(st *State, b Burial) { st.spareRestoredWatch(b, b.Path, time.Time{}) }

func GitVerificationBudget(elapsed time.Duration) time.Duration {
	return gitVerificationBudget(elapsed)
}

func SetBeforePurgeDelete(s *Service, fn func()) func() {
	previous := s.beforePurgeDelete
	s.beforePurgeDelete = fn
	return func() { s.beforePurgeDelete = previous }
}

func SameDirectory(s *Service, a, b string) (bool, error) { return s.sameDirectory(a, b) }

func WriteRestoreIntent(b Burial, parent RestoreParent, name string) error {
	return writeRestoreIntent(b, parent, name)
}

func PendingBurialRetry(s *Service, st State, path string) string {
	return s.pendingBurialRetry(st, path)
}

func SetReplacementSync(fn func(*os.File) error) func() {
	previous := syncReplacement
	syncReplacement = fn
	return func() { syncReplacement = previous }
}

// GraveyardRoot exposes graveyardRoot to the external tests.
func GraveyardRoot(s *Service, path string) string { return s.graveyardRoot(path) }

func GraveyardRoots(s *Service, st State) []string { return s.graveyardRoots(st) }

func CheckGraveyardOwner(s *Service, root string, create bool) error {
	return s.checkGraveyardOwner(root, create)
}

func ScanGraveyardRoot(s *Service, root string) ([]PendingGrave, error) {
	return s.scanGraveyardRoots(State{}, s.resolveMissingGraveyardRoots(nil, []string{root}))
}

func ScanManifests(s *Service, st State) ([]PendingGrave, error) { return s.scanManifests(st) }

// Return the operation's containment check with its initial root snapshot.
func GraveyardContainment(s *Service, root string) func(string) bool {
	roots := s.resolveMissingGraveyardRoots(nil, []string{root})
	return func(path string) bool { return s.insideGraveyard(roots, path) }
}

func PendingGraveSnapshot(s *Service, st State) ([]PendingGrave, error) { return s.pendingGraves(st) }

// SetBurialStep installs the interruption seam and returns its cleanup.
func SetBurialStep(s *Service, fn func(string) error) func() {
	previous := s.afterBurialStep
	s.afterBurialStep = fn
	return func() { s.afterBurialStep = previous }
}

func SetAfterManifest(s *Service, fn func()) func() {
	return SetBurialStep(s, func(step string) error {
		if step == "manifest" {
			fn()
		}
		return nil
	})
}
