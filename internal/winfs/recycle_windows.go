package winfs

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"golang.org/x/sys/windows"
)

// Shell IFileOperation, called through its vtable.
var (
	clsidFileOperation = windows.GUID{Data1: 0x3AD05575, Data2: 0x8857, Data3: 0x4850, Data4: [8]byte{0x92, 0x77, 0x11, 0xB8, 0x5B, 0xDB, 0x8E, 0x09}}
	iidFileOperation   = windows.GUID{Data1: 0x947AAB5F, Data2: 0x0A5C, Data3: 0x4C13, Data4: [8]byte{0xB4, 0xD6, 0x4B, 0xF7, 0x83, 0x6F, 0xC9, 0xF8}}
	iidShellItem       = windows.GUID{Data1: 0x43826D1E, Data2: 0xE718, Data3: 0x42EE, Data4: [8]byte{0xBC, 0x55, 0xA1, 0xE2, 0x61, 0xC3, 0x7B, 0xFE}}

	ole32                           = windows.NewLazySystemDLL("ole32.dll")
	shell32                         = windows.NewLazySystemDLL("shell32.dll")
	procCoCreateInstance            = ole32.NewProc("CoCreateInstance")
	procSHCreateItemFromParsingName = shell32.NewProc("SHCreateItemFromParsingName")
)

const (
	clsctxInprocServer = 0x1

	fofSilent           = 0x0004
	fofNoConfirmation   = 0x0010
	fofAllowUndo        = 0x0040
	fofNoConfirmMkdir   = 0x0200
	fofNoErrorUI        = 0x0400
	fofNoConnected      = 0x2000 // do not drag along "connected" files such as report_files for report.html
	fofWantNukeWarning  = 0x4000
	fofxRecycleOnDelete = 0x00080000
	fofxEarlyFailure    = 0x00100000
	recycleFlags        = fofSilent | fofNoConfirmation | fofNoConfirmMkdir | fofNoErrorUI | fofNoConnected | fofAllowUndo | fofWantNukeWarning | fofxRecycleOnDelete | fofxEarlyFailure
	sFalse              = 1
	vtRelease           = 2
	vtSetOperationFlags = 5
	vtDeleteItem        = 18
	vtPerformOperations = 21
)

// ErrNotRecycled means the shell refused to put the item in the Recycle Bin
// (for example it is too large or on a drive without one). Nothing was deleted.
var ErrNotRecycled = errors.New("Windows would not move it to the Recycle Bin (too large, on a drive without one, or in use); nothing was deleted") //nolint:staticcheck // ST1005: Windows is a proper noun in this public diagnostic.

type comObject struct{ vtbl *[32]uintptr }

func (o *comObject) call(method int, args ...uintptr) uintptr {
	r, _, _ := syscall.SyscallN(o.vtbl[method], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	return r
}

func (o *comObject) release() { o.call(vtRelease) }

func hresult(r uintptr, what string) error {
	if int32(r) < 0 {
		return fmt.Errorf("%s: %w", what, syscall.Errno(r))
	}
	return nil
}

// Recycle moves path to the Recycle Bin with the shell's IFileOperation. The
// object at path is checked against want immediately before the shell opens
// it, narrowing the window in which a swapped-in object could be hit. The
// nuke warning plus no-UI flags make the shell skip, not permanently delete,
// anything that cannot be recycled; the result is then verified on disk.
func (fs FS) Recycle(path string, want corefs.Identity) error {
	got, err := fs.Identify(path)
	if err != nil {
		return err
	}
	if got != want {
		return fmt.Errorf("%s is no longer the tracked object: %w", path, corefs.ErrUnsafe)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED|windows.COINIT_DISABLE_OLE1DDE); err != nil && !errors.Is(err, syscall.Errno(sFalse)) {
		return fmt.Errorf("initialize COM: %w", err)
	}
	defer windows.CoUninitialize()

	var op *comObject
	r, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidFileOperation)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidFileOperation)), uintptr(unsafe.Pointer(&op)))
	if err := hresult(r, "create file operation"); err != nil {
		return err
	}
	defer op.release()
	if err := hresult(op.call(vtSetOperationFlags, recycleFlags), "set operation flags"); err != nil {
		return err
	}

	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	var item *comObject
	r, _, _ = procSHCreateItemFromParsingName.Call(uintptr(unsafe.Pointer(name)), 0,
		uintptr(unsafe.Pointer(&iidShellItem)), uintptr(unsafe.Pointer(&item)))
	if err := hresult(r, "open "+path); err != nil {
		return err
	}
	defer item.release()

	if err := hresult(op.call(vtDeleteItem, uintptr(unsafe.Pointer(item)), 0), "queue recycle"); err != nil {
		return err
	}
	performErr := hresult(op.call(vtPerformOperations), "recycle")
	return confirmRecycled(path, performErr)
}

func confirmRecycled(path string, performErr error) error {
	// Trust the disk, not the shell's report: success means the path is gone.
	_, statErr := os.Lstat(path)
	switch {
	case errors.Is(statErr, os.ErrNotExist):
		return nil
	case statErr != nil:
		return fmt.Errorf("could not confirm the result: %w", statErr)
	case performErr != nil:
		return performErr
	default:
		return ErrNotRecycled
	}
}
