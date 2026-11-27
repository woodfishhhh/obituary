package app

import (
	"errors"
	"os"
	"strings"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
)

type commitFailureFS struct {
	corefs.FS
	blocked string
}

func (f commitFailureFS) MovePinned(path string, want corefs.Identity, to string) error {
	if err := f.FS.MovePinned(path, want, to); err != nil {
		return err
	}
	return os.Mkdir(f.blocked, 0o700)
}

type unavailableRootFS struct {
	corefs.FS
	root  string
	scans int
}

func (f *unavailableRootFS) RealPath(path string) (string, error) {
	if strings.EqualFold(path, f.root) {
		f.scans++
		return "", errors.New("volume is offline")
	}
	return f.FS.RealPath(path)
}
