//go:build linux || darwin

package lifecycle

import (
	"os"
)

func isReparse(info os.FileInfo) bool { return info.Mode()&os.ModeSymlink != 0 }
