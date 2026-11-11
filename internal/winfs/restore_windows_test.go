package winfs

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/windows"
)

func TestRestoreRollbackWithOpenChild(t *testing.T) {
	t.Parallel()
	fs := FS{}
	root := t.TempDir()
	parent, err := fs.OpenRestoreParent(root)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	outer, err := fs.MkdirRestoreParent(parent, "outer")
	if err != nil {
		t.Fatal(err)
	}
	child, err := fs.MkdirRestoreParent(outer, "child")
	if err != nil {
		t.Fatal(err)
	}
	name, _ := windows.UTF16PtrFromString(child.Path())
	h, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES, shareAll, nil, windows.OPEN_EXISTING, openFlags, 0)
	if err != nil {
		child.Close()
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	if err := child.Rollback(); err != nil {
		child.Close()
		t.Fatal(err)
	}
	child.Close()
	rollbackErr := outer.Rollback()
	outer.Close()
	if rollbackErr != nil {
		t.Fatalf("rollback left ancestor while child handle remains open: %v", rollbackErr)
	}
	if _, err := os.Stat(filepath.Join(root, "outer")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("rollback left ancestor while child handle remains open: %v", err)
	}
}

func TestMkdirRestoreParentStaysPinned(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	fs := FS{}
	root := t.TempDir()
	path, outside := filepath.Join(root, "parent"), filepath.Join(root, "outside")
	for _, p := range []string{path, outside} {
		if err := os.Mkdir(p, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	parent, err := fs.OpenRestoreParent(path)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	if err := os.Rename(path, path+"-old"); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", path, outside).CombinedOutput(); err != nil {
		t.Fatalf("junction: %v %s", err, out)
	}
	child, err := fs.MkdirRestoreParent(parent, "child")
	if err != nil {
		t.Fatal(err)
	}
	defer child.Close()
	if _, err := os.Stat(filepath.Join(path+"-old", "child")); err != nil {
		t.Fatal("directory recreation escaped pinned ancestor", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "child")); !os.IsNotExist(err) {
		t.Fatal("directory recreation entered replacement junction")
	}
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(path+"-old", "link"), outside).CombinedOutput(); err != nil {
		t.Fatalf("junction: %v %s", err, out)
	}
	if child, err := fs.MkdirRestoreParent(parent, "link"); err == nil {
		child.Close()
		t.Fatal("directory recreation followed new reparse child")
	}
	for _, name := range []string{"..", `..\escape`, "x:y", `C:\escape`} {
		if child, err := fs.MkdirRestoreParent(parent, name); err == nil {
			child.Close()
			t.Fatal("invalid directory component accepted")
		}
	}
}

func TestOpenRestoreParentRequiresOrdinaryDirectory(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	root := t.TempDir()
	a := filepath.Join(root, "a")
	if err := os.Mkdir(a, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, a).CombinedOutput(); err != nil {
		t.Fatalf("junction: %v %s", err, out)
	}
	file := filepath.Join(root, "file")
	write(t, file)
	for _, path := range []string{link, file} {
		parent, err := (FS{}).OpenRestoreParent(path)
		if parent != nil {
			parent.Close()
		}
		if !errors.Is(err, corefs.ErrUnsafe) {
			t.Fatalf("unverified restore parent accepted: %s %v", path, err)
		}
	}
}

func TestMovePinnedIntoPreservesSourceAndDestination(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	parentPath := filepath.Join(root, "parent")
	if err := os.Mkdir(parentPath, 0o700); err != nil {
		t.Fatal(err)
	}
	parent, err := (FS{}).OpenRestoreParent(parentPath)
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	source, dest := filepath.Join(root, "source"), filepath.Join(parentPath, "dest")
	write(t, source)
	id, err := (FS{}).Identify(source)
	if err != nil {
		t.Fatal(err)
	}
	write(t, dest)
	if _, err := (FS{}).MovePinnedInto(source, id, parent, "dest"); !errors.Is(err, os.ErrExist) {
		t.Fatalf("collision was not explicit: %v", err)
	}
	if err := os.Rename(source, source+"-old"); err != nil {
		t.Fatal(err)
	}
	write(t, source)
	if _, err := (FS{}).MovePinnedInto(source, id, parent, "new"); err == nil {
		t.Fatal("source identity swap accepted")
	}
	for _, path := range []string{source, source + "-old", dest} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"../escape", `..\escape`, `C:\escape`, "stream:ads", ".."} {
		if _, err := (FS{}).MovePinnedInto(source+"-old", id, parent, name); !errors.Is(err, corefs.ErrUnsafe) {
			t.Fatalf("relative rename escaped parent: %s %v", name, err)
		}
	}
}

func TestFindRestoredRechecksParentAfterLocate(t *testing.T) {
	for _, mode := range []string{"renamed", "replaced"} {
		t.Run(mode, func(t *testing.T) {
			fs := FS{}
			root := t.TempDir()
			parent := filepath.Join(root, "parent")
			if err := os.Mkdir(parent, 0700); err != nil {
				t.Fatal(err)
			}
			source := filepath.Join(parent, "payload")
			write(t, source)
			pid, err := fs.Identify(parent)
			if err != nil {
				t.Fatal(err)
			}
			sid, err := fs.Identify(source)
			if err != nil {
				t.Fatal(err)
			}
			called := false
			_, err = fs.findRestored(pid, sid, func(id corefs.Identity) (string, error) {
				path, err := fs.Locate(id)
				if err != nil {
					return "", err
				}
				called = true
				if err := os.Rename(parent, parent+"-old"); err != nil {
					t.Fatal(err)
				}
				if mode == "replaced" {
					if err := os.Mkdir(parent, 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.Rename(filepath.Join(parent+"-old", "payload"), source); err != nil {
						t.Fatal(err)
					}
				}
				return path, nil
			})
			if !called || err == nil {
				t.Fatal("FindRestored accepted replaced parent identity")
			}
			if mode == "replaced" && !errors.Is(err, corefs.ErrUnsafe) {
				t.Fatalf("parent mismatch hidden by other defense: %v", err)
			}
		})
	}
}

func TestRestoreDirectoryRollbackPreservesExistingAndNonempty(t *testing.T) {
	fs := FS{}
	root := t.TempDir()
	p, err := fs.OpenRestoreParent(root)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	for _, mode := range []string{"existing", "nonempty", "created"} {
		t.Run(mode, func(t *testing.T) {
			path := filepath.Join(root, mode)
			if mode == "existing" {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			child, err := fs.MkdirRestoreParent(p, mode)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "nonempty" {
				write(t, filepath.Join(path, "keep"))
			}
			err = child.Rollback()
			child.Close()
			if mode == "nonempty" {
				if err == nil {
					t.Fatal("nonempty rollback must report retained directory")
				}
			} else if err != nil {
				t.Fatal(err)
			}
			_, err = os.Stat(path)
			if mode == "created" && !os.IsNotExist(err) {
				t.Fatal("rollback left own empty directory")
			}
			if mode != "created" && err != nil {
				t.Fatal("rollback deleted existing or nonempty directory")
			}
		})
	}
}
