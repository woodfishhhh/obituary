package winfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/windows"
)

type restoreParent struct {
	h       windows.Handle
	path    string
	id      corefs.Identity
	created bool
}

func (p *restoreParent) Rollback() error {
	if !p.created {
		return nil
	}
	// Go 1.26 internal/syscall/windows.Deleteat: POSIX semantics unlink an
	// open child now, so its parent can also be rolled back before handles close.
	// os.Root.Remove reopens by name; rollback must use this creation handle.
	flags := uint32(windows.FILE_DISPOSITION_DELETE | windows.FILE_DISPOSITION_POSIX_SEMANTICS)
	var iosb windows.IO_STATUS_BLOCK
	err := windows.NtSetInformationFile(p.h, &iosb, (*byte)(unsafe.Pointer(&flags)), 4, 64 /* FileDispositionInformationEx */)
	if errors.Is(err, windows.STATUS_INVALID_INFO_CLASS) || errors.Is(err, windows.STATUS_INVALID_PARAMETER) || errors.Is(err, windows.STATUS_NOT_SUPPORTED) {
		var disposition byte = 1
		err = windows.NtSetInformationFile(p.h, &iosb, &disposition, 1, windows.FileDispositionInformation)
		if err == nil {
			return fmt.Errorf("rollback of %s is pending handle closure; POSIX directory deletion is unavailable", p.path)
		}
	}
	if errors.Is(err, windows.STATUS_DIRECTORY_NOT_EMPTY) {
		return fmt.Errorf("rollback kept nonempty created directory %s: %w", p.path, err)
	}
	return err
}

func (p *restoreParent) Close() error              { return windows.CloseHandle(p.h) }
func (p *restoreParent) Path() string              { return p.path }
func (p *restoreParent) Identity() corefs.Identity { return p.id }

// OpenRestoreParent pins the checked destination without following its final
// reparse point. FILE_RENAME_INFORMATION requires traverse/read-attribute access
// on RootDirectory; sharing delete permits ordinary concurrent directory moves.
func (FS) OpenRestoreParent(path string) (corefs.RestoreParent, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(name, windows.FILE_TRAVERSE|windows.FILE_READ_ATTRIBUTES,
		shareAll, nil, windows.OPEN_EXISTING, openFlags, 0)
	if err != nil {
		return nil, err
	}
	var info windows.ByHandleFileInformation
	err = windows.GetFileInformationByHandle(h, &info)
	if err == nil && (info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0) {
		err = fmt.Errorf("restore parent is not an ordinary directory: %w", corefs.ErrUnsafe)
	}
	var id corefs.Identity
	if err == nil {
		id, err = identify(h, volumeRoot(path))
	}
	if err == nil {
		path, err = checkedFinalPath(h, open)
	}
	if err != nil {
		windows.CloseHandle(h)
		return nil, err
	}
	return &restoreParent{h: h, path: path, id: id}, nil
}

// MkdirRestoreParent creates directories relative to the pinned parent,
// retaining the returned handle and rejecting every reparse tag. os.Root's
// traversal intentionally follows in-root links, which restore cannot accept.
func (FS) MkdirRestoreParent(parent corefs.RestoreParent, name string) (corefs.RestoreParent, error) {
	p, ok := parent.(*restoreParent)
	if !ok || name == "" || name == "." || name == ".." || strings.ContainsAny(name, `\/:`) {
		return nil, fmt.Errorf("invalid restore child: %w", corefs.ErrUnsafe)
	}
	n, err := windows.NewNTUnicodeString(name)
	if err != nil {
		return nil, err
	}
	attrs := windows.OBJECT_ATTRIBUTES{RootDirectory: p.h, ObjectName: n, Attributes: windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE}
	attrs.Length = uint32(unsafe.Sizeof(attrs))
	var h windows.Handle
	var iosb windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(&h, windows.FILE_GENERIC_READ|windows.DELETE, &attrs, &iosb, nil,
		windows.FILE_ATTRIBUTE_NORMAL, shareAll, windows.FILE_OPEN_IF,
		windows.FILE_DIRECTORY_FILE|windows.FILE_OPEN_REPARSE_POINT, 0, 0)
	if err != nil {
		return nil, err
	}
	var info windows.ByHandleFileInformation
	err = windows.GetFileInformationByHandle(h, &info)
	if err == nil && info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		err = fmt.Errorf("restore child is a reparse point: %w", corefs.ErrUnsafe)
	}
	var id corefs.Identity
	var path string
	if err == nil {
		id, err = identify(h, volumeRoot(p.path))
	}
	if err == nil {
		path, err = checkedFinalPath(h, open)
	}
	child := &restoreParent{h: h, path: filepath.Clean(path), id: id, created: iosb.Information == 2 /* FILE_CREATED */}
	if err == nil && id.Serial != p.id.Serial {
		err = corefs.ErrUnsafe
	}
	if err != nil {
		err = errors.Join(err, child.Rollback())
		child.Close()
		return nil, err
	}
	return child, nil
}

// FindRestored searches only the recorded parent and compares file IDs on
// opened handles. A matching filename alone never proves a committed restore.
func (fs FS) FindRestored(parent, source corefs.Identity) (string, error) {
	return fs.findRestored(parent, source, fs.Locate)
}

func (fs FS) findRestored(parent, source corefs.Identity, locateParent func(corefs.Identity) (string, error)) (string, error) {
	dir, err := locateParent(parent)
	if err != nil {
		return "", err
	}
	pinned, err := fs.OpenRestoreParent(dir)
	if err != nil {
		return "", err
	}
	defer pinned.Close()
	p := pinned.(*restoreParent)
	if p.id.Serial != parent.Serial || p.id.FileID != parent.FileID {
		return "", corefs.ErrUnsafe
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var inspectErr error
	for _, entry := range entries {
		n, err := windows.NewNTUnicodeString(entry.Name())
		if err != nil {
			return "", err
		}
		h, err := openRestoredChild(p.h, n)
		if err != nil {
			inspectErr = errors.Join(inspectErr, err)
			continue
		}
		id, err := identify(h, parent.Volume)
		if err != nil {
			windows.CloseHandle(h)
			inspectErr = errors.Join(inspectErr, err)
			continue
		}
		if id.Serial != source.Serial || id.FileID != source.FileID {
			windows.CloseHandle(h)
			continue
		}
		actual, err := checkedFinalPath(h, open)
		windows.CloseHandle(h)
		return actual, err
	}
	if inspectErr != nil {
		return "", fmt.Errorf("restored identity could not be located in its recorded parent: %w", inspectErr)
	}
	return "", fmt.Errorf("restored identity is absent from its recorded parent: %w", corefs.ErrNotFound)
}

func openRestoredChild(parent windows.Handle, name *windows.NTUnicodeString) (windows.Handle, error) {
	attrs := windows.OBJECT_ATTRIBUTES{RootDirectory: parent, ObjectName: name, Attributes: windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE}
	attrs.Length = uint32(unsafe.Sizeof(attrs))
	var h windows.Handle
	err := windows.NtCreateFile(&h, windows.FILE_READ_ATTRIBUTES, &attrs, &windows.IO_STATUS_BLOCK{}, nil,
		windows.FILE_ATTRIBUTE_NORMAL, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, windows.FILE_OPEN,
		windows.FILE_OPEN_REPARSE_POINT|windows.FILE_OPEN_FOR_BACKUP_INTENT, 0, 0)
	return h, err
}

// MovePinnedInto uses a single-component relative name and the already opened
// parent handle. The source identity is checked on the handle being renamed.
func (FS) MovePinnedInto(path string, want corefs.Identity, parent corefs.RestoreParent, name string) (string, error) {
	p, ok := parent.(*restoreParent)
	if !ok || name == "" || name == "." || name == ".." || strings.ContainsAny(name, `\/:`) {
		return "", fmt.Errorf("invalid restore parent or filename: %w", corefs.ErrUnsafe)
	}
	var restored string
	err := movePinned(path, want, func(h windows.Handle) error {
		if err := renameInto(h, p.h, name); err != nil {
			return err
		}
		// The move already committed. An unavailable name must not turn it into
		// a retry; the empty result tells core to audit its expected-path fallback.
		if actual, err := checkedFinalPath(h, open); err == nil {
			restored = actual
		}
		return nil
	})
	return restored, err
}
