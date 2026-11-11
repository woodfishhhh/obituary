package core

import (
	"errors"
	"io"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// Diagnostic readers must share deletion so an atomic replacement can finish
// while this snapshot is read.
func readShared(path string) ([]byte, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	var h windows.Handle
	for attempt := 0; ; attempt++ {
		h, err = windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
		if err == nil {
			break
		}
		transient := errors.Is(err, windows.ERROR_SHARING_VIOLATION) || errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_FILE_NOT_FOUND)
		if !transient || attempt == 4 {
			return nil, &os.PathError{Op: "open", Path: path, Err: err}
		}
		time.Sleep(time.Duration(attempt+1) * time.Millisecond)
	}
	f := os.NewFile(uintptr(h), path)
	defer f.Close()
	return io.ReadAll(f)
}
