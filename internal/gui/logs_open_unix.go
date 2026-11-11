//go:build linux || darwin

package gui

import (
	"os"
)

func openLog(path string) (*os.File, error) { return os.Open(path) }
