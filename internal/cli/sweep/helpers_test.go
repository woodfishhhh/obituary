package sweep

import (
	"github.com/woodfishhhh/obituary/internal/core"
)

type lockedBurialFS struct {
	core.FS
	lock func()
}

func (fs lockedBurialFS) MovePinned(path string, want core.Identity, to string) error {
	if err := fs.FS.MovePinned(path, want, to); err != nil {
		return err
	}
	fs.lock()
	return nil
}
