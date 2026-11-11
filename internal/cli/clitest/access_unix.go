//go:build linux || darwin

package clitest

import (
	"testing"
)

func DenyAccess(t *testing.T, path string, mask uint32) { t.Helper(); t.Skip("Windows ACL fixture") }
