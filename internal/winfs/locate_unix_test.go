//go:build linux || darwin

package winfs

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestPOSIXBirthTimeCheckedAtLookup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload")
	makePOSIXPayload(t, path, false, "keep")
	fs := FS{}
	want, parent := posixID(t, path), posixID(t, dir)
	root, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	got, isDir, err := fs.BurialEntry(root, path, parent)
	if err != nil || isDir || got != want || got == changedBirth(want) {
		t.Fatalf("burial inventory lost birth time: %+v %v", got, err)
	}
	if found, err := fs.FindRestoredIn(dir, parent, changedBirth(want)); err == nil {
		t.Fatalf("restore lookup accepted wrong source birth time: %s", found)
	}
	if found, err := fs.FindRestoredIn(dir, changedBirth(parent), want); err == nil {
		t.Fatalf("restore lookup accepted wrong parent birth time: %s", found)
	}
	if found, err := fs.FindRestoredIn(dir, parent, want); err != nil || found != path {
		t.Fatalf("durable restore lookup failed: %s %v", found, err)
	}
	fs.creationID = func(uint64) uint64 { return 0 }
	weak, err := fs.Identify(path)
	if err != nil {
		t.Fatal(err)
	}
	got, _, err = fs.BurialEntry(root, path, parent)
	if err != nil || got.RequirePersistent() == nil || got != weak || got == want {
		t.Fatalf("weak child acquired a durable identity: %+v %v", got, err)
	}
	parentBirth, err := strconv.ParseUint(parent.FileID[:16], 16, 64)
	if err != nil {
		t.Fatal(err)
	}
	fs.creationID = func(value uint64) uint64 {
		if value == parentBirth {
			return 0
		}
		return value
	}
	weakParent, err := fs.Identify(dir)
	if err != nil {
		t.Fatal(err)
	}
	if found, err := fs.FindRestoredIn(dir, weakParent, want); err == nil {
		t.Fatalf("weak restore parent accepted: %s", found)
	}
}
