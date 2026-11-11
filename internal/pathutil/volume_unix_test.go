//go:build linux || darwin

package pathutil

import (
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestPOSIXMountRootUsesDeviceBoundary(t *testing.T) {
	dir := t.TempDir()
	root, err := MountRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	var source, mount unix.Stat_t
	if err := unix.Stat(dir, &source); err != nil {
		t.Fatal(err)
	}
	if err := unix.Stat(root, &mount); err != nil {
		t.Fatal(err)
	}
	if source.Dev != mount.Dev {
		t.Fatal("mount changed device")
	}
	if parent := filepath.Dir(root); parent != root {
		var up unix.Stat_t
		if err := unix.Stat(parent, &up); err != nil || up.Dev == mount.Dev {
			t.Fatal("not a device boundary", err)
		}
	}
	missing, err := MountRoot(filepath.Join(dir, "absent", "child"))
	if err != nil || missing != root {
		t.Fatal(missing, err)
	}
}
