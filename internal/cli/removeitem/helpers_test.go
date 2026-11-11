//go:build windows

package removeitem

import (
	"testing"

	"github.com/spf13/cobra"
	"golang.org/x/sys/windows"
)

// Use the public command registration, including parameter parsing and errors.
func itemCommand() *cobra.Command { return NewCmd() }

func setItemAttrs(t *testing.T, path string, attrs uint32) {
	t.Helper()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetFileAttributes(p, attrs); err != nil {
		t.Fatal(err)
	}
}
