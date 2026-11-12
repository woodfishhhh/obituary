package lifecycle

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// FilterGraveyard selects original locations, including descendants. Require
// a path-component boundary so similarly prefixed siblings stay separate.
// Resolve 8.3 input names and reuse FindPath's exact-first ASCII
// comparison; Unicode folding and filepath.Rel's case rules are unsuitable.
func (s *Service) FilterGraveyard(view *GraveyardView, directory string) error {
	directory, err := s.referencePath(directory)
	if err != nil {
		return err
	}
	directory = filepath.Clean(directory)
	var burials []*Burial
	for i := range view.Burials {
		burials = append(burials, &view.Burials[i])
	}
	for _, p := range view.Pending {
		burials = append(burials, p.Burial)
	}
	for _, r := range view.Restored {
		burials = append(burials, r.Burial)
	}
	prefixes := make([]string, len(burials))
	for i, b := range burials {
		if b == nil {
			continue // no known original location to include
		}
		original := b.Path
		if b.Kind == KindContainer {
			if b.Project == "" {
				continue
			}
			original = b.Project
		}
		path, err := pathutil.Abs(original)
		if err != nil {
			return err
		}
		if !b.LongNames {
			path, err = s.referencePath(path)
			if err != nil {
				return err
			}
		}
		path = filepath.Clean(path)
		if len(path) < len(directory) {
			continue
		}
		if len(path) > len(directory) && !strings.HasSuffix(directory, `\`) && path[len(directory)] != filepath.Separator {
			continue
		}
		prefixes[i] = path[:len(directory)]
	}
	matches, exact := matchingPaths(prefixes, directory)
	keep := make(map[int]bool, len(matches))
	for _, i := range matches {
		if !exact && prefixes[i] != prefixes[matches[0]] {
			return fmt.Errorf("directory matches multiple spellings; use obit graveyard without a directory filter")
		}
		keep[i] = true
	}
	filtered := GraveyardView{Burials: []Burial{}, Pending: []PendingGrave{}}
	i := 0
	for _, b := range view.Burials {
		if keep[i] {
			filtered.Burials = append(filtered.Burials, b)
		}
		i++
	}
	for _, p := range view.Pending {
		if keep[i] {
			filtered.Pending = append(filtered.Pending, p)
		}
		i++
	}
	for _, r := range view.Restored {
		if keep[i] {
			filtered.Restored = append(filtered.Restored, r)
		}
		i++
	}
	*view = filtered
	return nil
}
