//go:build linux || darwin

package winfs

import (
	"crypto/rand"
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
	st, err := (FS{}).burialStat(int(dir.Fd()), "nested")
	if err != nil {
		t.Fatal(err)
	}
	tree := purgeTree{plan: corefs.PurgePlan{Object: id}}
	if err := tree.entry(int(dir.Fd()), "nested", st); err == nil {
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
	if err := (FS{}).PurgeGrave(grave, posixID(t, grave), corefs.PurgePlan{Name: "child", Object: id, Stage: ".obit-purge-" + rand.Text(), Guard: func(check func(corefs.ProtectedIDs) error) error { return check(corefs.ProtectedIDs{id.FileID: true}) }}); err == nil {
		t.Fatal("protected child accepted for removal")
	}
	if got, err := os.ReadFile(file); err != nil || string(got) != "keep" {
		t.Fatal("changed protected child", err)
	}
}
