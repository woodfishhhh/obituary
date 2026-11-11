//go:build linux || darwin

package core

import "os"

func readShared(path string) ([]byte, error) { return os.ReadFile(path) }
