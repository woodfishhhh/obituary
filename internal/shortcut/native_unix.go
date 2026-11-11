//go:build !windows

package shortcut

import "fmt"

type Native struct{}

func (Native) Encode(string) ([]byte, error) {
	return nil, fmt.Errorf("Windows shortcuts are unsupported on this platform")
}
func (Native) Target([]byte) (string, error) {
	return "", fmt.Errorf("Windows shortcuts are unsupported on this platform")
}
