package rm

import (
	"errors"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
)

type removeRecycleFS struct {
	corefs.FS
	destination string
	fail        bool
}

func (fs removeRecycleFS) Recycle(path string, id corefs.Identity) error {
	if fs.fail {
		return errors.New("recycle unavailable")
	}
	return fs.MovePinned(path, id, fs.destination)
}
