package lifecycle

import (
	"github.com/woodfishhhh/obituary/internal/core/internal/fileio"
)

func readShared(path string) ([]byte, error) { return fileio.ReadShared(path) }
