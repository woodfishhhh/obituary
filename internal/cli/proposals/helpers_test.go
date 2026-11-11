package proposals

import (
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
