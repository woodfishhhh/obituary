//go:build windows

// A separate process holds the second installation file without sharing it.
package main

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/windows"
)

func main() {
	path, err := windows.UTF16PtrFromString(os.Args[1])
	if err != nil {
		panic(err)
	}
	var share uint32
	if len(os.Args) > 2 && os.Args[2] == "deny-delete" {
		share = windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE
	}
	h, err := windows.CreateFile(path, windows.GENERIC_READ, share, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		panic(err)
	}
	defer windows.CloseHandle(h)
	fmt.Println("locked")
	_, _ = io.Copy(io.Discard, os.Stdin)
}
