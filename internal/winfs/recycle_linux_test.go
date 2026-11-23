package winfs

import (
	"errors"
	"golang.org/x/sys/unix"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRecycleLinuxDesktopEntry(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	path := filepath.Join(dir, "two words % #.txt")
	for i := 0; i < 2; i++ {
		if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
			t.Fatal(err)
		}
		fs := FS{}
		id, err := fs.Identify(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := fs.Recycle(path, id); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("source: %v", err)
		}
	}
	root := filepath.Join(dir, "Trash")
	entries, err := os.ReadDir(filepath.Join(root, "info"))
	if err != nil || len(entries) != 2 {
		t.Fatalf("entries %v: %v", entries, err)
	}
	for _, e := range entries {
		info, err := os.ReadFile(filepath.Join(root, "info", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.TrimSpace(string(info)), "\n")
		if len(lines) != 3 || lines[0] != "[Trash Info]" || lines[1] != "Path="+(&url.URL{Path: path}).EscapedPath() {
			t.Fatalf("bad info: %q", info)
		}
		if _, err := time.ParseInLocation("2006-01-02T15:04:05", strings.TrimPrefix(lines[2], "DeletionDate="), time.Local); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(filepath.Join(root, "files", strings.TrimSuffix(e.Name(), ".trashinfo")))
		if err != nil || string(data) != "keep" {
			t.Fatal(string(data), err)
		}
	}
}

func TestRecycleLinuxPublishFailureRestoresSource(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	path := filepath.Join(dir, "document")
	if err := os.WriteFile(path, []byte("retained"), 0600); err != nil {
		t.Fatal(err)
	}
	fs := FS{}
	id, err := fs.Identify(path)
	if err != nil {
		t.Fatal(err)
	}
	moves := 0
	fs.rename = func(a int, n string, b int, m string) error {
		moves++
		if moves == 2 {
			return unix.EIO
		}
		return renameExclusive(a, n, b, m)
	}
	if err := fs.Recycle(path, id); !errors.Is(err, unix.EIO) {
		t.Fatal(err)
	}
	if got, err := os.ReadFile(path); err != nil || string(got) != "retained" {
		t.Fatal(string(got), err)
	}
}

func TestRecycleLinuxRejectsRedirectedTrash(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", dir)
	target := t.TempDir()
	if err := os.Symlink(target, filepath.Join(dir, "Trash")); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "source")
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	fs := FS{}
	id, err := fs.Identify(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := fs.Recycle(path, id); err == nil {
		t.Fatal("redirected trash accepted")
	}
	if b, err := os.ReadFile(path); err != nil || string(b) != "keep" {
		t.Fatal(string(b), err)
	}
	entries, err := os.ReadDir(target)
	if err != nil || len(entries) != 0 {
		t.Fatal(entries, err)
	}
}
