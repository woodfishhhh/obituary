package explorer

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func TestLabelsCommitBlocksEditorSave(t *testing.T) {
	for _, action := range []string{"update", "remove"} {
		t.Run(action, func(t *testing.T) {
			dir, dates := fixture(t)
			applyDates(t, dir, dates)
			if action == "remove" {
				dates = nil
			} else {
				dates["report.log"] = dates["report.log"].Add(time.Hour * 24)
			}
			p, err := prepareLabels(t, dir, dates)
			if err != nil {
				t.Fatal(err)
			}
			pending := filepath.Join(dir, "editor.tmp")
			if err := os.WriteFile(pending, []byte("[User]\nImportant=preserve me\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			attempted := false
			beforeLabelsCommit = func() error {
				attempted = true
				return tryLabelEditorSave(dir)
			}
			t.Cleanup(func() { beforeLabelsCommit = nil })
			if err := p.Apply(); err != nil || !attempted {
				t.Fatalf("verified label commit failed: %v (attempted=%v)", err, attempted)
			}
			if data, err := os.ReadFile(pending); err != nil || string(data) != "[User]\nImportant=preserve me\n" {
				t.Fatalf("pending editor version lost: %q %v", data, err)
			}
		})
	}
}

func TestLabelsFinalStreamsPreserveConcurrentEdit(t *testing.T) {
	for _, change := range []string{"new", "changed", "removed", "delete"} {
		t.Run(change, func(t *testing.T) {
			dir, dates := fixture(t)
			applyDates(t, dir, dates)
			stream := filepath.Join(dir, iniName) + ":user-note"
			if change == "changed" || change == "removed" {
				if err := writeSharedStream(stream, "old data"); err != nil {
					t.Fatal(err)
				}
			}
			if change == "delete" {
				dates = nil
			} else {
				dates["report.log"] = dates["report.log"].Add(24 * time.Hour)
			}
			p, err := prepareLabels(t, dir, dates)
			if err != nil {
				t.Fatal(err)
			}
			edit := func() error {
				if change == "removed" {
					return os.Remove(stream) // This test's stream, not user content.
				}
				return writeSharedStream(stream, "new data") // Same size exercises the hash.
			}
			beforeLabelsPublish = func(string) error { return edit() }
			beforeLabelsDelete = edit
			t.Cleanup(func() { beforeLabelsPublish, beforeLabelsDelete = nil, nil })
			if err := p.Apply(); err == nil || !bytes.Equal(readINI(t, dir), p.before.data) {
				t.Fatalf("concurrent stream edit lost: %v", err)
			}
			got, err := os.ReadFile(stream)
			if change == "removed" {
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("removed stream resurrected: %q %v", got, err)
				}
			} else if err != nil || string(got) != "new data" {
				t.Fatalf("concurrent stream edit lost: %q %v", got, err)
			}
			beforeLabelsPublish, beforeLabelsDelete = nil, nil
			applyDates(t, dir, dates)
			assertNoLabelTemporary(t, dir)
		})
	}
}

func writeSharedStream(path, data string) error {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(name, windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.CREATE_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(h), path)
	_, err = f.WriteString(data)
	return errors.Join(err, f.Close())
}

func TestLabelsFinalSecurityPreservesConcurrentDACL(t *testing.T) {
	dir, dates := fixture(t)
	applyDates(t, dir, dates)
	dates["report.log"] = dates["report.log"].Add(24 * time.Hour)
	p, err := prepareLabels(t, dir, dates)
	if err != nil {
		t.Fatal(err)
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sd, err := windows.SecurityDescriptorFromString("D:P(A;;FA;;;SY)(A;;FA;;;" + user.User.Sid.String() + ")")
	if err != nil {
		t.Fatal(err)
	}
	acl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	path, changed := filepath.Join(dir, iniName), ""
	beforeLabelsPublish = func(string) error {
		if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
			return err
		}
		got, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err == nil {
			changed = got.String()
		}
		// SetSecurityInfo may set Archive. Keep attributes at the planned
		// value so this regression proves the security comparison itself.
		return errors.Join(err, setAttributes(path, p.before.attrs))
	}
	t.Cleanup(func() { beforeLabelsPublish = nil })
	if err := p.Apply(); err == nil || changed == "" || !bytes.Equal(readINI(t, dir), p.before.data) {
		t.Fatalf("concurrent security edit lost: %v", err)
	}
	got, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil || got.String() != changed {
		t.Fatalf("concurrent security edit lost: %v %v", got, err)
	}
	beforeLabelsPublish = nil
	applyDates(t, dir, dates)
}

func tryLabelEditorSave(dir string) error {
	path, err := windows.UTF16PtrFromString(filepath.Join(dir, iniName))
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(path, windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err == nil {
		windows.CloseHandle(h)
		return errors.New("label verification permitted in-place editing")
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer root.Close()
	if err := root.Rename("editor.tmp", iniName); err == nil {
		return errors.New("label verification permitted atomic editor replacement")
	}
	return nil
}

func TestLabelsCreatePreservesConcurrentFile(t *testing.T) {
	dir, dates := fixture(t)
	p, err := prepareLabels(t, dir, dates)
	if err != nil {
		t.Fatal(err)
	}
	const user = "[User]\nImportant=preserve me\n"
	beforeLabelsCommit = func() error { return os.WriteFile(filepath.Join(dir, iniName), []byte(user), 0o600) }
	t.Cleanup(func() { beforeLabelsCommit = nil })
	if err := p.Apply(); err == nil || string(readINI(t, dir)) != user {
		t.Fatalf("label creation overwrote concurrent file: %v", err)
	}
}

func TestLabelsCommitRetriesWhileWriterOpen(t *testing.T) {
	dir, dates := fixture(t)
	path := filepath.Join(dir, iniName)
	const user = "[User]\nImportant=preserve me\n"
	if err := os.WriteFile(path, []byte(user), 0o600); err != nil {
		t.Fatal(err)
	}
	p, err := prepareLabels(t, dir, dates)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := p.Apply(); err == nil || string(readINI(t, dir)) != user {
		t.Fatalf("busy writer lost its file: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := p.Apply(); err != nil {
		t.Fatalf("retry after writer closed: %v", err)
	}
}

func TestLabelsFinalVerificationPreservesEdits(t *testing.T) {
	for _, kind := range []string{"content", "identity"} {
		for _, remove := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%v", kind, remove), func(t *testing.T) {
				dir, dates := fixture(t)
				applyDates(t, dir, dates)
				if remove {
					dates = nil
				} else {
					dates["report.log"] = dates["report.log"].Add(time.Hour * 24)
				}
				p, err := prepareLabels(t, dir, dates)
				if err != nil {
					t.Fatal(err)
				}
				user := p.before.data
				if kind == "content" {
					user = []byte("[User]\nImportant=preserve me\n")
				}
				beforeLabelsWrite = func() error {
					return saveBeforeLabelCommit(dir, user, p.before.attrs, kind == "identity")
				}
				t.Cleanup(func() { beforeLabelsWrite = nil })
				if err := p.Apply(); err == nil || !bytes.Equal(readINI(t, dir), user) {
					t.Fatalf("final label verification lost user edit: %s remove=%v err=%v", kind, remove, err)
				}
			})
		}
	}
}

func saveBeforeLabelCommit(dir string, data []byte, attrs uint32, replace bool) error {
	path := filepath.Join(dir, iniName)
	if replace {
		tmp := filepath.Join(dir, "editor.tmp")
		if err := os.WriteFile(tmp, data, 0o600); err != nil {
			return err
		}
		if err := os.Rename(tmp, path); err != nil {
			return err
		}
	} else if err := os.WriteFile(path, data, 0o600); err != nil {
		return err
	}
	return setAttributes(path, attrs)
}

func TestLabelsProtectTemporarySource(t *testing.T) {
	for _, action := range []string{"create", "update"} {
		for _, attack := range []string{"write", "rename"} {
			t.Run(action+"/"+attack, func(t *testing.T) {
				dir, dates := fixture(t)
				if action == "update" {
					if err := os.WriteFile(filepath.Join(dir, iniName), []byte("; user"), 0o600); err != nil {
						t.Fatal(err)
					}
				}
				p, err := prepareLabels(t, dir, dates)
				if err != nil {
					t.Fatal(err)
				}
				attempted := false
				beforeLabelsPublish = func(path string) error {
					attempted = true
					if err := attackLabelTemporary(path, attack); !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
						t.Errorf("label temporary source was not protected: %s: %v", attack, err)
					}
					return nil
				}
				t.Cleanup(func() { beforeLabelsPublish = nil })
				if err := p.Apply(); err != nil || !attempted || !bytes.Equal(readINI(t, dir), p.after) {
					t.Fatalf("label temporary source published wrong content: %v", err)
				}
				assertNoLabelTemporary(t, dir)
			})
		}
	}
}

func attackLabelTemporary(path, attack string) error {
	if attack == "rename" {
		if err := os.Rename(path, path+".parked"); err != nil {
			return err
		}
		return os.WriteFile(path, []byte("unverified editor bytes"), 0o600)
	}
	name, err := windows.UTF16PtrFromString(path)
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
	_, err = f.Write([]byte("unverified editor bytes"))
	return errors.Join(err, f.Close())
}

func assertNoLabelTemporary(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".obit-labels-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("label temporary files remain: %v %v", matches, err)
	}
}

func TestLabelsCleanupTemporaryAfterFailure(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprint(existing), func(t *testing.T) {
			dir, dates := fixture(t)
			if existing {
				applyDates(t, dir, dates)
				dates["report.log"] = dates["report.log"].Add(24 * time.Hour)
			}
			p, err := prepareLabels(t, dir, dates)
			if err != nil {
				t.Fatal(err)
			}
			rejected := errors.New("reject label publication")
			beforeLabelsPublish = func(string) error { return rejected }
			t.Cleanup(func() { beforeLabelsPublish = nil })
			if err := p.Apply(); !errors.Is(err, rejected) {
				t.Fatalf("label commit rejection: %v", err)
			}
			assertNoLabelTemporary(t, dir)
			if existing && !bytes.Equal(readINI(t, dir), p.before.data) {
				t.Fatal("failed label publication changed the original")
			}
		})
	}
}

func TestLabelsTemporaryStaysInPinnedParent(t *testing.T) {
	for _, kind := range []string{"directory", "junction"} {
		t.Run(kind, func(t *testing.T) {
			if testing.Short() && kind == "junction" {
				t.Skip("requires a junction subprocess; directory replacement covers short mode")
			}
			for _, reject := range []bool{false, true} {
				t.Run(fmt.Sprint(reject), func(t *testing.T) {
					testLabelsTemporaryParentSwap(t, kind, reject)
				})
			}
		})
	}
}

func testLabelsTemporaryParentSwap(t *testing.T, kind string, reject bool) {
	t.Helper()
	dir, dates := fixture(t)
	outside := t.TempDir()
	moved := filepath.Join(t.TempDir(), "moved")
	p, err := prepareLabels(t, dir, dates)
	if err != nil {
		t.Fatal(err)
	}
	outer, err := os.OpenRoot(filepath.Dir(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer outer.Close()
	// Keep the same pinned object while allowing the fixture to move its name;
	// this isolates the creation boundary from Apply's separate directory lock.
	root, err := outer.OpenRoot(filepath.Base(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := p.check(root); err != nil {
		t.Fatal(err)
	}
	beforeLabelsWrite = func() error {
		if err := os.Rename(dir, moved); err != nil {
			return err
		}
		if kind == "junction" {
			out, err := exec.Command("cmd", "/c", "mklink", "/J", dir, outside).CombinedOutput()
			if err != nil {
				return fmt.Errorf("junction: %s: %w", out, err)
			}
		} else if err := os.Mkdir(dir, 0o700); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, "sentinel"), []byte("user content"), 0o600)
	}
	observed := false
	rejected := errors.New("reject label publication after parent swap")
	beforeLabelsPublish = func(string) error {
		observed = true
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) != 1 || entries[0].Name() != "sentinel" {
			t.Errorf("label temporary escaped pinned parent: %v: %v", entries, err)
		}
		if reject {
			return rejected
		}
		return nil
	}
	t.Cleanup(func() {
		beforeLabelsWrite, beforeLabelsPublish = nil, nil
		if kind == "junction" {
			_ = os.Remove(dir) // Remove this fixture's link, never its target.
		}
	})
	err = p.write(root)
	if !observed || reject && !errors.Is(err, rejected) {
		t.Fatalf("label parent-swap write = %v, observed=%v", err, observed)
	}
	assertNoLabelTemporary(t, dir)
	assertNoLabelTemporary(t, moved)
	data, readErr := os.ReadFile(filepath.Join(dir, "sentinel"))
	if readErr != nil || string(data) != "user content" {
		t.Fatalf("replacement directory content changed: %q %v", data, readErr)
	}
	if _, statErr := os.Stat(filepath.Join(dir, iniName)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("label publication reached replacement directory: %v", statErr)
	}
	if err == nil {
		if !bytes.Equal(readINI(t, moved), p.after) {
			t.Fatal("label publication missed pinned parent")
		}
	} else if _, statErr := os.Stat(filepath.Join(moved, iniName)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("failed label publication left a target: %v", statErr)
	}
}
