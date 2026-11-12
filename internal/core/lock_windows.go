package core

import (
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

func LockFile(path string, exclusive bool) (func(), error) {
	return lifecycle.LockFile(path, exclusive)
}

func TryLockExclusive(path string) (release func(), ok bool, err error) {
	return lifecycle.TryLockExclusive(path)
}
