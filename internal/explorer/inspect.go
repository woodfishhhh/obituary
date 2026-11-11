package explorer

import (
	"path/filepath"
	"slices"
)

// CleanupTargets reads the existing index without taking a lock, creating a
// directory or applying labels. Indexed directories include healthy labels;
// their presence alone is not evidence of a failed cleanup.
func CleanupTargets(home string) ([]string, error) {
	dirs, err := loadDirectories(filepath.Join(home, "explorer.json"))
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		paths = append(paths, dir.Path)
	}
	slices.Sort(paths)
	return slices.Compact(paths), nil
}
