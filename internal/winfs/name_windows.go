package winfs

import (
	"errors"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

var compareStringOrdinal = kernel32.NewProc("CompareStringOrdinal")

// SameDirectoryEntry compares potentially absent leaves using their surviving
// parent identities and the parent's actual case semantics. os.SameFile cannot
// identify an absent leaf or expose per-directory case sensitivity; reuse the
// x/sys FileCaseSensitiveInfo binding instead of guessing from path strings.
func SameDirectoryEntry(a, b string) (bool, error) {
	first, err := openFollowing(filepath.Dir(a))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(first)
	second, err := openFollowing(filepath.Dir(b))
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(second)
	id, err := queryID(first)
	if err != nil {
		return false, err
	}
	other, err := queryID(second)
	if err != nil || id != other {
		return false, err
	}
	a, b = filepath.Base(a), filepath.Base(b)
	if a == b {
		return true, nil
	}
	var flags uint32
	if err := windows.GetFileInformationByHandleEx(first, windows.FileCaseSensitiveInfo, (*byte)(unsafe.Pointer(&flags)), uint32(unsafe.Sizeof(flags))); err != nil {
		return false, err
	}
	if flags&windows.FILE_CS_FLAG_CASE_SENSITIVE_DIR != 0 {
		return false, nil
	}
	return (FS{}).SameName(a, b)
}

// CompareStringOrdinal uses the operating system's uppercase table, unlike
// Go's Unicode simple folding (which merges K and the Kelvin sign). x/sys does
// not expose this API, so only this binding is local; comparison stays in Win32.
func (FS) SameName(a, b string) (bool, error) {
	wa, err := windows.UTF16FromString(a)
	if err != nil {
		return false, err
	}
	wb, err := windows.UTF16FromString(b)
	if err != nil {
		return false, err
	}
	r, _, err := compareStringOrdinal.Call(uintptr(unsafe.Pointer(&wa[0])), uintptr(len(wa)-1), uintptr(unsafe.Pointer(&wb[0])), uintptr(len(wb)-1), 1)
	if r == 0 {
		return false, err
	}
	return r == 2, nil // CSTR_EQUAL
}
