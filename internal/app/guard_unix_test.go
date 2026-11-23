//go:build linux || darwin

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/winfs"
)

func TestDrvFSOwnershipAppliesOnlyToManagedPaths(t *testing.T) {
	path := os.Getenv("OBIT_TEST_DRVFS_PATH")
	if path == "" {
		t.Skip("requires an existing WSL Windows fixture")
	}
	t.Chdir(filepath.Dir(path))
	if _, _, err := (winfs.FS{}).LongPath("."); err != nil {
		t.Errorf("cannot resolve project on DrvFS: %v", err)
	}
	g := Guard(t.TempDir())
	if err := g.Check(path, true); err == nil || !strings.Contains(err.Error(), "Windows") {
		t.Errorf("missing Windows handoff: %v", err)
	}
	if err := g.Check(t.TempDir(), false); err != nil {
		t.Errorf("native target rejected from DrvFS project: %v", err)
	}
}

func TestPOSIXGuardProtectsSystemAndHome(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "user")
	if err := os.Mkdir(user, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OBIT_USER_HOME", user)
	g := Guard(filepath.Join(dir, "data"))
	for _, path := range []string{"/", "/etc", user} {
		if err := g.Check(path, true); err == nil {
			t.Fatalf("protected directory accepted: %s", path)
		}
	}
	path := filepath.Join(dir, "scratch")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := g.Check(path, false); err != nil {
		t.Fatal(err)
	}
}
