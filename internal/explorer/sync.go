package explorer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/pathutil"
	"github.com/woodfishhhh/obituary/internal/project"
)

// Options selects where display-only dates are enabled. Folders contains the
// current Downloads and Desktop known-folder paths, supplied by the caller.
type Options struct {
	Enabled, InGit, Everywhere bool
	Folders                    []string
}

type directory struct {
	Path      string        `json:"path"`
	Identity  core.Identity `json:"identity"`
	Candidate bool          `json:"-"` // No write has been attempted in this sync.
}

type projection struct {
	dates map[string]map[string]time.Time
	dirs  map[string]directory
	auth  map[string]authorization
}

var findProject = project.FindContext

// Sync reconciles the current ledger with directories previously labelled by
// this home. The shared lock serializes CLI and daemon projections; it never
// holds store.lock while writing Explorer files or asking Git.
func Sync(svc *core.Service, options Options) error {
	if err := os.MkdirAll(svc.Store.Home, 0o700); err != nil {
		return err
	}
	release, ok, err := core.TryLockExclusive(filepath.Join(svc.Store.Home, "explorer.lock"))
	if err != nil {
		return err
	}
	if !ok {
		return errors.New("explorer sync busy; next tick will reconcile")
	}
	defer release()
	index := filepath.Join(svc.Store.Home, "explorer.json")
	previous, err := loadDirectories(index)
	if err != nil {
		return err
	}
	state, err := svc.Store.ReadSnapshot()
	if err != nil {
		return err
	}
	wanted, err := desired(svc, state.Items, options)
	if err != nil {
		return err
	} // retain all old labels when a location is uncertain
	for _, old := range previous {
		wanted.dirs[directoryKey(old.Identity)] = old
	}
	return syncDirectories(svc, index, wanted)
}

func syncDirectories(svc *core.Service, index string, wanted projection) error {
	var failures []error
	for id, dir := range wanted.dirs {
		path, err := svc.FS.Locate(dir.Identity)
		if errors.Is(err, core.ErrNotFound) {
			delete(wanted.dirs, id)
			continue
		}
		if err == nil {
			auth := wanted.auth[path]
			auth.Identity = dir.Identity
			err = reconcile(path, auth, wanted.dates[path], func() error {
				// Prepare has succeeded. Persist the cleanup target before Apply,
				// whose failure may still mean publication needs confirmation.
				dir.Candidate = false
				wanted.dirs[id] = dir
				return saveDirectories(index, wanted.dirs)
			})
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", dir.Path, err))
			continue
		}
		if len(wanted.dates[path]) == 0 {
			delete(wanted.dirs, id)
		}
	}
	return errors.Join(append(failures, saveDirectories(index, wanted.dirs))...)
}

func desired(svc *core.Service, items []core.Item, options Options) (projection, error) {
	wanted := projection{make(map[string]map[string]time.Time), make(map[string]directory), make(map[string]authorization)}
	if !options.Enabled {
		return wanted, nil
	}
	folders, err := scopeFolders(svc.FS, options.Folders)
	if err != nil {
		return wanted, err
	}
	for _, item := range items {
		if item.Identity == nil {
			continue
		}
		path, err := livePath(svc, *item.Identity)
		if err != nil {
			return wanted, err
		}
		if path == "" || pathutil.IsExplorerMetadata(filepath.Base(path)) {
			continue
		}
		if err := wanted.add(svc, path, item, options, folders); err != nil {
			return wanted, err
		}
	}
	return wanted, nil
}

func scopeFolders(fs core.FS, folders []string) ([]string, error) {
	roots := make([]string, 0, len(folders))
	for _, folder := range folders {
		path, err := pathutil.Abs(folder)
		if err != nil {
			return nil, err
		}
		// Locate returns resolved long names, so compare known folders in the
		// same spelling, including redirected folders reached through a junction.
		path, err = fs.Resolve(path)
		if err != nil {
			return nil, err
		}
		roots = append(roots, path)
	}
	return roots, nil
}

func inFolders(fs core.FS, dir string, folders []string) (bool, error) {
	for {
		for _, folder := range folders {
			if same, err := fs.SameName(dir, folder); err != nil || same {
				return same, err
			}
		}
		parent := pathutil.Parent(dir)
		if parent == dir {
			return false, nil
		}
		dir = parent
	}
}

func directoryKey(id core.Identity) string { return fmt.Sprintf("%d:%s", id.Serial, id.FileID) }

func livePath(svc *core.Service, id core.Identity) (string, error) {
	path, err := svc.FS.Locate(id)
	if errors.Is(err, core.ErrNotFound) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	// A successful move can precede its ledger commit. Reuse the existing
	// protected-location rules so a stale Item never decorates a grave.
	err = svc.Guard.CheckContext(svc.Store.Context(), path, true)
	var refused *core.RefusedError
	if errors.As(err, &refused) {
		return "", nil
	}
	return path, err
}

func reconcile(path string, auth authorization, dates map[string]time.Time, beforeApply func() error) error {
	p, err := Prepare(path, auth, dates)
	if err != nil {
		return err
	}
	if len(dates) > 0 && auth.Project.Root != "" {
		if err := auth.Project.ExcludeExplorerLabels(); err != nil {
			return err
		}
	}
	if err := beforeApply(); err != nil {
		return err
	}
	return p.Apply()
}

func loadDirectories(path string) (map[string]directory, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var dirs map[string]directory
	err = json.Unmarshal(data, &dirs)
	return dirs, err
}

func saveDirectories(path string, dirs map[string]directory) error {
	targets := make(map[string]directory)
	for id, dir := range dirs {
		if !dir.Candidate {
			targets[id] = dir
		}
	}
	data, err := json.Marshal(targets)
	if err != nil {
		return err
	}
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer root.Close()
	name := filepath.Base(path) + ".tmp"
	if err := root.WriteFile(name, data, 0o600); err != nil {
		return err
	}
	return root.Rename(name, filepath.Base(path))
}
