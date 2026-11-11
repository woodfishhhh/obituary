package rmdir

import (
	"github.com/woodfishhhh/obituary/internal/core"
)

type concurrentBurialFS struct {
	core.FS
	before func(string)
}

func (f concurrentBurialFS) MovePinned(path string, id core.Identity, to string) error {
	f.before(path)
	return f.FS.MovePinned(path, id, to)
}

func (f concurrentBurialFS) MovePinnedInto(path string, id core.Identity, parent core.RestoreParent, name string) (string, error) {
	f.before(path)
	return f.FS.MovePinnedInto(path, id, parent, name)
}
