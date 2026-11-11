//go:build windows

package runkey

import "strings"

func RegistryForTest(root string) Registry {
	if !strings.HasPrefix(root, `Software\obituary-test-`) {
		panic("Run tests require a disposable registry root")
	}
	return native{root: root}
}
