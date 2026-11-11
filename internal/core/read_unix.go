//go:build linux || darwin

package core

import "github.com/woodfishhhh/obituary/internal/core/internal/fileio"

func readShared(path string) ([]byte, error) { return fileio.ReadShared(path) }
