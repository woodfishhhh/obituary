// Package desktop changes presentation of windows owned by this process.
package desktop

import (
	"errors"
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

const WindowClass = "ObituaryManagementWindow"

var user32 = windows.NewLazySystemDLL("user32.dll")
var findWindow = user32.NewProc("FindWindowExW")
var windowPID = user32.NewProc("GetWindowThreadProcessId")
var showWindow = user32.NewProc("ShowWindow")
var visibleWindow = user32.NewProc("IsWindowVisible")

// OwnWindow checks the PID as window class names are shared across processes.
func OwnWindow(class string) (windows.Handle, error) {
	name, err := windows.UTF16PtrFromString(class)
	if err != nil {
		return 0, err
	}
	var after uintptr
	for {
		hwnd, _, _ := findWindow.Call(0, after, uintptr(unsafe.Pointer(name)), 0)
		if hwnd == 0 {
			return 0, fmt.Errorf("window %s is not ready", class)
		}
		var pid uint32
		windowPID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if pid == uint32(os.Getpid()) {
			return windows.Handle(hwnd), nil
		}
		after = hwnd
	}
}

func taskbarStyle(style uintptr, enabled bool) uintptr {
	const toolWindow, appWindow = 0x80, 0x40000
	if enabled {
		return (style &^ toolWindow) | appWindow
	}
	return (style &^ appWindow) | toolWindow
}

// Wails v2 exposes no taskbar switch. Win32 requires hiding the window while
// changing extended styles; only our own native management window is touched.
func SetTaskbar(enabled bool) error {
	hwnd, err := OwnWindow(WindowClass)
	if err != nil {
		return err
	}
	getName, setName := "GetWindowLongPtrW", "SetWindowLongPtrW"
	if runtime.GOARCH == "386" {
		getName, setName = "GetWindowLongW", "SetWindowLongW"
	}
	index := int32(-20)
	style, _, _ := user32.NewProc(getName).Call(uintptr(hwnd), uintptr(index))
	next := taskbarStyle(style, enabled)
	if next == style {
		return nil
	}
	visible, _, _ := visibleWindow.Call(uintptr(hwnd))
	if visible != 0 {
		showWindow.Call(uintptr(hwnd), 0)
		defer showWindow.Call(uintptr(hwnd), 5)
	}
	result, _, err := user32.NewProc(setName).Call(uintptr(hwnd), uintptr(index), next)
	if result == 0 && !errors.Is(err, windows.ERROR_SUCCESS) {
		return err
	}
	return nil
}
