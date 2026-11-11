package hook

import (
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestSystemDiscoveryRootsUseKnownFolders(t *testing.T) {
	roots := systemDiscoveryRoots()
	for _, id := range []*windows.KNOWNFOLDERID{
		windows.FOLDERID_ProgramFiles, windows.FOLDERID_ProgramFilesX86, windows.FOLDERID_Windows,
	} {
		path, err := windows.KnownFolderPath(id, windows.KF_FLAG_DONT_VERIFY)
		if err != nil {
			continue // Unavailable locations retain diagnostics.
		}
		if !inProtectedDiscoveryRoot(path, roots) || !inProtectedDiscoveryRoot(filepath.Join(path, "app"), roots) {
			t.Fatalf("known system folder missing from discovery roots: %q", path)
		}
	}
}

func TestDiscoveryProtectedWindowsPathSpellings(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Program Files")
	for _, path := range []string{root, strings.ToUpper(root), filepath.ToSlash(root), `\\?\` + root} {
		if !inProtectedDiscoveryRoot(path, []string{root}) {
			t.Errorf("equivalent spelling was not classified: %q", path)
		}
	}
	for _, path := range []string{`\\?\` + root + ".", root + "X", filepath.Join(root+"X", "WindowsApps")} {
		if inProtectedDiscoveryRoot(path, []string{root}) {
			t.Errorf("unrelated spelling suppressed diagnostics: %q", path)
		}
	}
}
