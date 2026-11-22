package docker

import (
	"errors"
	"path/filepath"
	"strings"
)

// volumeReference checks Docker-visible volume mounts and bind mounts whose
// source overlaps the volume Mountpoint. It intentionally does not inspect the
// daemon's private storage directory.
func volumeReference(c Container, name, point string) (bool, error) {
	if name == "" || point == "" {
		return false, errors.New("volume reference requires a name and mountpoint")
	}
	for _, volume := range c.Volumes {
		if volume == name {
			return true, nil
		}
	}
	point = filepath.Clean(point)
	for _, source := range c.Binds {
		if pathsOverlap(source, point) {
			return true, nil
		}
	}
	return false, nil
}

func pathsOverlap(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if a == b {
		return true
	}
	return pathContains(a, b) || pathContains(b, a)
}

func pathContains(parent, child string) bool {
	if parent == string(filepath.Separator) {
		return strings.HasPrefix(child, parent)
	}
	return strings.HasPrefix(child, parent+string(filepath.Separator))
}
