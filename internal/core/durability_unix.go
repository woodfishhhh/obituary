//go:build linux || darwin

package core

import "os"

// Linux fsync(2): flushing the file alone does not persist its renamed directory
// entry. Open through the same Root used for the replacement, not its old path.
func syncParentDirectory(root *os.Root) error {
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	defer dir.Close()
	return syncFile(dir)
}
