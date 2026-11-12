//go:build linux || darwin

package lifecycle

import (
	"os"

	"github.com/woodfishhhh/obituary/internal/core/internal/fileio"
)

func syncParentDirectory(root *os.Root) error { return fileio.SyncParentDirectory(root) }
