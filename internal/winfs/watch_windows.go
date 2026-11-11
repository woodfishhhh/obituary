package winfs

import (
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"golang.org/x/sys/windows"
)

// os.Stat cannot bind 128-bit identity to attributes or exclude open writers.
// Use x/sys's CreateFile sharing contract and GetFileInformationByHandle, as
// metadata_windows.go does for Git snapshots. Keep deny-write handles through
// MovePinned so a FILE_SHARE_DELETE downloader cannot write into a grave.
func (FS) ObserveWatch(path string, hold bool) (core.WatchSnapshot, func(), error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return core.WatchSnapshot{}, nil, err
	}
	share, access := uint32(shareAll), uint32(windows.FILE_READ_ATTRIBUTES)
	if hold {
		share &^= windows.FILE_SHARE_WRITE
		// Attribute-only handles bypass Windows sharing checks; request data
		// access (FILE_LIST_DIRECTORY for directories) to exclude writers.
		access |= windows.FILE_READ_DATA
	}
	h, err := windows.CreateFile(p, access, share, nil, windows.OPEN_EXISTING, openFlags, 0)
	if err != nil {
		return core.WatchSnapshot{}, nil, err
	}
	release := func() { _ = windows.CloseHandle(h) }
	id, err := identify(h, volumeRoot(path))
	if err != nil {
		release()
		return core.WatchSnapshot{}, nil, err
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		release()
		return core.WatchSnapshot{}, nil, err
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		release()
		return core.WatchSnapshot{}, nil, core.ErrUnsafe
	}
	return core.WatchSnapshot{Identity: id, Size: int64(info.FileSizeHigh)<<32 | int64(info.FileSizeLow),
		Modified: time.Unix(0, info.LastWriteTime.Nanoseconds()).UTC(), Directory: info.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0}, release, nil
}
