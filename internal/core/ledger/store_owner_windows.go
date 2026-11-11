package ledger

import (
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

// os.SameFile cannot persist an identity. Use the same FILE_ID_INFO contract
// as winfs (which depends on core and cannot be imported by the store).
func dataDirectoryID(root *os.Root) (string, error) {
	f, err := root.Open(".")
	if err != nil {
		return "", err
	}
	defer f.Close()
	var id struct {
		Volume uint64
		File   [16]byte
	}
	err = windows.GetFileInformationByHandleEx(windows.Handle(f.Fd()), windows.FileIdInfo, (*byte)(unsafe.Pointer(&id)), uint32(unsafe.Sizeof(id)))
	return fmt.Sprintf("%x:%x", id.Volume, id.File), err
}
