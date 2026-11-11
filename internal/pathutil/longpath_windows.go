package pathutil

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// LongAbs expands existing 8.3 names for path comparisons, retaining missing
// descendants and lexical junction locations. filepath.EvalSymlinks also
// resolves junctions (Go's path/filepath/symlink_windows.go); comparisons that
// preserve the caller's location need GetLongPathNameW instead. Mutation
// boundaries still use FS.LongPath's handle-based identity checks.
func LongAbs(path string) (string, error) {
	abs, err := Abs(path)
	if err != nil {
		return "", err
	}
	expanded, err := longExistingPrefix(filepath.Clean(abs))
	if err != nil {
		return "", err
	}
	for prefix := expanded; ; prefix = filepath.Dir(prefix) {
		if err := Unchanged(prefix); err != nil {
			return "", err
		}
		if filepath.Dir(prefix) == prefix {
			break
		}
	}
	if RequiresDirectory(path) && !strings.HasSuffix(expanded, `\`) {
		expanded += `\`
	}
	return expanded, nil
}

func longExistingPrefix(path string) (string, error) {
	var suffix []string
	for {
		expanded, err := getLongPath(path)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				expanded = filepath.Join(expanded, suffix[i])
			}
			return expanded, nil
		}
		parent := filepath.Dir(path)
		if !errors.Is(err, windows.ERROR_FILE_NOT_FOUND) && !errors.Is(err, windows.ERROR_PATH_NOT_FOUND) || parent == path {
			return "", &os.PathError{Op: "GetLongPathName", Path: path, Err: err}
		}
		suffix = append(suffix, filepath.Base(path))
		path = parent
	}
}

func getLongPath(path string) (string, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	buf := make([]uint16, windows.MAX_PATH)
	for {
		n, err := windows.GetLongPathName(name, &buf[0], uint32(len(buf)))
		if err != nil {
			return "", err
		}
		if n < uint32(len(buf)) {
			return windows.UTF16ToString(buf[:n]), nil
		}
		buf = make([]uint16, n+1)
	}
}
