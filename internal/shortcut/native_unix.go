//go:build !windows

package shortcut

import "errors"

type Native struct{}

func (Native) Encode(string) ([]byte, error) {
	return nil, errors.New("shortcuts are only available on Windows")
}
func (Native) Target([]byte) (string, error) {
	return "", errors.New("shortcuts are only available on Windows")
}
