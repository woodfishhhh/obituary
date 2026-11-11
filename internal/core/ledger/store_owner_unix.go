//go:build linux || darwin

package ledger

import (
	"fmt"
	"os"
	"syscall"
)

func dataDirectoryID(root *os.Root) (string, error) {
	info, err := root.Stat(".")
	if err != nil {
		return "", err
	}
	st := info.Sys().(*syscall.Stat_t)
	return fmt.Sprintf("%x:%x", st.Dev, st.Ino), nil
}
