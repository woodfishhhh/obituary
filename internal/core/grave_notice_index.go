package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// The disposable path index is only a negative filter for hook notices. A hit
// always requires a fresh shared-lock ledger read; stale entries confer no
// authority. It deliberately uses exact names, avoiding case-sensitive aliases.
type graveNoticePath struct {
	Path      string `json:"path"`
	Directory bool   `json:"directory,omitempty"`
}

type graveNoticeIndex struct {
	Generation string            `json:"generation"`
	Paths      []graveNoticePath `json:"paths"`
}

func (s Store) writeGraveNoticeIndex(st State) {
	// Most registration-only homes have no graves. Keep their writes free of
	// advisory index work until a burial first needs it.
	if len(st.Burials) == 0 {
		if _, err := os.Stat(filepath.Join(s.Home, "grave-notice-paths.json")); os.IsNotExist(err) {
			return
		}
	}
	unlock, ok, err := TryLockExclusive(filepath.Join(s.Home, "grave-notice-paths.lock"))
	if err != nil || !ok {
		return
	}
	defer unlock()
	paths := make([]graveNoticePath, 0, len(st.Burials))
	for _, b := range st.Burials {
		paths = append(paths, graveNoticePath{filepath.Clean(b.Path), b.IsDir})
	}
	index := graveNoticeIndex{Paths: paths}
	if st.Graveyard != nil {
		index.Generation = st.Graveyard.Generation
	}
	data, err := json.Marshal(index)
	if err != nil {
		return
	}
	name := filepath.Join(s.Home, "grave-notice-paths.json")
	if old, err := readShared(name); err == nil && string(old) == string(data) {
		return
	}
	// Best effort: failure only suppresses an advisory. The ledger commit and its
	// original error are independent of this rebuildable, non-durable cache.
	root, err := os.OpenRoot(s.Home)
	if err != nil {
		return
	}
	defer root.Close()
	if err := root.WriteFile("grave-notice-paths.json.tmp", data, 0600); err == nil {
		_ = root.Rename("grave-notice-paths.json.tmp", "grave-notice-paths.json")
	}
}

// MightHaveBurial avoids reading the ledger for unrelated absent paths. Legacy
// homes acquire this cache on their next ordinary ledger write or session start.
func (s Store) MightHaveBurial(path string) bool {
	data, err := readShared(filepath.Join(s.Home, "grave-notice-paths.json"))
	if err != nil {
		return false
	}
	var index graveNoticeIndex
	if json.Unmarshal(data, &index) != nil {
		return false
	}
	return slices.ContainsFunc(index.Paths, func(p graveNoticePath) bool {
		return BurialPathMatches(p.Path, p.Directory, path)
	})
}

// BurialPathMatches is a conservative lexical match, including a separator at
// directory boundaries. Exact case avoids treating a missing case-sensitive
// neighbour as the buried object. Callers normalize paths before matching.
func BurialPathMatches(buried string, directory bool, path string) bool {
	buried, path = filepath.Clean(buried), filepath.Clean(path)
	return buried == path || directory && strings.HasPrefix(path, buried+string(filepath.Separator))
}

// RefreshGraveNoticeIndex initializes the advisory cache from a locked snapshot.
// It does not change the ledger or upgrade its version.
func (s Store) RefreshGraveNoticeIndex() {
	_ = s.withLock("store.lock", false, func() error {
		st, _, err := s.loadHot()
		if err != nil {
			return err
		}
		var index graveNoticeIndex
		data, readErr := readShared(filepath.Join(s.Home, "grave-notice-paths.json"))
		if readErr == nil && json.Unmarshal(data, &index) == nil && st.Graveyard != nil && index.Generation == st.Graveyard.Generation {
			return nil
		}
		err = s.loadGraveyard(&st)
		if err == nil {
			s.writeGraveNoticeIndex(st)
		}
		return err
	})
}
