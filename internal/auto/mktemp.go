package auto

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/pathutil"
	"github.com/woodfishhhh/obituary/internal/project"
)

// Owner is core.Service.Owner, backed by Git.
func Owner(path string) (string, bool, error) {
	p, ok, err := project.Find(path)
	if err != nil {
		return "", false, err
	}
	if !ok {
		return "", false, nil
	}
	tracked, err := p.Tracked(path)
	return p.Root, tracked, err
}

// OwnerBatch discovers every path independently (including nested worktrees),
// then reuses project.TrackedBatchContext's UTF-16 command-line chunking.
func OwnerBatch(ctx context.Context, paths []string) []core.Ownership {
	return queryOwners(ctx, paths, nil, nil, nil)
}

func queryOwners(ctx context.Context, paths []string, observers map[string]preparedIndex, known map[string]discoveredOwner, names map[string]string) []core.Ownership {
	answers := make([]core.Ownership, len(paths))
	groups := ownerGroups(ctx, paths, known, answers)
	for root, indices := range groups {
		group, preparedNames := ownerGroupPaths(paths, indices, names)
		var tracked []bool
		var snapshot *project.IndexSnapshot
		var afterFind func()
		var err error
		if observers == nil {
			tracked, err = (project.Project{Root: root}).TrackedBatchContext(ctx, group)
		} else {
			prepared := prepareOwnerIndex(ctx, root, observers)
			afterFind = prepared.afterFind
			err = prepared.err
			if err != nil {
				// Metadata probing is optional. Older Git versions and virtual
				// config paths keep the original fresh-query behavior.
				return OwnerBatch(ctx, paths)
			}
			tracked, snapshot, err = queryIndex(ctx, prepared.observer, group, preparedNames)
			if errors.Is(err, project.ErrIndexSnapshotUnavailable) {
				return OwnerBatch(ctx, paths)
			}
		}
		checkedAt := time.Now()
		for j, i := range indices {
			answers[i].Tracked, answers[i].Err = true, err
			if err == nil {
				answers[i].Tracked = tracked[j]
			}
			answers[i].CheckedAt = checkedAt
			if snapshot != nil {
				path := paths[i]
				answers[i].Current = func(ctx context.Context) (bool, time.Time, error) {
					started := time.Now()
					current, found, err := project.FindContext(ctx, path)
					if afterFind != nil {
						afterFind()
					}
					known[path] = discoveredOwner{project: current, found: found, err: err}
					if err != nil || !found || current.Root != root {
						return false, time.Time{}, err
					}
					valid, err := snapshot.Current(ctx)
					return valid, started, err
				}
			}
		}
	}
	return answers
}

func prepareOwnerIndex(ctx context.Context, root string, observers map[string]preparedIndex) preparedIndex {
	prepared, ok := observers[root]
	if !ok {
		prepared.observer, prepared.err = (project.Project{Root: root}).ObserveIndex(ctx)
		observers[root] = prepared
	}
	return prepared
}

func formatLifespan(d time.Duration) string {
	for _, u := range []struct {
		unit time.Duration
		name string
	}{{7 * 24 * time.Hour, "w"}, {24 * time.Hour, "d"}, {time.Hour, "h"}, {time.Minute, "m"}} {
		if d%u.unit == 0 {
			return fmt.Sprintf("%d%s", d/u.unit, u.name)
		}
	}
	return fmt.Sprintf("%ds", int64(d/time.Second))
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// MkTemp creates a temporary directory with a lifespan and returns its path.
// Inside a Git project it lives in .obit/tmp (excluded from Git); elsewhere
// in the system temporary directory.
func (e *Engine) MkTemp(lifespan time.Duration, name, cwd, session string) (string, *core.Item, error) {
	cwd, err := validateTempRequest(cwd, name, session)
	if err != nil {
		return "", nil, err
	}
	if err := e.Svc.Store.Context().Err(); err != nil {
		return "", nil, err
	}
	if name = strings.Trim(unsafeName.ReplaceAllString(name, "-"), "-."); name == "" {
		name = "tmp"
	}
	base := os.TempDir()
	proj, inProject, err := project.Find(cwd)
	if err != nil {
		return "", nil, err
	}
	if inProject {
		if err := e.Svc.Store.Context().Err(); err != nil {
			return "", nil, err
		}
		if err := proj.Ensure(); err != nil {
			return "", nil, fmt.Errorf("prepare %s: %w", proj.WorkDir(), err)
		}
		base = filepath.Join(proj.WorkDir(), "tmp")
	}
	if err := e.Svc.Store.Context().Err(); err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(base, 0o755); err != nil {
		return "", nil, err
	}
	dir, err := os.MkdirTemp(base, name+"-")
	if err != nil {
		return "", nil, err
	}
	results, err := e.Svc.Add([]string{dir}, core.AddOptions{
		ExpiresAt: e.now().Add(lifespan), Source: "mktemp", Project: proj.Root, Session: session,
	})
	rollback := func(cause error) (string, *core.Item, error) {
		if err := os.Remove(dir); err != nil {
			return dir, nil, errors.Join(cause, fmt.Errorf("rollback temporary directory %s: %w", dir, err))
		}
		return "", nil, cause
	}
	if err != nil && results == nil {
		return rollback(err)
	}
	if results[0].Error != "" {
		return rollback(fmt.Errorf("%s", results[0].Error))
	}
	return dir, results[0].Item, err
}

func validateTempRequest(cwd, name, session string) (string, error) {
	cwd, err := pathutil.Abs(cwd)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(cwd)
	if err != nil {
		return "", fmt.Errorf("mktemp cwd must be an existing directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("mktemp cwd %q is not a directory", cwd)
	}
	if err := core.ValidateMetadata("session", session); err != nil {
		return "", err
	}
	if len(name) > 128 {
		return "", errors.New("temporary directory name exceeds 128 bytes")
	}
	return cwd, nil
}
