package pathcheck

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

func IsLink(info fs.FileInfo) bool {
	return info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0
}

func comparisonPath(path string) string { return pathutil.ComparisonPath(filepath.Clean(path)) }

func samePath(a, b string) bool { return strings.EqualFold(comparisonPath(a), comparisonPath(b)) }

// IsInside reports whether child is strictly below parent.
func IsInside(child, parent string) bool {
	parent = strings.TrimRight(comparisonPath(parent), `\/`)
	child = comparisonPath(child)
	if len(child) <= len(parent)+1 {
		return false
	}
	sep := child[len(parent)]
	return strings.EqualFold(child[:len(parent)], parent) && (sep == '\\' || sep == '/')
}

func SamePath(a, b string) bool { return samePath(a, b) }
