//go:build linux || darwin

package winfs

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRestoreDirectorySearchBudgets(t *testing.T) {
	dir := t.TempDir()
	fs := FS{}
	for i := range 520 {
		if err := os.Mkdir(filepath.Join(dir, fmt.Sprint(i)), 0700); err != nil {
			t.Fatal(err)
		}
	}
	// Use actual enumeration order: POSIX directory iteration isn't sorted.
	handle, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	names, err := handle.Readdirnames(-1)
	handle.Close()
	if err != nil {
		t.Fatal(err)
	}
	want := posixID(t, filepath.Join(dir, names[len(names)-1]))
	parent, err := fs.OpenRestoreParent(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	if found := fs.FindRestoreDirectory(parent, want); found != "" {
		t.Fatalf("searched beyond the entry budget: %s", found)
	}
	first := posixID(t, filepath.Join(dir, names[0]))
	p := parent.(*restoreParent)
	search := directorySearch{fs: fs, want: first, left: 512, until: time.Now().Add(-time.Second)}
	if got := search.scan(p.file, dir, true); got != "" || search.left != 512 {
		t.Fatalf("expired search consumed entries: %s %d", got, search.left)
	}
}
