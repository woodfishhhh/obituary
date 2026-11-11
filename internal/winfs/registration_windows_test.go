package winfs

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestFileAncestryRequiresStableVolume(t *testing.T) {
	for _, format := range []string{"NTFS", "ReFS", "FAT", "FAT32", "exFAT", "unknown", ""} {
		for _, flags := range []uint32{0, windows.FILE_SUPPORTS_OPEN_BY_FILE_ID} {
			want := flags != 0 && (format == "NTFS" || format == "ReFS")
			if got := stableRegistrationFormat(format, flags); got != want {
				t.Fatalf("unstable volume accepted: %q flags=%x got=%v want=%v", format, flags, got, want)
			}
		}
	}
}

func TestFileAncestryRejectsDirectories(t *testing.T) {
	path := t.TempDir()
	fs := FS{}
	id, err := fs.Identify(path)
	if err != nil {
		t.Fatal(err)
	}
	a, err := fs.OpenFileAncestry(path, id)
	if err == nil {
		a.Close()
		t.Fatal("directory obtained regular-file proof")
	}
}

func TestFileAncestryPinsOrdinaryFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.log")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	fs := FS{}
	id, err := fs.Identify(path)
	if err != nil {
		t.Fatal(err)
	}
	a, err := fs.OpenFileAncestry(path, id)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	pinned := a.(*fileAncestry)
	pinned.path = path + ".other"
	if pinned.confirms(id) {
		t.Fatal("ancestry accepted a different current name")
	}
	pinned.path = path
	if err := os.Rename(path, path+".moved"); err == nil {
		t.Fatal("file ancestry did not pin the candidate")
	}
	if err := os.Rename(dir, dir+".moved"); err == nil {
		t.Fatal("file ancestry did not pin the parent")
	}
	a.Close()
	if err := os.Rename(path, path+".moved"); err != nil {
		t.Fatalf("ancestry leaked a handle: %v", err)
	}
}
