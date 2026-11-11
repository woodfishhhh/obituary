package project

import (
	"os/exec"
	"syscall"
	"unicode/utf16"

	"golang.org/x/sys/windows"
)

// caseInsensitive: NTFS names compare without case.
const caseInsensitive = true

func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
}

// Include Go's CreateProcess quoting, UTF-16 code units and a separator/NUL.
func commandArgSize(arg string) int {
	return len(utf16.Encode([]rune(syscall.EscapeArg(arg)))) + 1
}
