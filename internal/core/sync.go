package core

import (
	"os"

	"github.com/woodfishhhh/obituary/internal/core/internal/fileio"
)

func syncFile(file *os.File) error { return fileio.SyncFile(file) }

func SetFileSyncForTesting(fn func(*os.File) error) func() { return fileio.SetFileSyncForTesting(fn) }
