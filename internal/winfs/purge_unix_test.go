//go:build linux || darwin

package winfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPurgeEntryRejectsForeignDevice(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "nested", "child")
	makePOSIXPayload(t, filepath.Dir(file), true, "keep")
	dir, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	id := posixID(t, root)
	id.Serial++
	if err := (FS{}).removeGraveEntry(int(dir.Fd()), "nested", id); err == nil {
		t.Fatal("purge traversed foreign device")
	}
	if got, err := os.ReadFile(file); err != nil || string(got) != "keep" {
		t.Fatal(string(got), err)
	}
}
