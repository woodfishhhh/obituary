//go:build linux || darwin

package core

import (
	"os"

	"github.com/woodfishhhh/obituary/internal/core/internal/fileio"
)

func syncParentDirectory(root *os.Root) error { return fileio.SyncParentDirectory(root) }
