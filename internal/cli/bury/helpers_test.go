package bury

import (
	"errors"

	"github.com/woodfishhhh/obituary/internal/core"
)

type approvalFaultFS struct {
	core.FS
	beforeRealPath func(string)
	beforeIdentify func(string)
}

func (fs approvalFaultFS) RealPath(path string) (string, error) {
	if fs.beforeRealPath != nil {
		fs.beforeRealPath(path)
	}
	return fs.FS.RealPath(path)
}

func (fs approvalFaultFS) Identify(path string) (core.Identity, error) {
	if fs.beforeIdentify != nil {
		fs.beforeIdentify(path)
	}
	return fs.FS.Identify(path)
}

type failMoveFS struct{ core.FS }

func (f failMoveFS) MovePinned(string, core.Identity, string) error {
	return errors.New("sharing violation")
}

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
