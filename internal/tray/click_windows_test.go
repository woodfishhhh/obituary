//go:build windows

package tray

import (
	"runtime"
	"sync"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	wmLButtonDown = 0x0201
	wmRButtonDown = 0x0204
	wmRButtonUp   = 0x0205
	wmCommand     = 0x0111
)

func TestTrayMessageDispatch(t *testing.T) {
	tests := []struct {
		name        string
		msg         uint32
		lParam      uintptr
		wantHandled bool
		wantDblClk  int
	}{
		{"left double click opens panel", wmSystrayMessage, wmLButtonDblClk, true, 1},
		{"left single click swallowed to avoid blocking double click", wmSystrayMessage, wmLButtonUp, true, 0},
		{"right click forwarded to native popup menu", wmSystrayMessage, wmRButtonUp, false, 0},
		{"left button down forwarded", wmSystrayMessage, wmLButtonDown, false, 0},
		{"right button down forwarded", wmSystrayMessage, wmRButtonDown, false, 0},
		{"menu command forwarded", wmCommand, 1, false, 0},
		{"ncdestroy forwarded", wmNCDestroy, 0, false, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clicks := 0
			got := dispatchTrayMessage(tc.msg, tc.lParam, func() { clicks++ })
			if got != tc.wantHandled || clicks != tc.wantDblClk {
				t.Fatalf("dispatch(%#x, %#x) handled=%v clicks=%d, want handled=%v clicks=%d",
					tc.msg, tc.lParam, got, clicks, tc.wantHandled, tc.wantDblClk)
			}
		})
	}
	if !dispatchTrayMessage(wmSystrayMessage, wmLButtonDblClk, nil) {
		t.Fatal("nil double-click callback should still consume WM_LBUTTONDBLCLK")
	}
}

type forwardedMessage struct {
	msg    uint32
	lParam uintptr
}

var (
	testWndMu       sync.Mutex
	testWndPrev     uintptr
	testWndMessages []forwardedMessage
	testWndOnce     sync.Once
	testWndProcPtr  uintptr
)

func testOrigWndProc(hwnd windows.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	testWndMu.Lock()
	if msg == wmSystrayMessage || msg == wmCommand || msg == wmNCDestroy {
		testWndMessages = append(testWndMessages, forwardedMessage{msg: msg, lParam: lParam})
	}
	prev := testWndPrev
	testWndMu.Unlock()
	ret, _, _ := callWindowProcW.Call(prev, uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

func newMessageOnlyWindow(t *testing.T) windows.Handle {
	t.Helper()
	className, err := windows.UTF16PtrFromString("STATIC")
	if err != nil {
		t.Fatal(err)
	}
	const hwndMessage = ^uintptr(2) // HWND_MESSAGE (-3)
	createWindowExW := clickUser32.NewProc("CreateWindowExW")
	hwnd, _, err := createWindowExW.Call(
		0, uintptr(unsafe.Pointer(className)), 0, 0,
		0, 0, 0, 0, hwndMessage, 0, 0, 0,
	)
	if hwnd == 0 {
		t.Fatalf("CreateWindowExW: %v", err)
	}
	destroyWindow := clickUser32.NewProc("DestroyWindow")
	t.Cleanup(func() { destroyWindow.Call(hwnd) })
	return windows.Handle(hwnd)
}

func TestSubclassWindowDispatchesTrayMessages(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hwnd := newMessageOnlyWindow(t)
	testWndOnce.Do(func() { testWndProcPtr = windows.NewCallback(testOrigWndProc) })
	basePrev, err := setWindowProc(hwnd, testWndProcPtr)
	if err != nil {
		t.Fatal(err)
	}
	testWndMu.Lock()
	testWndPrev = basePrev
	testWndMessages = nil
	testWndMu.Unlock()

	dblClicks := 0
	uninstall, err := attachTrayHook(hwnd, func() { dblClicks++ })
	if err != nil {
		t.Fatal(err)
	}
	defer uninstall()

	sendMessageW := clickUser32.NewProc("SendMessageW")
	send := func(msg uint32, wParam, lParam uintptr) {
		sendMessageW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	}

	// Full Windows double-click sequence followed by right-click for the menu.
	send(wmSystrayMessage, 100, wmLButtonDown)
	send(wmSystrayMessage, 100, wmLButtonUp)
	send(wmSystrayMessage, 100, wmLButtonDblClk)
	send(wmSystrayMessage, 100, wmLButtonUp)
	send(wmSystrayMessage, 100, wmRButtonDown)
	send(wmSystrayMessage, 100, wmRButtonUp)

	if dblClicks != 1 {
		t.Fatalf("double-click count = %d, want 1", dblClicks)
	}
	testWndMu.Lock()
	got := append([]forwardedMessage(nil), testWndMessages...)
	testWndMessages = nil
	testWndMu.Unlock()
	want := []forwardedMessage{
		{wmSystrayMessage, wmLButtonDown},
		{wmSystrayMessage, wmRButtonDown},
		{wmSystrayMessage, wmRButtonUp},
	}
	if len(got) != len(want) {
		t.Fatalf("forwarded = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("forwarded[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}

	// Restoring the original WndProc lets WM_LBUTTONUP reach it again.
	uninstall()
	send(wmSystrayMessage, 100, wmLButtonUp)
	testWndMu.Lock()
	afterUninstall := append([]forwardedMessage(nil), testWndMessages...)
	testWndMu.Unlock()
	if len(afterUninstall) != 1 || afterUninstall[0] != (forwardedMessage{wmSystrayMessage, wmLButtonUp}) {
		t.Fatalf("after uninstall forwarded = %+v", afterUninstall)
	}
	if activeTrayHook.Load() != nil {
		t.Fatal("activeTrayHook not cleared after uninstall")
	}
}
