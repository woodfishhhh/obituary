package pathcheck

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func IsLink(info fs.FileInfo) bool {
	return info.Mode()&os.ModeSymlink != 0 || info.Mode()&os.ModeIrregular != 0
}

func samePath(a, b string) bool { return strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) }

// IsInside reports whether child is strictly below parent.
func IsInside(child, parent string) bool {
	parent = strings.TrimRight(filepath.Clean(parent), `\/`)
	child = filepath.Clean(child)
	if len(child) <= len(parent)+1 {
		return false
	}
	sep := child[len(parent)]
	return strings.EqualFold(child[:len(parent)], parent) && (sep == '\\' || sep == '/')
}

func SamePath(a, b string) bool { return samePath(a, b) }
