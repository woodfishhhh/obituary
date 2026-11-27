package bury

import (
	"errors"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
)

type approvalFaultFS struct {
	corefs.FS
	beforeRealPath func(string)
	beforeIdentify func(string)
}

func (fs approvalFaultFS) RealPath(path string) (string, error) {
	if fs.beforeRealPath != nil {
		fs.beforeRealPath(path)
	}
	return fs.FS.RealPath(path)
}

func (fs approvalFaultFS) Identify(path string) (corefs.Identity, error) {
	if fs.beforeIdentify != nil {
		fs.beforeIdentify(path)
	}
	return fs.FS.Identify(path)
}

type failMoveFS struct{ corefs.FS }

func (f failMoveFS) MovePinned(string, corefs.Identity, string) error {
	return errors.New("sharing violation")
}

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
