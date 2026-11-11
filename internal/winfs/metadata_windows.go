package winfs

import (
	"errors"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// MetadataStamp identifies one observed version of Git's metadata. Unlike
// core.FS.Identify this permits links: it observes metadata, never moves it.
// Both identity and attributes come from the same handle. os.Stat/SameFile do
// not expose the 128-bit FileIdInfo required by the rest of the Windows layer.
type MetadataStamp struct {
	Serial    uint64
	FileID    [16]byte
	Size      uint64
	Modified  int64
	Directory bool
}

func ReadMetadata(path string) (MetadataStamp, error) {
	if stamp, supported, err := metadataByName(path); supported || err != nil {
		return stamp, err
	}
	return metadataByHandle(path)
}

func metadataByHandle(path string) (MetadataStamp, error) {
	h, err := openFollowing(path)
	if err != nil {
		return MetadataStamp{}, err
	}
	defer windows.CloseHandle(h)
	id, err := queryID(h)
	if err != nil {
		return MetadataStamp{}, err
	}
	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(h, &info); err != nil {
		return MetadataStamp{}, err
	}
	return metadataStamp(id.VolumeSerialNumber, id.FileID, uint64(info.FileSizeHigh)<<32|uint64(info.FileSizeLow), info.LastWriteTime.Nanoseconds(), info.FileAttributes), nil
}

func metadataStamp(serial uint64, id [16]byte, size uint64, modified int64, attributes uint32) MetadataStamp {
	return MetadataStamp{Serial: serial, FileID: id, Size: size, Modified: modified, Directory: attributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0}
}

var metadataByNameProc = sync.OnceValue(func() *windows.LazyProc {
	p := windows.NewLazySystemDLL("api-ms-win-core-file-l2-1-4.dll").NewProc("GetFileInformationByName")
	if p.Find() != nil {
		return nil
	}
	return p
})

// FILE_STAT_BASIC_INFORMATION and FileStatBasicByNameInfo are documented by
// Microsoft for Windows 11 24H2. Neither os.Stat nor x/sys exposes this class.
// Use the handle fallback for reparse points and unsupported systems so both
// paths preserve the full 128-bit ID and follow the same object.
type fileStatBasicInformation struct {
	FileID                int64
	CreationTime          int64
	LastAccessTime        int64
	LastWriteTime         int64
	ChangeTime            int64
	AllocationSize        int64
	EndOfFile             int64
	FileAttributes        uint32
	ReparseTag            uint32
	NumberOfLinks         uint32
	DeviceType            uint32
	DeviceCharacteristics uint32
	Reserved              uint32
	VolumeSerialNumber    uint64
	FileID128             [16]byte
}

func metadataByName(path string) (MetadataStamp, bool, error) {
	p := metadataByNameProc()
	if p == nil {
		return MetadataStamp{}, false, nil
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return MetadataStamp{}, false, err
	}
	var info fileStatBasicInformation
	r, _, err := p.Call(uintptr(unsafe.Pointer(name)), 3, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info))
	if r == 0 {
		if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) || errors.Is(err, windows.ERROR_PATH_NOT_FOUND) {
			return MetadataStamp{}, true, err
		}
		return MetadataStamp{}, false, nil // the handle query supplies the result or error
	}
	// Restrict this optimization to local disk objects with a usable ID.
	// FILE_DEVICE_DISK=7; FILE_REMOTE_DEVICE=0x10. Follow reparse points using
	// the existing openFollowing contract instead of treating their ID as data.
	if info.DeviceType != 7 || info.DeviceCharacteristics&0x10 != 0 || info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 || info.FileID128 == ([16]byte{}) {
		return MetadataStamp{}, false, nil
	}
	stamp := windows.Filetime{LowDateTime: uint32(info.LastWriteTime), HighDateTime: uint32(uint64(info.LastWriteTime) >> 32)}
	return metadataStamp(info.VolumeSerialNumber, info.FileID128, uint64(info.EndOfFile), stamp.Nanoseconds(), info.FileAttributes), true, nil
}
