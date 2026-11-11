package hook

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

var protectedDiscoveryRoots = systemDiscoveryRoots

func skipDiscoveryDenied(err error) bool {
	var pathErr *os.PathError
	return errors.As(err, &pathErr) && errors.Is(pathErr.Err, os.ErrPermission) &&
		inProtectedDiscoveryRoot(pathErr.Path, protectedDiscoveryRoots())
}

func inProtectedDiscoveryRoot(path string, roots []string) bool {
	path, ok := discoveryComparisonPath(path)
	if !ok {
		return false
	}
	for _, root := range roots {
		root, ok := discoveryComparisonPath(root)
		if ok && (path == root || strings.HasPrefix(path, root+string(filepath.Separator))) {
			return true
		}
	}
	return false
}

func discoveryComparisonPath(path string) (string, bool) {
	// Denied paths cannot be resolved by opening them. Compare normalized,
	// absolute spellings and retain errors when the location is uncertain.
	if !filepath.IsAbs(path) {
		return "", false
	}
	path, err := pathutil.Abs(path)
	if err != nil {
		return "", false
	}
	return strings.ToLower(filepath.Clean(path)), true
}
