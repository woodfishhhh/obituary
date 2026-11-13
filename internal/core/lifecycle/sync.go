package lifecycle

import (
	"os"

	"github.com/woodfishhhh/obituary/internal/core/internal/fileio"
)

func SetFileSyncForTesting(fn func(*os.File) error) func() { return fileio.SetFileSyncForTesting(fn) }
