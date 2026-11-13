//go:build linux || darwin

package winfs

import (
	"fmt"
	"os"
	"path/filepath"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
)

func (fs FS) MovePinned(path string, want corefs.Identity, to string) error {
	parent, err := fs.OpenRestoreParent(filepath.Dir(to))
	if err != nil {
		return err
	}
	defer parent.Close()
	_, err = fs.moveInto(path, want, parent.(*restoreParent), filepath.Base(to), false)
	return err
}

func (fs FS) MovePinnedInto(path string, want corefs.Identity, parent corefs.RestoreParent, name string) (string, error) {
	p, ok := parent.(*restoreParent)
	if !ok || !singleName(name) {
		return "", fmt.Errorf("invalid restore parent or filename: %w", corefs.ErrUnsafe)
	}
	return fs.moveInto(path, want, p, name, true)
}

func (FS) Recycle(string, corefs.Identity) error {
	return fmt.Errorf("this platform supports only the graveyard; set mode = \"graveyard\" to bury files: %w", corefs.ErrUnsupported)
}

func (FS) MakeGrave(dir string) error {
	if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
		return err
	}
	return os.Mkdir(dir, 0o700)
}
