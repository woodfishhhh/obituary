package winfs

import (
	"errors"
	"os"
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
	if err := checkOpenFiles(st.Dev, st.Ino); !errors.Is(err, errWatchFileOpen) {
		t.Fatalf("open: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := checkOpenFiles(st.Dev, st.Ino); err != nil {
		t.Fatalf("closed: %v", err)
	}
}
