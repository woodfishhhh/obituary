package winfs

import (
	"fmt"
	"io/fs"
	"os"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/windows"
)

func (FS) OpenBurialDirectory(path string, want corefs.Identity) (fs.ReadDirFile, error) {
	name, err := windows.UTF16PtrFromString(path)
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

type directoryChanges windows.Handle

// A wait on the recursive kernel notification also sees events still queued for delivery.
func (FS) NewDirectoryChanges(root string) (corefs.DirectoryChanges, error) {
	volume, err := windows.UTF16PtrFromString(volumeRoot(root))
	if err != nil {
		return nil, err
	}
	if windows.GetDriveType(volume) == windows.DRIVE_REMOTE {
		return nil, fmt.Errorf("cannot verify remote directory changes")
	}
	h, err := windows.FindFirstChangeNotification(root, true, windows.FILE_NOTIFY_CHANGE_FILE_NAME|windows.FILE_NOTIFY_CHANGE_DIR_NAME|windows.FILE_NOTIFY_CHANGE_ATTRIBUTES)
	return directoryChanges(h), err
}
func (directoryChanges) WatchDirectory(string) error { return nil }
func (d directoryChanges) Close() error {
	return windows.FindCloseChangeNotification(windows.Handle(d))
}
func (d directoryChanges) Check() error {
	result, err := windows.WaitForSingleObject(windows.Handle(d), 0)
	if err != nil {
		return err
	}
	if result != uint32(windows.WAIT_TIMEOUT) {
		return fmt.Errorf("burial directory changed during inspection")
	}
	return nil
}
