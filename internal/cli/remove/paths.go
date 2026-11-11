package remove

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Git/MSYS installations are the shell's / even on a non-system drive.
// Require a runtime file, or a Bash executable together with an etc directory;
// a directory merely named usr or a standalone Bash binary is not a root.
func isMSYSRoot(path string) (bool, error) {
	for _, marker := range []string{"msys-2.0.dll", "bash.exe"} {
		info, err := os.Stat(filepath.Join(path, "usr", "bin", marker))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, err
		}
		if !info.Mode().IsRegular() {
			continue
		}
		if marker == "msys-2.0.dll" {
			return true, nil
		}
		info, err = os.Stat(filepath.Join(path, "etc"))
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return info.IsDir(), nil
	}
	return false, nil
}

// Win32 collapses a/.. before looking up a. Check each raw prefix first so a
// missing directory, regular file or reparse point cannot select its sibling.
// Keep this rejection even with -f: it is an ambiguous operand, not an absent
// target. Never use Join/Clean/Abs before these Lstat checks.
func CheckParentTraversal(ref string) error {
	path := filepath.FromSlash(ref)
	volume := filepath.VolumeName(path)
	parts := strings.Split(path[len(volume):], string(filepath.Separator))
	lastParent := -1
	for i, part := range parts {
		if part == ".." {
			lastParent = i
		}
	}
	prefix := volume
	if strings.HasPrefix(path[len(volume):], string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	for _, part := range parts[:max(0, lastParent)] {
		if part == "" {
			continue
		}
		if prefix != "" && !strings.HasSuffix(prefix, string(filepath.Separator)) {
			prefix += string(filepath.Separator)
		}
		prefix += part
		info, err := os.Lstat(prefix)
		if err != nil || !info.IsDir() || info.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return errors.New("No such file or directory") //nolint:staticcheck // ST1005: rm diagnostics and their contract tests depend on this spelling.
		}
	}
	return nil
}

func EmptyDirectory(path string) error {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return err
	}
	defer f.Close()
	entries, err := f.ReadDir(1)
	if len(entries) != 0 {
		return errors.New("Directory not empty") //nolint:staticcheck // ST1005: rmdir diagnostics and their contract tests depend on this spelling.
	}
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}
