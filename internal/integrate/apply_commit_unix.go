//go:build linux || darwin

package integrate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func createIntegrationTemporary(root *os.Root, name string) (*os.File, error) {
	return root.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
}

func (p *integrationTarget) removeTemporary(f *os.File, name string) error {
	info, err := p.root.Lstat(name)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	opened, err := f.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(info, opened) {
		return fmt.Errorf("temporary file changed: %s", name)
	}
	return p.root.Remove(name)
}

func (p *integrationTarget) verifiedPOSIXOriginal(c Change) ([]byte, error) {
	data, info, err := p.read()
	if err := checkOriginalData(c, data, err); err != nil {
		return nil, err
	}
	if err := checkIntegrationIdentity(c, info); err != nil {
		return nil, err
	}
	if c.identity != nil && (info.Size() != c.identity.Size() || !info.ModTime().Equal(c.identity.ModTime())) {
		return nil, fmt.Errorf("%s: installation metadata changed after planning", c.Path)
	}
	return data, nil
}

func syncIntegrationDirectory(root *os.Root) error {
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}

// The directory stays pinned. POSIX cannot exclude an editor after the final
// check; the numbered backup retains the verified bytes in that narrow window.
func (p *integrationTarget) publish(c Change, tmp *os.File) error {
	data, err := p.verifiedPOSIXOriginal(c)
	if err != nil {
		return err
	}
	if c.existed {
		if err := p.backupPOSIX(c, data); err != nil {
			return err
		}
	}
	if err := tmp.Sync(); err != nil {
		return err
	}
	if beforeIntegrationCommit != nil {
		if err := beforeIntegrationCommit(c.Path); err != nil {
			return err
		}
	}
	if _, err := p.verifiedPOSIXOriginal(c); err != nil {
		return err
	}
	name := filepath.Base(tmp.Name())
	if c.existed {
		err = p.root.Rename(name, p.name)
	} else {
		err = renameIntegrationExclusive(p.root, name, p.name)
	}
	if err != nil {
		return err
	}
	return syncIntegrationDirectory(p.root)
}

func (p *integrationTarget) backupPOSIX(c Change, data []byte) error {
	backup := p
	if c.backup != nil {
		var err error
		backup, err = openIntegrationTarget(*c.backup, true)
		if err != nil {
			return err
		}
		defer backup.root.Close()
	}
	for n := 0; n < 100; n++ {
		f, err := backup.root.OpenFile(backupPath(backup.name, n), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return err
		}
		_, writeErr := f.Write(data)
		if backup == p {
			p.backedUp = filepath.Join(p.root.Name(), backupPath(p.name, n))
		}
		if err := errors.Join(writeErr, f.Sync(), f.Close()); err != nil {
			return err
		}
		return syncIntegrationDirectory(backup.root)
	}
	return fmt.Errorf("too many backups of %s", c.Path)
}

// A removal preserves the actual directory entry as a sibling backup, then
// checks what moved. A conflicting editor save is restored without replacement.
func (p *integrationTarget) removeOriginal(c Change) error {
	if _, err := p.verifiedPOSIXOriginal(c); err != nil {
		return err
	}
	if beforeIntegrationCommit != nil {
		if err := beforeIntegrationCommit(c.Path); err != nil {
			return err
		}
	}
	for n := 0; n < 100; n++ {
		name := backupPath(p.name, n)
		err := renameIntegrationExclusive(p.root, p.name, name)
		if errors.Is(err, os.ErrExist) {
			continue
		}
		if err != nil {
			return err
		}
		moved := *p
		moved.name = name
		if _, err := moved.verifiedPOSIXOriginal(c); err != nil {
			if restoreErr := renameIntegrationExclusive(p.root, name, p.name); restoreErr != nil {
				return errors.Join(err, fmt.Errorf("restore refused; file retained at %s: %w", filepath.Join(p.root.Name(), name), restoreErr))
			}
			return errors.Join(err, syncIntegrationDirectory(p.root))
		}
		return syncIntegrationDirectory(p.root)
	}
	return fmt.Errorf("too many backups of %s", c.Path)
}
