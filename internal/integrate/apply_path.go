package integrate

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// The plan stores identity, not open handles: inspecting a plan must not keep
// directories locked. Apply reopens and compares this nearest existing parent.
type integrationLocation struct {
	path string
	info os.FileInfo
}

type integrationTarget struct {
	root     *os.Root
	name     string
	location integrationLocation
}

// Test seam after pinning the destination parent, before any mutation in it.
var beforeIntegrationWrite func(string) error

// Test seam after the final content check, while the existing file is locked.
var beforeIntegrationCommit func(string) error

// Test seam after verification and locking, before temporary-file creation.
var beforeIntegrationTemporary func(*integrationTarget) error

// Root.OpenRoot uses Windows Openat with FILE_SHARE_DELETE. That is safe for
// our handle-relative writes, but an external host will reopen an absolute
// filename. os.OpenRoot uses syscall.Open without sharing delete: hold that
// extra handle, verify it is our pinned parent, then reject any intervening
// reparse before handing the path to the host. The handle prevents renames for
// the entire CLI call, while the CLI may still atomically replace its file.
func (p *integrationTarget) lockHostDirectory() error {
	locked, err := os.OpenRoot(p.root.Name())
	if err != nil {
		return err
	}
	info, err := locked.Stat(".")
	if err == nil && !os.SameFile(info, p.location.info) {
		err = fmt.Errorf("%s: host configuration directory changed while locking", p.root.Name())
	}
	if err == nil {
		err = checkInside(p.root.Name(), p.root.Name())
	}
	if err != nil {
		locked.Close()
		return err
	}
	p.root.Close()
	p.root = locked
	return nil
}

// Use Go's os/root_windows.go and internal/syscall/windows/at_windows.go rather
// than reimplementing NtCreateFile/Renameat. Walk from the volume by directory
// handles, including the scope root, and retain the final parent for ALL I/O.
// Root methods reject escaping symlinks even if a name changes after Lstat.
func openIntegrationTarget(c Change, create bool) (_ *integrationTarget, err error) {
	path, err := pathutil.Abs(c.Path)
	if err != nil {
		return nil, err
	}
	if c.project != "" {
		rel, err := filepath.Rel(c.project, path)
		if err != nil || !filepath.IsLocal(rel) {
			return nil, fmt.Errorf("%s is outside the project %s", path, c.project)
		}
	}
	volume := pathutil.VolumeRoot(path)
	root, err := os.OpenRoot(volume)
	if err != nil {
		return nil, err
	}
	rel, err := filepath.Rel(volume, path)
	if err != nil {
		root.Close()
		return nil, err
	}
	parts := strings.Split(rel, string(filepath.Separator))
	return walkIntegrationTarget(c, create, root, parts)
}

func walkIntegrationTarget(c Change, create bool, root *os.Root, parts []string) (_ *integrationTarget, err error) {
	defer func() {
		if err != nil {
			root.Close()
		}
	}()
	matched := c.location == nil
	for i := 0; ; i++ {
		info, err := root.Stat(".") // handle-derived FileInfo, including its file ID
		if err != nil {
			return nil, err
		}
		if c.location != nil && root.Name() == c.location.path {
			if !os.SameFile(info, c.location.info) {
				return nil, fmt.Errorf("%s: installation directory changed after planning", root.Name())
			}
			matched = true
		}
		if i == len(parts)-1 {
			if !matched {
				return nil, fmt.Errorf("%s: planned installation directory is missing", c.Path)
			}
			return &integrationTarget{root: root, name: parts[i], location: integrationLocation{root.Name(), info}}, nil
		}
		childInfo, err := root.Lstat(parts[i])
		if errors.Is(err, os.ErrNotExist) {
			if !matched {
				return nil, fmt.Errorf("%s: planned installation directory is missing", c.Path)
			}
			if !create {
				return &integrationTarget{root: root, name: filepath.Join(parts[i:]...), location: integrationLocation{root.Name(), info}}, nil
			}
			if err := root.Mkdir(parts[i], 0o755); err != nil && !errors.Is(err, os.ErrExist) {
				return nil, err
			}
			childInfo, err = root.Lstat(parts[i])
		}
		if err != nil {
			return nil, err
		}
		child, err := openIntegrationChild(root, parts[i], c.Path, childInfo)
		if err != nil {
			return nil, err
		}
		root.Close()
		root = child
	}
}

func openIntegrationChild(root *os.Root, name, path string, childInfo os.FileInfo) (*os.Root, error) {
	if err := ordinaryIntegrationEntry(childInfo); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Join(root.Name(), name), err)
	}
	child, err := root.OpenRoot(name)
	if err != nil {
		return nil, err
	}
	opened, statErr := child.Stat(".")
	if statErr != nil || !os.SameFile(childInfo, opened) {
		child.Close()
		return nil, fmt.Errorf("%s: installation directory changed while opening: %w", path, errors.Join(statErr, os.ErrInvalid))
	}
	return child, nil
}

func ordinaryIntegrationEntry(info os.FileInfo) error {
	if info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
		return fmt.Errorf("refusing a link or junction")
	}
	return nil
}

func (p *integrationTarget) read() ([]byte, os.FileInfo, error) {
	info, err := p.root.Lstat(p.name)
	if err != nil {
		return nil, nil, err
	}
	if err := ordinaryIntegrationEntry(info); err != nil {
		return nil, nil, err
	}
	f, err := p.root.Open(p.name)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil {
		return nil, nil, err
	}
	if !os.SameFile(info, opened) {
		return nil, nil, fmt.Errorf("%s changed while opening", p.name)
	}
	data, err := io.ReadAll(f)
	return data, opened, err
}

func (p *integrationTarget) checkOriginal(c Change) error {
	data, info, err := p.read()
	if err := checkOriginalData(c, data, err); err != nil {
		return err
	}
	return checkIntegrationIdentity(c, info)
}

func checkIntegrationIdentity(c Change, info os.FileInfo) error {
	if c.claude == nil && c.identity != nil && !os.SameFile(c.identity, info) {
		return fmt.Errorf("%s: installation file changed after planning", c.Path)
	}
	return nil
}

// Existing numbered backups are never overwritten, including links planted
// after preflight. Both the probe and exclusive creation stay under this handle.
func (p *integrationTarget) backup(data []byte, write bool) error {
	for n := 0; n < 100; n++ {
		name := backupPath(p.name, n)
		info, err := p.root.Lstat(name)
		if err == nil {
			if err := ordinaryIntegrationEntry(info); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			continue
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if !write {
			return nil
		}
		f, err := p.root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if errors.Is(err, os.ErrExist) {
			// A competing backup is allowed; a newly planted link is not.
			info, err := p.root.Lstat(name)
			if err != nil {
				return err
			}
			if err := ordinaryIntegrationEntry(info); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		_, err = f.Write(data)
		return errors.Join(err, f.Close())
	}
	return fmt.Errorf("too many backups of %s", p.name)
}

func (p *integrationTarget) write(c Change) (err error) {
	if beforeIntegrationWrite != nil {
		if err := beforeIntegrationWrite(c.Path); err != nil {
			return err
		}
	}
	if err := p.checkOriginal(c); err != nil {
		return err
	}
	// Win32 commit APIs use absolute names. Reuse the host-directory lock to
	// keep those names bound to this already checked parent throughout commit.
	if err := p.lockHostDirectory(); err != nil {
		return err
	}
	if c.delete {
		return p.removeOriginal(c)
	}
	if beforeIntegrationTemporary != nil {
		if err := beforeIntegrationTemporary(p); err != nil {
			return err
		}
	}
	tmp := ".obit-" + rand.Text() + ".tmp"
	f, err := createIntegrationTemporary(p.root, tmp)
	if err != nil {
		return err
	}
	published := false
	defer func() {
		if !published {
			err = errors.Join(err, p.removeTemporary(f, tmp))
		}
		err = errors.Join(err, f.Close())
	}()
	_, writeErr := f.Write(c.data)
	info, statErr := f.Stat()
	if err := errors.Join(writeErr, statErr); err != nil {
		return err
	}
	if err := p.publish(c, f); err != nil {
		return err
	}
	published = true
	if c.next != nil {
		c.next.identity = info // the final receipt expects our pending receipt
	}
	return nil
}
