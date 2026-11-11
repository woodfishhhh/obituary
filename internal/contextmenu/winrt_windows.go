package contextmenu

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Minimal WinRT ABI, following internal/winfs/recycle_windows.go and Microsoft's
// win32metadata WinSDK headers (windows.data.xml.dom.h/windows.ui.notifications.h).
// The standard library/x/sys has no toast projection. go-toast/toast's toast.go
// shells out to PowerShell; go-ole's winrt.go counts runes rather than UTF-16 units
// for HSTRING, truncating non-BMP filenames. Neither fits this direct native path.
var (
	combase                = windows.NewLazySystemDLL("combase.dll")
	roInitialize           = combase.NewProc("RoInitialize")
	roUninitialize         = combase.NewProc("RoUninitialize")
	roActivateInstance     = combase.NewProc("RoActivateInstance")
	roGetActivationFactory = combase.NewProc("RoGetActivationFactory")
	windowsCreateString    = combase.NewProc("WindowsCreateString")
	windowsDeleteString    = combase.NewProc("WindowsDeleteString")
)

type runtimeObject struct{ vtable *[16]uintptr }

//go:uintptrescapes
func (o *runtimeObject) call(method int, args ...uintptr) uintptr {
	r, _, _ := syscall.SyscallN(o.vtable[method], append([]uintptr{uintptr(unsafe.Pointer(o))}, args...)...)
	return r
}

func (o *runtimeObject) release() { o.call(2) }

func winrtError(hr uintptr, operation string) error {
	if int32(hr) < 0 {
		return fmt.Errorf("%s: HRESULT 0x%08x", operation, uint32(hr))
	}
	return nil // S_FALSE also initializes the apartment successfully.
}

func withWinRT(fn func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	hr, _, _ := roInitialize.Call(1) // RO_INIT_MULTITHREADED
	if err := winrtError(hr, "initialize WinRT"); err != nil {
		return err
	}
	defer roUninitialize.Call()
	return fn()
}

func withHString(value string, fn func(uintptr) error) error {
	u16, err := windows.UTF16FromString(value)
	if err != nil {
		return err
	}
	var handle uintptr
	hr, _, _ := windowsCreateString.Call(uintptr(unsafe.Pointer(&u16[0])), uintptr(len(u16)-1), uintptr(unsafe.Pointer(&handle)))
	if err := winrtError(hr, "create HSTRING"); err != nil {
		return err
	}
	defer windowsDeleteString.Call(handle)
	return fn(handle)
}

func activate(class string, iid *windows.GUID) (*runtimeObject, error) {
	var object *runtimeObject
	err := withHString(class, func(name uintptr) error {
		var hr uintptr
		if iid == nil {
			hr, _, _ = roActivateInstance.Call(name, uintptr(unsafe.Pointer(&object)))
		} else {
			hr, _, _ = roGetActivationFactory.Call(name, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&object)))
		}
		return winrtError(hr, "activate "+class)
	})
	return object, err
}

func (o *runtimeObject) query(iid *windows.GUID) (*runtimeObject, error) {
	var result *runtimeObject
	err := winrtError(o.call(0, uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&result))), "query WinRT interface")
	return result, err
}
