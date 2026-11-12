package integrate

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// os.Root.Rename replaces existing targets; creation and rollback must not.
func renameIntegrationExclusive(root *os.Root, from, to string) error {
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()
	err = unix.Renameat2(int(dir.Fd()), from, int(dir.Fd()), to, unix.RENAME_NOREPLACE)
	if errors.Is(err, unix.EINVAL) || errors.Is(err, unix.EOPNOTSUPP) || errors.Is(err, unix.ENOSYS) {
		return fmt.Errorf("this filesystem does not support safe no-replace renames (%s); on WSL use the Linux filesystem or the Windows version of Obituary: %w", root.Name(), err)
	}
	return err
}
