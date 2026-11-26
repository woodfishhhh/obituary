package lifecycle

import (
	"golang.org/x/sys/windows"
	"os"
	"syscall"
)

// ModeSymlink/ModeIrregular do not cover all Windows reparse tags.
func isReparse(info os.FileInfo) bool {
	return info.Sys().(*syscall.Win32FileAttributeData).FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
}

const restoreLinkLoop = windows.ERROR_CANT_RESOLVE_FILENAME
