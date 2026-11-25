package winfs

import (
	"errors"
	"golang.org/x/sys/unix"
	"os"
	"testing"
)

func TestBurialLookupRejectsMountCrossing(t *testing.T) {
	parent, err := os.Open("/")
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	var root, proc unix.Stat_t
	if unix.Fstat(int(parent.Fd()), &root) != nil || unix.Stat("/proc", &proc) != nil || root.Dev == proc.Dev {
		t.Skip("no separate procfs mount")
	}
	_, err = (FS{}).burialStat(int(parent.Fd()), "proc")
	if !errors.Is(err, unix.EXDEV) {
		t.Fatalf("mount boundary: %v", err)
	}
}
