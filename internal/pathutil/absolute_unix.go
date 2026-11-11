//go:build linux || darwin

package pathutil

import (
	"fmt"
	"path/filepath"
	"strings"
)

func Abs(path string) (string, error) {
	if err := WholeObject(path); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(path)
	if err == nil && RequiresDirectory(path) && abs != "/" {
		abs += "/"
	}
	return abs, err
}

func LongAbs(path string) (string, error) { return Abs(path) }

func RequiresDirectory(path string) bool {
	return strings.HasSuffix(path, "/") && filepath.Clean(path) != "/"
}

func WholeObject(path string) error {
	if strings.ContainsRune(path, 0) {
		return fmt.Errorf("path contains NUL")
	}
	return nil
}

func Unchanged(path string) error {
	if err := WholeObject(path); err != nil {
		return err
	}
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return fmt.Errorf("path %q is not an absolute clean name", path)
	}
	return nil
}
