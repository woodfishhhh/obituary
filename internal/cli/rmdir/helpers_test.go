package rmdir

import (
	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
)

type concurrentBurialFS struct {
	corefs.FS
	before func(string)
}

func (f concurrentBurialFS) MovePinned(path string, id corefs.Identity, to string) error {
	f.before(path)
	return f.FS.MovePinned(path, id, to)
}

func (f concurrentBurialFS) MovePinnedInto(path string, id corefs.Identity, parent corefs.RestoreParent, name string) (string, error) {
	f.before(path)
	return f.FS.MovePinnedInto(path, id, parent, name)
}
