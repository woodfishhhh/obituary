package winfs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/woodfishhhh/obituary/internal/testenv"
	"golang.org/x/sys/windows"
)

func TestBurialDescentRequiresKnownReparseTag(t *testing.T) {
	attrs := uint32(windows.FILE_ATTRIBUTE_DIRECTORY | windows.FILE_ATTRIBUTE_REPARSE_POINT)
	for _, tag := range []uint32{windows.IO_REPARSE_TAG_SYMLINK, windows.IO_REPARSE_TAG_MOUNT_POINT, 0x9000001a, 0xa00000ff} {
		descend, err := burialDescent(attrs, tag)
		known := tag == windows.IO_REPARSE_TAG_SYMLINK || tag == windows.IO_REPARSE_TAG_MOUNT_POINT
		if descend || (err == nil) != known {
			t.Fatalf("tag %#x: descend=%v err=%v", tag, descend, err)
		}
	}
}

func TestBurialWatcherRechecksRootACL(t *testing.T) {
	dir := t.TempDir()
	child := filepath.Join(dir, "child")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{dir, child} {
		t.Run(filepath.Base(target), func(t *testing.T) {
			w, err := (FS{}).NewDirectoryChanges(dir)
			if err != nil {
				t.Fatal(err)
			}
			defer w.Close()
			if err := w.Check(); err != nil {
				t.Fatal(err)
			}
			testenv.DenyAccess(t, target, windows.FILE_LIST_DIRECTORY)
			if w.Check() == nil {
				t.Fatal("revoked list access authorized cached inventory")
			}
		})
	}
}
