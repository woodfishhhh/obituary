//go:build linux || darwin

package testenv

import "testing"

func ShortPath(t *testing.T, path string) string {
	t.Helper()
	t.Skip("Windows short-name fixture")
	return ""
}
func DenyEnumeration(t *testing.T, path string) { t.Helper(); t.Skip("Windows ACL fixture") }
