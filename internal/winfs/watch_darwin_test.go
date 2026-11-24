package winfs

import (
	"errors"
	"os"
	"syscall"
	"testing"
)

func TestDarwinOpenWatchDescriptor(t *testing.T) {
	file, err := os.CreateTemp(t.TempDir(), "open")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}
	st := info.Sys().(*syscall.Stat_t)
	err = inspectDarwinFDs(os.Getpid(), uint64(st.Dev), st.Ino)
	if !errors.Is(err, errWatchFileOpen) {
		t.Fatalf("own open descriptor: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := inspectDarwinFDs(os.Getpid(), uint64(st.Dev), st.Ino); err != nil {
		t.Fatalf("closed descriptor: %v", err)
	}
}
