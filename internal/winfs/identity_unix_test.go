//go:build linux || darwin

package winfs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
)

func TestPOSIXIdentityAndLinks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	fs := FS{}
	want, err := fs.Identify(path)
	if err != nil || len(want.FileID) != 32 {
		t.Fatal(want, err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	linkID, err := fs.Identify(link)
	if err != nil || linkID == want {
		t.Fatalf("symlink followed: %+v %v", linkID, err)
	}
	followed, err := fs.IdentifyFollowing(link)
	if err != nil || followed != want {
		t.Fatal(followed, err)
	}
	renamed := filepath.Join(dir, "renamed")
	if err := os.Rename(path, renamed); err != nil {
		t.Fatal(err)
	}
	got, err := fs.Identify(renamed)
	if err != nil || got != want {
		t.Fatalf("rename changed identity: %+v %v", got, err)
	}
	if _, err := fs.Locate(want); !errors.Is(err, corefs.ErrUnsupported) || errors.Is(err, corefs.ErrNotFound) {
		t.Fatalf("unknown inode was declared missing: %v", err)
	}
	if err := os.Link(renamed, path); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Identify(path); !errors.Is(err, corefs.ErrUnsafe) {
		t.Fatalf("hard link accepted: %v", err)
	}
	if _, err := fs.Identify(filepath.Join(dir, "absent")); !errors.Is(err, corefs.ErrNotFound) {
		t.Fatal(err)
	}
}

func TestPOSIXCanonicalPreservesFinalLinkAndCase(t *testing.T) {
	dir := t.TempDir()
	actual := filepath.Join(dir, "Actual")
	if err := os.Mkdir(actual, 0o700); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(dir, "alias")
	if err := os.Symlink(actual, alias); err != nil {
		t.Fatal(err)
	}
	leaf := filepath.Join(actual, "link")
	if err := os.Symlink("missing", leaf); err != nil {
		t.Fatal(err)
	}
	got, err := Canonical(filepath.Join(alias, "link"))
	if err != nil || got != leaf {
		t.Fatal(got, err)
	}
	if same, err := (FS{}).SameName("A", "a"); err != nil || same {
		t.Fatal(same, err)
	}
	id := corefs.Identity{Volume: "/CaseSensitive", Serial: 1, FileID: "1"}
	if id.Normalized() != id {
		t.Fatal("mount name casing changed")
	}
}
