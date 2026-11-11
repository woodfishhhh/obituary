// Package pathutil provides volume-aware parent traversal for Windows paths.
package pathutil

import (
	"path/filepath"
	"strings"
)

// Parent stops at a drive/share root, including extended UNC.
func Parent(path string) string {
	path = filepath.Clean(path)
	root := VolumeRoot(path)
	if strings.TrimRight(path, `\/`) == strings.TrimRight(root, `\/`) {
		return path
	}
	return filepath.Dir(path)
}
