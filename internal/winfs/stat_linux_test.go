package winfs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/unix"
)

func Test9pDeviceOwnership(t *testing.T) {
	const mounts = "12 1 0:51 / /volume rw - 9p C: rw,aname=drvfs;path=C:\n13 1 0:52 / /mnt/c rw - 9p remote rw,aname=other"
	if err := check9pDevice(mounts, "0:51"); !errors.Is(err, corefs.ErrUnsupported) {
		t.Fatalf("DrvFS accepted: %v", err)
	}
	if err := check9pDevice(mounts, "0:52"); err != nil {
		t.Fatalf("other 9p share rejected: %v", err)
	}
	if err := check9pDevice(mounts, "0:53"); err == nil {
		t.Fatal("unidentified 9p device accepted")
	}
}

func TestPOSIXIdentityRefusesInodeOnlyFID(t *testing.T) {
	// Linux UAPI AT_HANDLE_FID (0x200) is not exposed by x/sys/unix yet.
	const handleFID = 0x200
	lookups := 0
	lookup := func(fd int, name string, flags int) (unix.FileHandle, int, error) {
		lookups++
		if flags&handleFID != 0 {
			// Observed on user-namespace overlayfs with nfs_export disabled:
			// the same successful FID is returned after deletion and recreation.
			return unix.NewFileHandle(129, []byte{0xe7, 0xec, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0}), 1, nil
		}
		return unix.FileHandle{}, 0, unix.EOPNOTSUPP
	}
	id, err := linuxCreationIDWith(42, "", 1, lookup)
	if err != nil || id != 0 || lookups != 1 {
		t.Fatalf("inode-only FID authorized a durable identity: %x calls=%d err=%v", id, lookups, err)
	}
}

func TestPOSIXGenerationSeparatesEqualBirthTimes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload")
	seen := map[uint64]bool{}
	for i := 0; i < 128; i++ {
		makePOSIXPayload(t, path, false, "new")
		fd, err := unix.Open(path, moveOpenFlags, 0)
		if err != nil {
			t.Fatal(err)
		}
		// Filesystems may report exactly the same creation timestamp after
		// inode reuse. Exercise real export handles with that timestamp fixed.
		generation, err := linuxCreationID(fd, "", 1)
		unix.Close(fd)
		if err != nil || generation == 0 || seen[generation] {
			t.Fatalf("generation did not separate equal birth times: %x %v", generation, err)
		}
		seen[generation] = true
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
}
