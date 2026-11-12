package lifecycle

import (
	"os"
	"syscall"
)

// ModeSymlink/ModeIrregular do not cover all Windows reparse tags.
func isReparse(info os.FileInfo) bool {
	return info.Sys().(*syscall.Win32FileAttributeData).FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
}
