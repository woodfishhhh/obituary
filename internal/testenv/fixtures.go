package testenv

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func RequireDistinctUnicodeNames(t *testing.T) {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "K"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "K"), 0700); os.IsExist(err) {
		t.Skip("filesystem normalizes K and the Kelvin sign to the same directory name")
	} else if err != nil {
		t.Fatal(err)
	}
}

var fixtureDrivePath = regexp.MustCompile(`\b[A-Za-z]:[\\/]+(?:[^"\r\n]*[^"\\\r\n])?`)

// NativeFixture gives fixed Windows paths in test payloads and their expected
// values a POSIX spelling. It is only for fixture text, never user input.
func NativeFixture(text string) string {
	if runtime.GOOS == "windows" {
		return text
	}
	return fixtureDrivePath.ReplaceAllStringFunc(text, func(path string) string {
		path = strings.ReplaceAll(path[2:], `\\`, `/`)
		return strings.ReplaceAll(path, `\`, `/`)
	})
}

func WindowsOnly(t *testing.T, reason string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		t.Skip(reason)
	}
}

// ForeignFixture keeps negative locality cases foreign on either platform.
func ForeignFixture(path string) string {
	if runtime.GOOS == "windows" {
		return path
	}
	return "ssh://fixture" + path
}

func PythonCommand() string {
	if runtime.GOOS == "windows" {
		return "python"
	}
	return "python3"
}
