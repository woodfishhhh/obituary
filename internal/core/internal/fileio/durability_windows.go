package fileio

import "os"

// Windows durability is the existing Sync on the renamed file handle.
func syncParentDirectory(*os.Root) error { return nil }
