package winfs

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	findFirstFileName = kernel32.NewProc("FindFirstFileNameW")
	findNextFileName  = kernel32.NewProc("FindNextFileNameW")
)

// HardlinkNames enumerates every name on the file's volume. It accepts a fully
// resolved path so a junction cannot substitute a different volume root.
// https://learn.microsoft.com/windows/win32/api/fileapi/nf-fileapi-findfirstfilenamew
func HardlinkNames(path string) ([]string, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	volume := make([]uint16, 32768)
	if err := windows.GetVolumePathName(p, &volume[0], uint32(len(volume))); err != nil {
		return nil, err
	}
	root := windows.UTF16ToString(volume)
	buf := make([]uint16, windows.MAX_PATH)
	var handle windows.Handle
	for {
		n := uint32(len(buf))
		r, _, err := findFirstFileName.Call(uintptr(unsafe.Pointer(p)), 0, uintptr(unsafe.Pointer(&n)), uintptr(unsafe.Pointer(&buf[0])))
		handle = windows.Handle(r)
		if handle != windows.InvalidHandle {
			break
		}
		if errors.Is(err, windows.ERROR_NOT_SUPPORTED) {
			// Some SMB shares reject FindFirstFileNameW. A handle-reported
			// link count of exactly one proves this is the complete name set;
			// multiple/unknown links still fail closed, including npm installs.
			h, openErr := openFollowing(path)
			if openErr != nil {
				return nil, openErr
			}
			var info windows.ByHandleFileInformation
			infoErr := windows.GetFileInformationByHandle(h, &info)
			windows.CloseHandle(h)
			if infoErr == nil && info.NumberOfLinks == 1 {
				return []string{path}, nil
			}
		}
		if !errors.Is(err, windows.ERROR_MORE_DATA) {
			return nil, fmt.Errorf("enumerate hard-link names for %q: %w", path, err)
		}
		buf = make([]uint16, n)
	}
	defer windows.FindClose(handle)
	var names []string
	for {
		names = append(names, filepath.Join(root, strings.TrimLeft(windows.UTF16ToString(buf), `\`)))
		for {
			n := uint32(len(buf))
			r, _, err := findNextFileName.Call(uintptr(handle), uintptr(unsafe.Pointer(&n)), uintptr(unsafe.Pointer(&buf[0])))
			if r != 0 {
				break
			}
			if errors.Is(err, windows.ERROR_HANDLE_EOF) {
				return names, nil
			}
			if !errors.Is(err, windows.ERROR_MORE_DATA) {
				return nil, fmt.Errorf("enumerate remaining hard-link names for %q: %w", path, err)
			}
			buf = make([]uint16, n)
		}
	}
}
