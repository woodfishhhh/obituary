package rm

import (
	"errors"

	"github.com/woodfishhhh/obituary/internal/core"
)

type removeRecycleFS struct {
	core.FS
	destination string
	fail        bool
}

func (fs removeRecycleFS) Recycle(path string, id core.Identity) error {
	if fs.fail {
		return errors.New("recycle unavailable")
	}
	return fs.MovePinned(path, id, fs.destination)
}
