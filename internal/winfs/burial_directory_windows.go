package winfs

import (
	"fmt"
	"io/fs"
	"os"
	"strings"
	"unicode/utf8"
	"unsafe"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/windows"
)

func (FS) OpenBurialDirectory(path string, want corefs.Identity) (fs.ReadDirFile, error) {
	name, err := windows.UTF16PtrFromString(longPath(path))
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateFile(name, windows.FILE_LIST_DIRECTORY|windows.FILE_READ_ATTRIBUTES, shareAll, nil, windows.OPEN_EXISTING, openFlags, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(h), path)
	var info windows.ByHandleFileInformation
	err = windows.GetFileInformationByHandle(h, &info)
	if err == nil && (info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0) {
		err = corefs.ErrUnsafe
	}
	if err == nil {
		var got corefs.Identity
		got, err = identify(h, want.Volume)
		if err == nil && got != want {
			err = corefs.ErrUnsafe
		}
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return f, nil
}

func (FS) BurialEntry(_ fs.ReadDirFile, path string, dir corefs.Identity) (corefs.Identity, bool, error) {
	// ReadDir replaces unpaired UTF-16 surrogates with RuneError. Its name may alias a sibling.
	if strings.ContainsRune(path, utf8.RuneError) {
		return corefs.Identity{}, false, corefs.ErrUnsafe
	}
	h, err := open(longPath(path))
	if err != nil {
		return corefs.Identity{}, false, err
	}
	defer windows.CloseHandle(h)
	// FILE_ATTRIBUTE_TAG_INFO is two DWORDs. x/sys exposes the query but not its result structure.
	var info struct{ Attributes, Tag uint32 }
	if err = windows.GetFileInformationByHandleEx(h, windows.FileAttributeTagInfo, (*byte)(unsafe.Pointer(&info)), uint32(unsafe.Sizeof(info))); err != nil {
		return corefs.Identity{}, false, err
	}
	descend, err := burialDescent(info.Attributes, info.Tag)
	if err != nil {
		return corefs.Identity{}, false, err
	}
	id, err := identify(h, dir.Volume)
	return id, descend, err
}

func burialDescent(attributes, tag uint32) (bool, error) {
	if attributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return false, nil
	}
	if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
		return true, nil
	}
	if tag == windows.IO_REPARSE_TAG_SYMLINK || tag == windows.IO_REPARSE_TAG_MOUNT_POINT {
		return false, nil
	}
	return false, fmt.Errorf("unverified directory reparse tag %#x", tag)
}

type directoryChanges struct {
	handle windows.Handle
	root   string
	id     corefs.Identity
}

// A wait on the recursive kernel notification also sees events still queued for delivery.
func (FS) NewDirectoryChanges(root string) (corefs.DirectoryChanges, error) {
	volume, err := windows.UTF16PtrFromString(volumeRoot(root))
	if err != nil {
		return nil, err
	}
	if windows.GetDriveType(volume) == windows.DRIVE_REMOTE {
		return nil, fmt.Errorf("cannot verify remote directory changes")
	}
	id, err := (FS{}).Identify(root)
	if err != nil {
		return nil, err
	}
	h, err := windows.FindFirstChangeNotification(longPath(root), true, windows.FILE_NOTIFY_CHANGE_FILE_NAME|windows.FILE_NOTIFY_CHANGE_DIR_NAME|windows.FILE_NOTIFY_CHANGE_ATTRIBUTES|windows.FILE_NOTIFY_CHANGE_SECURITY)
	if err != nil {
		return nil, err
	}
	return directoryChanges{h, root, id}, nil
}
func (directoryChanges) WatchDirectory(string) error { return nil }
func (d directoryChanges) Close() error {
	return windows.FindCloseChangeNotification(d.handle)
}
func (d directoryChanges) Check() error {
	f, err := (FS{}).OpenBurialDirectory(d.root, d.id)
	if err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	result, err := windows.WaitForSingleObject(d.handle, 0)
	if err != nil {
		return err
	}
	if result != uint32(windows.WAIT_TIMEOUT) {
		return fmt.Errorf("burial directory changed during inspection")
	}
	return nil
}
