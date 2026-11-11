package integrate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestCommitBlocksConcurrentEdits(t *testing.T) {
	for _, action := range []string{"update", "remove"} {
		t.Run(action, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "managed")
			writeIntegrationFile(t, path, "authorized original")
			writeIntegrationFile(t, filepath.Join(root, "editor.tmp"), "NEW USER POLICY")
			c, err := readIntegrationChange(Scope{Project: root}, path)
			if err != nil {
				t.Fatal(err)
			}
			c.Action, c.data, c.delete = action, []byte("installer content"), action == "remove"
			attempted := false
			beforeIntegrationCommit = func(string) error {
				attempted = true
				return tryIntegrationEditorSave(root, path)
			}
			t.Cleanup(func() { beforeIntegrationCommit = nil })
			if err := Apply([]Change{c}); err != nil || !attempted {
				t.Fatalf("verified commit did not finish: %v (attempted=%v)", err, attempted)
			}
			if read(t, path+".bak") != "authorized original" || read(t, filepath.Join(root, "editor.tmp")) != "NEW USER POLICY" {
				t.Fatal("commit did not retain the verified backup and pending editor save")
			}
			if action == "remove" {
				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("verified remove left target: %v", err)
				}
			} else if read(t, path) != "installer content" {
				t.Fatal("verified update did not publish content")
			}
		})
	}
}

func tryIntegrationEditorSave(root, path string) error {
	if _, err := os.Stat(path + ".bak"); err == nil {
		return errors.New("backup preceded locked verification")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	name, err := integrationWindowsName(path)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(name, windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err == nil {
		windows.CloseHandle(h)
		return errors.New("verification permitted in-place editing")
	}
	dir, err := os.OpenRoot(root)
	if err != nil {
		return err
	}
	defer dir.Close()
	if err := dir.Rename("editor.tmp", filepath.Base(path)); err == nil {
		return errors.New("verification permitted atomic editor replacement")
	}
	return nil
}

func TestCommitConflictPreservesEditorVersion(t *testing.T) {
	for _, action := range []string{"update", "remove"} {
		t.Run(action, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "managed")
			writeIntegrationFile(t, path, "authorized original")
			c, err := readIntegrationChange(Scope{Project: root}, path)
			if err != nil {
				t.Fatal(err)
			}
			c.Action, c.data, c.delete = action, []byte("installer content"), action == "remove"
			beforeIntegrationWrite = func(string) error {
				tmp := filepath.Join(root, "editor.tmp")
				writeIntegrationFile(t, tmp, "NEW USER POLICY")
				return os.Rename(tmp, path)
			}
			t.Cleanup(func() { beforeIntegrationWrite = nil })
			if err := Apply([]Change{c}); err == nil || read(t, path) != "NEW USER POLICY" {
				t.Fatalf("commit conflict lost editor version: %v", err)
			}
			if _, err := os.Stat(path + ".bak"); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("conflicting commit created backup: %v", err)
			}
		})
	}
}

func TestUpdateKeepsTargetPresent(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "managed")
	writeIntegrationFile(t, path, "authorized original")
	stop, ready := make(chan struct{}), make(chan struct{})
	result := make(chan error, 1)
	go observeIntegrationName(path, stop, ready, result)
	<-ready
	t.Cleanup(func() {
		close(stop)
		if err := <-result; err != nil {
			t.Error(err)
		}
	})
	for i := range 20 {
		c, err := readIntegrationChange(Scope{Project: root}, path)
		if err != nil {
			t.Fatal(err)
		}
		c.Action, c.data = "update", fmt.Appendf(nil, "installer version %d", i)
		if err := Apply([]Change{c}); err != nil {
			t.Fatal(err)
		}
	}
	if read(t, path) != "installer version 19" {
		t.Fatal("update did not publish final version")
	}
}

func observeIntegrationName(path string, stop <-chan struct{}, ready chan<- struct{}, result chan<- error) {
	_, firstErr := os.Stat(path)
	close(ready)
	for {
		select {
		case <-stop:
			result <- firstErr
			return
		default:
			if _, err := os.Stat(path); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("update reader lost target name: %w", err)
			}
		}
	}
}

func TestCreatePreservesConcurrentFile(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "managed")
	c, err := readIntegrationChange(Scope{Project: root}, path)
	if err != nil {
		t.Fatal(err)
	}
	c.Action, c.data = "create", []byte("installer content")
	beforeIntegrationCommit = func(string) error {
		writeIntegrationFile(t, path, "NEW USER POLICY")
		return nil
	}
	t.Cleanup(func() { beforeIntegrationCommit = nil })
	err = Apply([]Change{c})
	if err == nil || read(t, path) != "NEW USER POLICY" {
		t.Fatalf("create overwrote concurrent file: %v", err)
	}
	assertIntegrationTempRemoved(t, root)
}

func TestCommitProtectsTemporarySource(t *testing.T) {
	for _, variant := range []string{"update-in-place", "update-file", "update-directory", "create-in-place"} {
		t.Run(variant, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "managed")
			if variant != "create-in-place" {
				writeIntegrationFile(t, path, "authorized original")
			}
			c, err := readIntegrationChange(Scope{Project: root}, path)
			if err != nil {
				t.Fatal(err)
			}
			c.Action, c.data = "update", []byte("approved installer bytes")
			if !c.existed {
				c.Action = "create"
			}
			protectIntegrationTemp(t, root, variant)
			if err := Apply([]Change{c}); err != nil {
				t.Fatalf("protected temporary source did not publish: %v", err)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != string(c.data) {
				t.Fatalf("temporary source protection failed: published %q: %v", data, err)
			}
			if c.existed && read(t, path+".bak") != "authorized original" {
				t.Fatal("temporary source attack changed the verified backup")
			}
			assertIntegrationTempRemoved(t, root)
		})
	}
}

func protectIntegrationTemp(t *testing.T, root, variant string) {
	t.Helper()
	attempted := false
	beforeIntegrationCommit = func(string) error {
		attempted = true
		matches, err := filepath.Glob(filepath.Join(root, ".obit-*.tmp"))
		if err != nil || len(matches) != 1 {
			t.Fatalf("temporary source fixture = %v: %v", matches, err)
		}
		if err := attackIntegrationTemp(matches[0], variant); !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			t.Errorf("temporary source protection failed: %s attack = %v", variant, err)
		}
		return nil // Finish publishing so the test also observes the installed bytes.
	}
	t.Cleanup(func() {
		beforeIntegrationCommit = nil
		if !attempted {
			t.Error("temporary source attack was not attempted")
		}
	})
}

func attackIntegrationTemp(path, variant string) error {
	if variant == "update-file" || variant == "update-directory" {
		if err := os.Rename(path, path+".parked"); err != nil {
			return err
		}
		if variant == "update-directory" {
			if err := os.Mkdir(path, 0o700); err != nil {
				return err
			}
			path = filepath.Join(path, "user-payload")
		}
		return os.WriteFile(path, []byte("unrelated user content"), 0o600)
	}
	name, err := integrationWindowsName(path)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(name, windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.TRUNCATE_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(h), path)
	_, err = f.Write([]byte("unverified in-place bytes"))
	return errors.Join(err, f.Close())
}

func assertIntegrationTempRemoved(t *testing.T, root string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(root, ".obit-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("commit left temporary sources: %v: %v", matches, err)
	}
}

func TestCreateIntegrationTemporaryUsesPinnedParent(t *testing.T) {
	base := t.TempDir()
	parent, moved := filepath.Join(base, "parent"), filepath.Join(base, "moved")
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	outer, err := os.OpenRoot(base)
	if err != nil {
		t.Fatal(err)
	}
	defer outer.Close()
	root, err := outer.OpenRoot("parent") // share-delete permits the test's swap
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := os.Rename(parent, moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(parent, 0o700); err != nil {
		t.Fatal(err)
	}
	file, err := createIntegrationTemporary(root, ".obit-probe.tmp")
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.WriteString("approved content")
	if err := errors.Join(writeErr, file.Close()); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(parent)
	if err != nil || len(entries) != 0 {
		t.Fatalf("temporary file escaped pinned parent: %v: %v", entries, err)
	}
	if read(t, filepath.Join(moved, ".obit-probe.tmp")) != "approved content" {
		t.Fatal("temporary creation missed pinned parent")
	}
}

func TestCommitTemporaryStaysInPinnedParent(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a junction subprocess")
	}
	for _, reject := range []bool{false, true} {
		t.Run(fmt.Sprint(reject), func(t *testing.T) {
			base, outside := t.TempDir(), t.TempDir()
			parent, moved := filepath.Join(base, "parent"), filepath.Join(base, "moved")
			if err := os.Mkdir(parent, 0o700); err != nil {
				t.Fatal(err)
			}
			writeIntegrationFile(t, filepath.Join(outside, "sentinel"), "unrelated content")
			c, err := readIntegrationChange(Scope{Project: base}, filepath.Join(parent, "managed"))
			if err != nil {
				t.Fatal(err)
			}
			c.Action, c.data = "create", []byte("approved content")
			beforeIntegrationTemporary = func(p *integrationTarget) error {
				// Retain the pinned object, but release only the name lock so the
				// fixture can test creation independently of that other safeguard.
				shared, err := p.root.OpenRoot(".")
				if err != nil {
					return err
				}
				p.root.Close()
				p.root = shared
				if err := os.Rename(parent, moved); err != nil {
					return err
				}
				makeIntegrationJunction(t, parent, outside)
				return nil
			}
			observed := false
			rejected := errors.New("reject after temporary creation")
			beforeIntegrationCommit = func(string) error {
				observed = true
				entries, err := os.ReadDir(outside)
				if err != nil || len(entries) != 1 || entries[0].Name() != "sentinel" {
					t.Errorf("temporary file escaped pinned parent: %v: %v", entries, err)
				}
				if reject {
					return rejected
				}
				return nil
			}
			t.Cleanup(func() {
				beforeIntegrationTemporary, beforeIntegrationCommit = nil, nil
				_ = os.Remove(parent) // This fixture's junction, never its target.
			})
			err = Apply([]Change{c})
			if !observed || reject && !errors.Is(err, rejected) || !reject && err != nil {
				t.Fatalf("pinned-parent commit = %v, observed=%v", err, observed)
			}
			assertIntegrationTempRemoved(t, outside)
			assertIntegrationTempRemoved(t, moved)
			if read(t, filepath.Join(outside, "sentinel")) != "unrelated content" {
				t.Fatal("junction target content changed")
			}
			if !reject && read(t, filepath.Join(moved, "managed")) != "approved content" {
				t.Fatal("installation missed pinned parent")
			}
			if _, err := os.Stat(filepath.Join(moved, "managed")); reject && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("failed installation left a target: %v", err)
			}
			if _, err := os.Stat(filepath.Join(outside, "managed")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("installation reached junction target: %v", err)
			}
		})
	}
}

func TestCommitCleansTemporarySourceOnFailure(t *testing.T) {
	for _, exists := range []bool{false, true} {
		t.Run(fmt.Sprint(exists), func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "managed")
			if exists {
				writeIntegrationFile(t, path, "authorized original")
			}
			c, err := readIntegrationChange(Scope{Project: root}, path)
			if err != nil {
				t.Fatal(err)
			}
			c.Action, c.data = "update", []byte("installer content")
			rejected := errors.New("commit rejected")
			beforeIntegrationCommit = func(string) error { return rejected }
			t.Cleanup(func() { beforeIntegrationCommit = nil })
			if err := Apply([]Change{c}); !errors.Is(err, rejected) {
				t.Fatalf("commit rejection = %v", err)
			}
			assertIntegrationTempRemoved(t, root)
			if exists && read(t, path) != "authorized original" {
				t.Fatal("failed commit changed the original")
			}
			if _, err := os.Stat(path); !exists && !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("failed commit created a target: %v", err)
			}
			if err := os.Rename(root, root+"-released"); err != nil {
				t.Fatalf("failed commit retained a handle: %v", err)
			}
			if err := os.Rename(root+"-released", root); err != nil {
				t.Fatal(err)
			}
		})
	}
}
