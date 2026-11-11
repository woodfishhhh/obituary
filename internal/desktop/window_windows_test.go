package desktop

import (
	"golang.org/x/sys/windows"
	"os"
	"runtime"
	"testing"
	"unsafe"
)

func TestTaskbarStylePreservesOtherBits(t *testing.T) {
	const other = 0x100 | 0x80000
	hidden := taskbarStyle(other|0x40000, false)
	if hidden != other|0x80 {
		t.Fatalf("hidden: %x", hidden)
	}
	shown := taskbarStyle(hidden, true)
	if shown != other|0x40000 {
		t.Fatalf("shown: %x", shown)
	}
}

type windowClass struct {
	Size, Style                        uint32
	Proc                               uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background windows.Handle
	Menu, Class                        *uint16
	SmallIcon                          windows.Handle
}

func TestNativeTaskbarSwitch(t *testing.T) {
	if os.Getenv("OBIT_TEST_DESKTOP") != "1" {
		t.Skip("set OBIT_TEST_DESKTOP=1 for isolated native window tests")
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	name, _ := windows.UTF16PtrFromString(WindowClass)
	proc := windows.NewCallback(func(hwnd uintptr, msg uint32, w, l uintptr) uintptr {
		result, _, _ := user32.NewProc("DefWindowProcW").Call(hwnd, uintptr(msg), w, l)
		return result
	})
	class := windowClass{Proc: proc, Class: name}
	class.Size = uint32(unsafe.Sizeof(class))
	atom, _, err := user32.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&class)))
	if atom == 0 {
		t.Fatal(err)
	}
	defer user32.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(name)), 0)
	hwnd, _, err := user32.NewProc("CreateWindowExW").Call(0x40000, uintptr(unsafe.Pointer(name)), 0, 0x00cf0000, 0, 0, 300, 200, 0, 0, 0, 0)
	if hwnd == 0 {
		t.Fatal(err)
	}
	defer user32.NewProc("DestroyWindow").Call(hwnd)
	for _, enabled := range []bool{false, true, false} {
		if err := SetTaskbar(enabled); err != nil {
			t.Fatal(err)
		}
		index := int32(-20)
		style, _, _ := user32.NewProc("GetWindowLongPtrW").Call(hwnd, uintptr(index))
		if (style&0x40000 != 0) != enabled || (style&0x80 == 0) != enabled {
			t.Fatalf("enabled=%t style=%x", enabled, style)
		}
	}
}
