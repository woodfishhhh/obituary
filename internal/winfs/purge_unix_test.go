//go:build linux || darwin

package winfs

import (
	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
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
	if err := (FS{}).removeGraveEntry(int(dir.Fd()), "nested", id, -1, nil); err == nil {
		t.Fatal("purge traversed foreign device")
	}
	if got, err := os.ReadFile(file); err != nil || string(got) != "keep" {
		t.Fatal(string(got), err)
	}
}

func TestPurgeGraveUsesProtectedHandleIdentity(t *testing.T) {
	dir := t.TempDir()
	grave := filepath.Join(dir, "grave")
	makePOSIXPayload(t, grave, true, "keep")
	file := filepath.Join(grave, "child")
	id := posixID(t, file)
	if err := (FS{}).PurgeGrave(grave, posixID(t, grave), corefs.ProtectedIDs{id.FileID: true}); err == nil {
		t.Fatal("protected child accepted for removal")
	}
	if got, err := os.ReadFile(file); err != nil || string(got) != "keep" {
		t.Fatal("changed protected child", err)
	}
}
