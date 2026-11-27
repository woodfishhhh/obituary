package sweep

import (
	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
)

type lockedBurialFS struct {
	corefs.FS
	lock func()
}

func (fs lockedBurialFS) MovePinned(path string, want corefs.Identity, to string) error {
	if err := fs.FS.MovePinned(path, want, to); err != nil {
		return err
	}
	fs.lock()
	return nil
}
