//go:build linux || darwin

package fileio

import "os"

func readShared(path string) ([]byte, error) { return os.ReadFile(path) }
