package winfs

import (
	"slices"
	"strings"
	"unsafe"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/pathutil"
	"golang.org/x/sys/windows"
)

type fileAncestry struct {
	path    string
	handles []windows.Handle
	ids     []core.Identity
}

func (a *fileAncestry) Path() string                { return a.path }
func (a *fileAncestry) Identities() []core.Identity { return a.ids }
func (a *fileAncestry) Close() error {
	for _, h := range a.handles {
		windows.CloseHandle(h)
	}
	a.handles = nil
	return nil
}

// OpenFileAncestry reuses x/sys's handle/volume APIs. os.Root intentionally
// follows links and shares deletion; this proof needs no reparse traversal and
// no FILE_SHARE_DELETE while the file's ancestry is compared with the ledger.
func (fs FS) OpenFileAncestry(path string, want core.Identity) (core.FileAncestry, error) {
	want = want.Normalized()
	if len(want.Volume) != 3 || want.Volume[1:] != `:\` {
		return nil, core.ErrUnsafe // network/volume aliases are not a local proof
	}
	root, err := windows.UTF16PtrFromString(want.Volume)
	if err != nil || windows.GetDriveType(root) != windows.DRIVE_FIXED {
		return nil, core.ErrUnsafe
	}
	path, complete, err := fs.LongPath(path)
	if err != nil || !complete {
		return nil, core.ErrUnsafe
	}
	var paths []string
	for dir := path; ; dir = pathutil.Parent(dir) {
		paths = append(paths, dir)
		if pathutil.Parent(dir) == dir {
			break
		}
	}
	slices.Reverse(paths)
	a := &fileAncestry{path: path}
	for i, name := range paths {
		if err := a.pin(name, want.Volume, i == len(paths)-1); err != nil {
			a.Close()
			return nil, err
		}
	}
	if !a.confirms(want) {
		a.Close()
		return nil, core.ErrUnsafe
	}
	return a, nil
}

func (a *fileAncestry) confirms(want core.Identity) bool {
	if a.ids[len(a.ids)-1] != want || !stableRegistrationVolume(a.handles[0]) {
		return false
	}
	// A reparse target or DOS-device alias can change without renaming a pinned
	// directory. The final pinned file must still occupy the captured namespace.
	path, err := checkedFinalPath(a.handles[len(a.handles)-1], open)
	if err != nil {
		return false
	}
	same, err := (FS{}).SameName(a.path, path)
	return err == nil && same
}

func (a *fileAncestry) pin(path, volume string, file bool) error {
	if err := pathutil.Unchanged(path); err != nil {
		return err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	// Metadata-only handles do not enforce the delete-sharing exclusion on
	// Windows. READ_DATA (LIST_DIRECTORY for parents) makes the name pin real;
	// denied data/list access simply makes this optional optimization unavailable.
	h, err := windows.CreateFile(name, windows.FILE_READ_ATTRIBUTES|windows.FILE_READ_DATA,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, openFlags, 0)
	if err != nil {
		return err
	}
	a.handles = append(a.handles, h)
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		return err
	}
	directory := info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0
	if file == directory || info.FileAttributes&(windows.FILE_ATTRIBUTE_REPARSE_POINT|windows.FILE_ATTRIBUTE_DEVICE) != 0 {
		return core.ErrUnsafe
	}
	if directory {
		var flags uint32
		if err := windows.GetFileInformationByHandleEx(h, windows.FileCaseSensitiveInfo, (*byte)(unsafe.Pointer(&flags)), 4); err != nil || flags != 0 {
			return core.ErrUnsafe
		}
	}
	id, err := identify(h, volume)
	if err != nil {
		return err
	}
	if len(a.ids) > 0 && id.Serial != a.ids[0].Serial {
		return core.ErrUnsafe
	}
	a.ids = append(a.ids, id)
	return nil
}

func stableRegistrationVolume(h windows.Handle) bool {
	var flags uint32
	var name [32]uint16
	err := windows.GetVolumeInformationByHandle(h, nil, 0, nil, nil, &flags, &name[0], uint32(len(name)))
	format := windows.UTF16ToString(name[:])
	return err == nil && stableRegistrationFormat(format, flags)
}

func stableRegistrationFormat(format string, flags uint32) bool {
	return flags&windows.FILE_SUPPORTS_OPEN_BY_FILE_ID != 0 &&
		(strings.EqualFold(format, "NTFS") || strings.EqualFold(format, "ReFS"))
}
