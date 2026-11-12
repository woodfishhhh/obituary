package integrate

import (
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
	return unix.RenameatxNp(int(dir.Fd()), from, int(dir.Fd()), to, unix.RENAME_EXCL)
}
