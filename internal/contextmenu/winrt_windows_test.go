package contextmenu

import (
	"testing"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestWinRTHStringPreservesUTF16(t *testing.T) {
	getBuffer := combase.NewProc("WindowsGetStringRawBuffer")
	compare := windows.NewLazySystemDLL("kernel32.dll").NewProc("CompareStringOrdinal")
	for _, value := range []string{"", "中文", "😀.txt", "trailing 😀"} {
		if err := withHString(value, func(handle uintptr) error {
			var size uint32
			buffer, _, _ := getBuffer.Call(handle, uintptr(unsafe.Pointer(&size)))
			if size != uint32(len(utf16.Encode([]rune(value)))) {
				t.Fatalf("length: %d %q", size, value)
			}
			expected, err := windows.UTF16FromString(value)
			if err != nil {
				return err
			}
			if equal, _, err := compare.Call(buffer, uintptr(size), uintptr(unsafe.Pointer(&expected[0])), uintptr(size), 0); equal != 2 {
				t.Fatalf("HSTRING differs from %q: %v", value, err)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := winrtError(1, "S_FALSE"); err != nil {
		t.Fatal(err)
	}
	if err := winrtError(0x80004005, "failure"); err == nil {
		t.Fatal("swallowed HRESULT")
	}
}
