package winfs

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestLinuxWatchFDVisibility(t *testing.T) {
	const mount = "1 0 0:1 / /proc rw - proc proc rw"
	if !visibleProcessTable(mount) || visibleProcessTable(mount+",hidepid=2") || visibleProcessTable("") {
		t.Fatal("hidden process table accepted")
	}
	err := eachProcNumber("/proc/self/fd", func(string) error { return os.ErrPermission })
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("unreadable fd: %v", err)
	}
}

func TestLinuxWatchFileOpenAndClosed(t *testing.T) {
	if os.Getenv("OBIT_TEST_PRIVATE_PROC") != "1" {
		t.Skip("requires an isolated, fully readable procfs")
	}
	file, err := os.CreateTemp(t.TempDir(), "watch")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	st := info.Sys().(*syscall.Stat_t)
	if err := checkOpenFiles(st.Dev, st.Ino, st.Uid, true); !errors.Is(err, errWatchFileOpen) {
		t.Fatalf("open: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := checkOpenFiles(st.Dev, st.Ino, st.Uid, true); err != nil {
		t.Fatalf("closed: %v", err)
	}
}

func TestLinuxWatchUnreadableProcessOwner(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		name, ids       string
		private, retain bool
	}{
		{"owner-denied", "1000 1000 1000 1000", true, true}, {"saved-owner", "0 0 1000 0", true, true}, {"other-private", "1001 1001 1001 1001", true, false}, {"root-private", "0 0 0 0", true, false}, {"other-writable", "1001 1001 1001 1001", false, true}, {"unknown-owner", "?", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(filepath.Join(dir, "status"), []byte("Uid: "+tc.ids+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			err := linuxWatchGap(dir, 1000, tc.private, os.ErrPermission)
			if (err != nil) != tc.retain {
				t.Errorf("retain=%v, want %v: %v", err != nil, tc.retain, err)
			}
		})
	}
}
