package explorer

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/woodfishhhh/obituary/internal/project"
)

// The real Git replacement regression is skipped in short mode. This uses
// real project discovery in an ordinary directory to cover the same pin.
func TestDirectoryDiscoveryKeepsParentPinned(t *testing.T) {
	svc := syncService(t)
	dir := filepath.Join(t.TempDir(), "target")
	storeItems(t, svc, trackedFile(t, dir, "report.log"))
	original, attempted := findProject, false
	var moveErr error
	findProject = func(ctx context.Context, path string) (project.Project, bool, error) {
		proj, found, err := original(ctx, path)
		attempted = true
		moveErr = os.Rename(dir, dir+".moved")
		return proj, found, err
	}
	t.Cleanup(func() { findProject = original })
	err := Sync(svc, Options{Enabled: true, Everywhere: true})
	if err != nil || !attempted || moveErr == nil {
		t.Fatalf("directory discovery was not pinned: sync=%v attempted=%v rename=%v", err, attempted, moveErr)
	}
	assertLabel(t, dir, "report.log", true)
}
