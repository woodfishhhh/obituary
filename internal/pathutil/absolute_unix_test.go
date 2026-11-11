//go:build linux || darwin

package pathutil

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPOSIXNamesPreserveColonBackslashAndTrailingSlash(t *testing.T) {
	path := filepath.Join(t.TempDir(), `A:B\C`)
	got, err := Abs(path + "/")
	if err != nil || got != path+"/" || !RequiresDirectory(got) {
		t.Fatal(got, err)
	}
	if err := Unchanged(path); err != nil {
		t.Fatal(err)
	}
	if _, err := Abs("bad\x00name"); err == nil {
		t.Fatal("NUL accepted")
	}
	if strings.Contains(got, `C:\`) {
		t.Fatal("POSIX name translated into Windows path")
	}
}
