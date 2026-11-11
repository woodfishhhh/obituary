package winfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"github.com/woodfishhhh/obituary/internal/pathutil"
	"golang.org/x/sys/windows"
)

// fileRenameInfo mirrors FILE_RENAME_INFO; FileName continues past the struct.
type fileRenameInfo struct {
	ReplaceIfExists uint32 // BOOLEAN in a DWORD-sized union
	_               uint32 // alignment for RootDirectory
	RootDirectory   windows.Handle
	FileNameLength  uint32
	FileName        [1]uint16
}

// MovePinned moves path to to on the same volume. The object is opened once,
// its identity is checked on that handle, and the rename is issued through
// the same handle, so the object that was checked is the object that moves.
// The rename never replaces an existing object at to.
func (fs FS) MovePinned(path string, want corefs.Identity, to string) error {
	parent, err := fs.IdentifyFollowing(filepath.Dir(to))
	if err != nil {
		return err
	}
	if parent.Serial != want.Serial {
		return fmt.Errorf("%s is not on the same volume as %s", to, path)
	}
	return movePinned(path, want, func(h windows.Handle) error { return renameByHandle(h, to) })
}

func movePinned(path string, want corefs.Identity, rename func(windows.Handle) error) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	h, err := windows.CreateFile(p, windows.DELETE|windows.SYNCHRONIZE|windows.FILE_READ_ATTRIBUTES,
		shareAll, nil, windows.OPEN_EXISTING, openFlags, 0)
	if err != nil {
		return mapNotFound(err)
	}
	defer windows.CloseHandle(h)
	got, err := identify(h, volumeRoot(path))
	if err != nil {
		return err
	}
	if got != want.Normalized() {
		return fmt.Errorf("%s is no longer the tracked object; locate it again", path)
	}
	return rename(h)
}

func renameByHandle(h windows.Handle, target string) error {
	return renameInto(h, 0, longPath(target))
}

func renameInto(h, parent windows.Handle, target string) error {
	name, err := windows.UTF16FromString(target)
	if err != nil {
		return err
	}
	nameBytes := (len(name) - 1) * 2 // without the terminating NUL
	size := int(unsafe.Offsetof(fileRenameInfo{}.FileName)) + nameBytes + 2
	buf := make([]byte, max(size, int(unsafe.Sizeof(fileRenameInfo{}))))
	info := (*fileRenameInfo)(unsafe.Pointer(&buf[0]))
	info.RootDirectory = parent
	info.FileNameLength = uint32(nameBytes)
	copy(unsafe.Slice(&info.FileName[0], len(name)), name)
	if parent == 0 {
		err = windows.SetFileInformationByHandle(h, windows.FileRenameInfo, &buf[0], uint32(len(buf)))
	} else {
		// Use the NT API for a RootDirectory-relative name to preserve the
		// pinned destination without Win32 full-path translation.
		err = windows.NtSetInformationFile(h, &windows.IO_STATUS_BLOCK{}, &buf[0], uint32(len(buf)), windows.FileRenameInformation)
		var status windows.NTStatus
		if errors.As(err, &status) {
			err = status.Errno()
		}
	}
	if err != nil {
		if errors.Is(err, windows.ERROR_ALREADY_EXISTS) || errors.Is(err, windows.ERROR_FILE_EXISTS) {
			return fmt.Errorf("%s already exists; nothing was replaced: %w", target, os.ErrExist)
		}
		return fmt.Errorf("move %s: %w", target, err)
	}
	return nil
}

func longPath(p string) string {
	switch {
	case strings.HasPrefix(p, `\\?\`):
		return p
	case strings.HasPrefix(p, `\\`):
		return `\\?\UNC\` + p[2:]
	default:
		return `\\?\` + p
	}
}

// MakeGrave creates dir, failing if it exists, after its parents; the
// per-volume `<vol>\.obituary` folder is hidden.
func (FS) MakeGrave(dir string) error {
	if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
		return err
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		return err
	}
	marker := filepath.Join(pathutil.VolumeRoot(dir), ".obituary")
	if strings.HasPrefix(strings.ToLower(dir), strings.ToLower(marker+`\`)) {
		p, err := windows.UTF16PtrFromString(marker)
		if err != nil {
			return err
		}
		attrs, err := windows.GetFileAttributes(p)
		if err == nil && attrs&windows.FILE_ATTRIBUTE_HIDDEN == 0 {
			_ = windows.SetFileAttributes(p, attrs|windows.FILE_ATTRIBUTE_HIDDEN)
		}
	}
	return nil
}

// RealPath resolves path's real location without following its last component.
func (FS) RealPath(path string) (string, error) { return Canonical(path) }
