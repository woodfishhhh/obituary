//go:build linux || darwin

package core

import (
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

func LockFile(path string, exclusive bool) (func(), error) {
	return lifecycle.LockFile(path, exclusive)
}

func TryLockExclusive(path string) (func(), bool, error) {
	return lifecycle.TryLockExclusive(path)
}
