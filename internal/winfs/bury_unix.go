//go:build linux || darwin

package winfs

import (
	"fmt"
	"os"
	"path/filepath"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
)

// renameat2(RENAME_NOREPLACE) pins directory descriptors, not the checked
// source inode. Neither os.Root.Rename nor x/sys/unix can meet MovePinned's
// same-source-handle promise. Do not substitute a check-then-rename window.
func (FS) MovePinned(string, corefs.Identity, string) error { return unsupportedMove() }
func (FS) MovePinnedInto(string, corefs.Identity, corefs.RestoreParent, string) (string, error) {
	return "", unsupportedMove()
}
func (FS) Recycle(string, corefs.Identity) error { return unsupportedMove() }

func unsupportedMove() error {
	return fmt.Errorf("automatic burial, resurrection and recycling are unavailable on this platform; source files and tracked entries are preserved: %w", corefs.ErrUnsupported)
}

func (FS) MakeGrave(dir string) error {
	if err := os.MkdirAll(filepath.Dir(dir), 0o700); err != nil {
		return err
	}
	return os.Mkdir(dir, 0o700)
}
