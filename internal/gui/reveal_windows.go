package gui

import (
	"errors"
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	shell32                        = windows.NewLazySystemDLL("shell32.dll")
	procILCreateFromPathW          = shell32.NewProc("ILCreateFromPathW")
	procILFree                     = shell32.NewProc("ILFree")
	procSHOpenFolderAndSelectItems = shell32.NewProc("SHOpenFolderAndSelectItems")
)

// revealPath opens the folder holding path in Explorer with it selected.
// SHOpenFolderAndSelectItems takes an item ID list rather than a command
// line, so explorer.exe's /select, parsing never sees commas or quotes in a
// name; the item itself is only selected, never opened.
func revealPath(path string) error {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	const sFalse = 1 // COM was already initialized on this thread; still balanced below.
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE); err != nil && !errors.Is(err, syscall.Errno(sFalse)) {
		return fmt.Errorf("initialize COM: %w", err)
	}
	defer windows.CoUninitialize()
	pidl, _, _ := procILCreateFromPathW.Call(uintptr(unsafe.Pointer(name)))
	if pidl == 0 {
		return fmt.Errorf("cannot find %s", path)
	}
	defer procILFree.Call(pidl)
	if hr, _, _ := procSHOpenFolderAndSelectItems.Call(pidl, 0, 0, 0); int32(hr) < 0 {
		return fmt.Errorf("open folder of %s: HRESULT 0x%08x", path, uint32(hr))
	}
	return nil
}
