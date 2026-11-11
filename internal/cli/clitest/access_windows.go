package clitest

import (
	"testing"

	"github.com/woodfishhhh/obituary/internal/testenv"
)

// DenyAccess changes only the named disposable fixture, never its children.
// Keep WRITE_DAC available so cleanup can restore the original permissions.
func DenyAccess(t *testing.T, path string, mask uint32) {
	t.Helper()
	testenv.DenyAccess(t, path, mask)
}
