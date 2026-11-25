//go:build linux || darwin

package winfs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBurialDirectoryAndQueuedChanges(t *testing.T) {
	backend := FS{}
	root := t.TempDir()
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0700); err != nil {
		t.Fatal(err)
	}
	id, err := backend.Identify(nested)
	if err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(nested, alias); err != nil {
		t.Fatal(err)
	}
	if dir, err := backend.OpenBurialDirectory(alias, id); err == nil {
		dir.Close()
		t.Fatal("followed directory symlink")
	}
	if dir, err := backend.OpenBurialDirectory(root, id); err == nil {
		dir.Close()
		t.Fatal("opened replaced directory")
	}
	dir, err := backend.OpenBurialDirectory(nested, id)
	if err != nil {
		t.Fatal(err)
	}
	dir.Close()
	changes, err := backend.NewDirectoryChanges(root)
	if err != nil {
		t.Fatal(err)
	}
	defer changes.Close()
	for _, name := range []string{root, nested} {
		if err := changes.WatchDirectory(name); err != nil {
			t.Fatal(err)
		}
	}
	if err := changes.Check(); err != nil {
		t.Fatalf("unchanged tree: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "arrived"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if changes.Check() == nil {
		t.Fatal("missed queued nested entry creation")
	}
}

func TestBurialChangesFollowRenamedDirectory(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "original")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	old, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	current := filepath.Join(parent, "renamed")
	if err := os.Rename(dir, current); err != nil {
		t.Fatal(err)
	}
	watch, err := (FS{}).NewDirectoryChanges(current)
	if err != nil {
		t.Fatal(err)
	}
	defer watch.Close()
	if err := watch.WatchDirectory(current); err != nil {
		t.Fatal(err)
	}
	if err := watch.Check(); err != nil {
		t.Fatal(err)
	}
	if err := old.WriteFile("note", nil, 0600); err != nil {
		t.Fatal(err)
	}
	if watch.Check() == nil {
		t.Fatal("watcher missed a write through a pre-rename directory handle")
	}
}
