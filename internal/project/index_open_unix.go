//go:build linux || darwin

package project

import (
	"os"
)

func openIndex(path string) (*os.File, error) { return os.Open(path) }
