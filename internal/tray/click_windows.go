//go:build windows

package tray

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/woodfishhhh/obituary/internal/desktop"
	"golang.org/x/sys/windows"
)

const (
	// Matches fyne.io/systray's legacy Shell_NotifyIconW callback message (WM_USER + 1).
	wmSystrayMessage = 0x0401
	wmLButtonUp      = 0x0202
	wmLButtonDblClk  = 0x0203
	wmNCDestroy      = 0x0082
	gwlpWndProc      = ^uintptr(3) // GWLP_WNDPROC (-4)
)

var (
	clickUser32     = windows.NewLazySystemDLL("user32.dll")
	callWindowProcW = clickUser32.NewProc("CallWindowProcW")
	defWindowProcW  = clickUser32.NewProc("DefWindowProcW")

	trayHookCallbackOnce sync.Once
	trayHookCallback     uintptr
	activeTrayHook       atomic.Pointer[trayWindowHook]
)

type trayWindowHook struct {
	hwnd          windows.Handle
	prev          uintptr
	onDoubleClick func()
}

// fyne.io/systray v1.12.2 exposes no double-click hook on Windows and opens a
// modal TrackPopupMenu synchronously on the first WM_LBUTTONUP, which consumes
// the second click before Explorer can deliver WM_LBUTTONDBLCLK. Subclassing
// our own process's SystrayClass window lets left double-click open the window
// directly while right-click continues opening the native popup menu.
func installDoubleClickHook(onDoubleClick func()) (func(), error) {
	hwnd, err := desktop.OwnWindow("SystrayClass")
	if err != nil {
		return nil, err
	}
	return attachTrayHook(hwnd, onDoubleClick)
}

func attachTrayHook(hwnd windows.Handle, onDoubleClick func()) (func(), error) {
	prev, err := getWindowProc(hwnd)
	if err != nil {
		return nil, err
	}
	cb := ensureTrayHookCallback()
	hook := &trayWindowHook{hwnd: hwnd, prev: prev, onDoubleClick: onDoubleClick}
	activeTrayHook.Store(hook)
	actualPrev, err := setWindowProc(hwnd, cb)
	if err != nil {
		activeTrayHook.CompareAndSwap(hook, nil)
		return nil, err
	}
	if actualPrev != prev {
		hook = &trayWindowHook{hwnd: hwnd, prev: actualPrev, onDoubleClick: onDoubleClick}
		activeTrayHook.Store(hook)
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			_, _ = setWindowProc(hwnd, hook.prev)
			activeTrayHook.CompareAndSwap(hook, nil)
		})
	}, nil
}

func ensureTrayHookCallback() uintptr {
	trayHookCallbackOnce.Do(func() {
		trayHookCallback = windows.NewCallback(traySubclassProc)
	})
	return trayHookCallback
}

func traySubclassProc(hwnd windows.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	hook := activeTrayHook.Load()
	if hook == nil || hook.hwnd != hwnd {
		ret, _, _ := defWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
		return ret
	}
	if dispatchTrayMessage(msg, lParam, hook.onDoubleClick) {
		return 0
	}
	prev := hook.prev
	if msg == wmNCDestroy {
		activeTrayHook.CompareAndSwap(hook, nil)
	}
	ret, _, _ := callWindowProcW.Call(prev, uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

// dispatchTrayMessage intercepts left tray clicks so WM_LBUTTONUP does not open
// a modal popup menu that swallows WM_LBUTTONDBLCLK. Right-clicks and all other
// messages return false and continue to fyne.io/systray's original WndProc.
func dispatchTrayMessage(msg uint32, lParam uintptr, onDoubleClick func()) bool {
	if msg != wmSystrayMessage {
		return false
	}
	switch lParam {
	case wmLButtonDblClk:
		if onDoubleClick != nil {
			onDoubleClick()
		}
		return true
	case wmLButtonUp:
		return true
	default:
		return false
	}
}

func windowProcAPI() (getName, setName string) {
	if runtime.GOARCH == "386" {
		return "GetWindowLongW", "SetWindowLongW"
	}
	return "GetWindowLongPtrW", "SetWindowLongPtrW"
}

func getWindowProc(hwnd windows.Handle) (uintptr, error) {
	getName, _ := windowProcAPI()
	prev, _, err := clickUser32.NewProc(getName).Call(uintptr(hwnd), gwlpWndProc)
	if prev == 0 && !errors.Is(err, windows.ERROR_SUCCESS) {
		return 0, err
	}
	return prev, nil
}

func setWindowProc(hwnd windows.Handle, proc uintptr) (uintptr, error) {
	_, setName := windowProcAPI()
	prev, _, err := clickUser32.NewProc(setName).Call(uintptr(hwnd), gwlpWndProc, proc)
	if prev == 0 && !errors.Is(err, windows.ERROR_SUCCESS) {
		return 0, err
	}
	return prev, nil
}
