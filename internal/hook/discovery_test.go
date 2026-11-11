package hook

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiscoveryProtectedPathBoundaries(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "Program Files")
	for _, tc := range []struct {
		name, path string
		want       bool
	}{
		{"root", root, true},
		{"child", filepath.Join(root, "WindowsApps", "app"), true},
		{"case", strings.ToUpper(root), true},
		{"slashes", filepath.ToSlash(root) + "/WindowsApps/", true},
		{"clean", root + string(filepath.Separator) + "child" + string(filepath.Separator) + "..", true},
		{"prefix", root + "X", false},
		{"prefix-child", filepath.Join(root+"X", "app"), false},
		{"outside", filepath.Join(base, "project"), false},
		{"ancestor", base, false},
		{"relative", "Program Files", false},
		{"empty", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := inProtectedDiscoveryRoot(tc.path, []string{root}); got != tc.want {
				t.Fatalf("protected(%q)=%v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestDiscoveryDeniedUsesFailedPath(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Program Files")
	setProtectedDiscoveryRoots(t, root)
	for _, tc := range []struct {
		name string
		err  error
		want bool
	}{
		{"wrapped", fmt.Errorf("discovery: %w", &os.PathError{Op: "resolve project directory", Path: root, Err: os.ErrPermission}), true},
		{"outside", &os.PathError{Op: "resolve project directory", Path: root + "X", Err: os.ErrPermission}, false},
		{"missing-path", os.ErrPermission, false},
		{"other-error", &os.PathError{Op: "resolve project directory", Path: root, Err: os.ErrNotExist}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := skipDiscoveryDenied(tc.err); got != tc.want {
				t.Fatalf("skip=%v, want %v: %v", got, tc.want, tc.err)
			}
		})
	}
	setProtectedDiscoveryRoots(t)
	if skipDiscoveryDenied(&os.PathError{Path: root, Err: os.ErrPermission}) {
		t.Fatal("an empty protected-root list suppressed a denial")
	}
}
