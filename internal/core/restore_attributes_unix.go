//go:build linux || darwin

package core

import "os"

func isReparse(info os.FileInfo) bool { return info.Mode()&os.ModeSymlink != 0 }
