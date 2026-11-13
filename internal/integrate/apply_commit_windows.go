package integrate

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/winfs"
	"golang.org/x/sys/windows"
)

func integrationWindowsName(path string) (*uint16, error) {
	// Match winfs.longPath; callers supply pathutil-normalized absolute names.
	if strings.HasPrefix(path, `\\`) {
		path = `\\?\UNC\` + path[2:]
	} else {
		path = `\\?\` + path
	}
	return windows.UTF16PtrFromString(path)
}

func openIntegrationCommitFile(path string, remove bool) (*os.File, error) {
	access := uint32(0)
	if remove {
		access |= windows.DELETE
	}
	return winfs.OpenCommitFile(path, access)
}

func createIntegrationTemporary(root *os.Root, name string) (*os.File, error) {
	return winfs.CreateCommitTemporaryAt(root, name)
}

func readIntegrationOriginal(path string, remove bool) (*os.File, os.FileInfo, []byte, error) {
	f, err := openIntegrationCommitFile(path, remove)
	if err != nil {
		return nil, nil, nil, err
	}
	info, err := f.Stat()
	if err == nil {
		err = ordinaryIntegrationEntry(info)
	}
	var data []byte
	if err == nil {
		data, err = io.ReadAll(f)
	}
	if err != nil {
		f.Close()
		return nil, nil, nil, err
	}
	return f, info, data, nil
}

func (p *integrationTarget) commitOriginal(c Change) (*os.File, error) {
	f, info, data, err := readIntegrationOriginal(filepath.Join(p.root.Name(), p.name), c.delete)
	if err != nil {
		return nil, err
	}
	err = checkOriginalData(c, data, nil)
	if err == nil {
		err = checkIntegrationIdentity(c, info)
	}
	if err == nil {
		err = integrationCommitHook(c.Path)
	}
	if err == nil && c.relocated == nil && !c.generated {
		if backupErr := p.backupChange(c, data, true); backupErr != nil {
			err = fmt.Errorf("back up %s: %w", c.Path, backupErr)
		}
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

func integrationCommitHook(path string) error {
	if beforeIntegrationCommit != nil {
		return beforeIntegrationCommit(path)
	}
	return nil
}

func deleteIntegrationHandle(f *os.File) error {
	return winfs.DeleteCommitFile(f)
}

func (p *integrationTarget) removeTemporary(f *os.File, _ string) error {
	return deleteIntegrationHandle(f)
}

func (p *integrationTarget) removeOriginal(c Change) error {
	old, err := p.commitOriginal(c)
	if err != nil {
		return err
	}
	defer old.Close()
	return deleteIntegrationHandle(old)
}

func (p *integrationTarget) publish(c Change, tmp *os.File) error {
	parent, err := p.root.Open(".")
	if err != nil {
		return err
	}
	defer parent.Close()
	buf, err := integrationRenameBuffer(windows.Handle(parent.Fd()), p.name, c.existed)
	if err != nil {
		return err
	}
	if err := p.preparePublish(c); err != nil {
		return err
	}
	return renameIntegrationHandle(tmp, buf)
}

func (p *integrationTarget) preparePublish(c Change) error {
	if !c.existed {
		if err := p.checkOriginal(c); err != nil {
			return err
		}
		return integrationCommitHook(c.Path)
	}
	old, err := p.commitOriginal(c)
	if err != nil {
		return err
	}
	// POSIX replacement keeps the target name continuously present and requires
	// releasing our deny-delete handle first: an editor save in that narrow gap
	// can still be replaced; the numbered backup is the last verified version.
	return old.Close()
}

func integrationRenameBuffer(parent windows.Handle, target string, replace bool) ([]byte, error) {
	return winfs.CommitRenameBuffer(parent, target, replace)
}

func renameIntegrationHandle(tmp *os.File, buf []byte) error {
	return winfs.RenameCommitFile(tmp, buf)
}
