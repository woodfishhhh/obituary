package pathcheck

import (
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// Exact names win. Only ASCII is folded for a missing exact match: Unicode
// simple folding aliases names (notably k and Kelvin sign) that NTFS separates.
// Conservative protection checks deliberately continue to use samePath.
func MatchingPaths(paths []string, path string) ([]int, bool) {
	if normalized, err := pathutil.Abs(path); err == nil {
		path = normalized
	} else {
		return nil, false
	}
	path = filepath.Clean(path)
	var exact, folded []int
	for i, candidate := range paths {
		if candidate == "" {
			continue
		}
		if normalized, err := pathutil.Abs(candidate); err == nil {
			candidate = normalized
		} else {
			continue
		}
		candidate = filepath.Clean(candidate)
		if candidate == path {
			exact = append(exact, i)
		}
		if asciiPath(candidate) == asciiPath(path) {
			folded = append(folded, i)
		}
	}
	if len(exact) > 0 {
		return exact, true
	}
	return folded, false
}

func asciiPath(path string) string {
	b := []byte(path)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}
