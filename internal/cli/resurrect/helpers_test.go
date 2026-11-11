package resurrect

import (
	"os"

	"github.com/woodfishhhh/obituary/internal/core"
)

type failBurialCommitFS struct {
	core.FS
	blocked string
}

func (f *failBurialCommitFS) MovePinned(path string, want core.Identity, to string) error {
	if err := f.FS.MovePinned(path, want, to); err != nil {
		return err
	}
	if f.blocked != "" {
		blocked := f.blocked
		f.blocked = ""
		return os.Mkdir(blocked, 0o700) // only the following store commit fails
	}
	return nil
}

func (f *failBurialCommitFS) MovePinnedInto(path string, want core.Identity, parent core.RestoreParent, name string) (string, error) {
	actual, err := f.FS.MovePinnedInto(path, want, parent, name)
	if err == nil && f.blocked != "" {
		blocked := f.blocked
		f.blocked = ""
		err = os.Mkdir(blocked, 0o700)
	}
	return actual, err
}
