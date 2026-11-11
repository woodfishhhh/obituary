package auto

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/pathutil"
	"github.com/woodfishhhh/obituary/internal/project"
)

type preparedIndex struct {
	observer  *project.IndexObserver
	err       error
	afterFind func() // Tests schedule a publication after the real project discovery.
}

type discoveredOwner struct {
	project project.Project
	found   bool
	err     error
}

func ownerGroups(ctx context.Context, paths []string, known map[string]discoveredOwner, answers []core.Ownership) map[string][]int {
	groups := make(map[string][]int)
	for i, path := range paths {
		if err := ctx.Err(); err != nil {
			for j := i; j < len(answers); j++ {
				answers[j].Tracked, answers[j].Err = true, err
			}
			break
		}
		discovery, cached := known[path]
		// A missing worktree has no index snapshot to prove it remains absent.
		// Folder policies can outlive the creation of a Git repository here.
		if !cached || !discovery.found {
			p, found, err := project.FindContext(ctx, path)
			discovery = discoveredOwner{project: p, found: found, err: err}
			if known != nil {
				known[path] = discovery
			}
		}
		p, ok, err := discovery.project, discovery.found, discovery.err
		answers[i].Root, answers[i].Err = p.Root, err
		answers[i].Tracked = err != nil
		if err == nil && !ok {
			answers[i].CheckedAt = time.Now()
		}
		if err == nil && ok {
			groups[p.Root] = append(groups[p.Root], i)
		}
	}
	return groups
}

func ownerGroupPaths(paths []string, indices []int, names map[string]string) ([]string, bool) {
	group := make([]string, len(indices))
	prepared := true
	for j, i := range indices {
		group[j] = paths[i]
		if long, ok := names[paths[i]]; ok {
			group[j] = long
		} else {
			prepared = false
		}
	}
	return group, prepared
}

func queryIndex(ctx context.Context, observer *project.IndexObserver, paths []string, prepared bool) ([]bool, *project.IndexSnapshot, error) {
	if prepared {
		return observer.TrackedRealSnapshot(ctx, paths)
	}
	return observer.TrackedSnapshot(ctx, paths)
}

// PrepareOwners has one sweep's lifetime. Slow path resolution happens before
// taking store.lock; only metadata checks and bounded fallback queries run inside it.
func PrepareOwners(ctx context.Context, paths []string) core.OwnerBatchFunc {
	known := make(map[string]discoveredOwner, len(paths))
	names := make(map[string]string, len(paths))
	directories := make(map[string]discoveredOwner)
	observers := make(map[string]preparedIndex)
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			known[path] = discoveredOwner{err: err}
			continue
		}
		long, err := pathutil.LongAbs(path)
		if err != nil {
			known[path] = discoveredOwner{err: err}
			continue
		}
		// Reuse only an already-long spelling. An alias may be reassigned
		// before the query and must be expanded again at that boundary.
		if long == path {
			names[path] = long
		}
		dir := path
		info, statErr := os.Stat(path)
		if statErr != nil || !info.IsDir() {
			dir = filepath.Dir(path)
		}
		discovery, ok := directories[dir]
		if !ok {
			p, found, err := project.FindContext(ctx, dir)
			discovery = discoveredOwner{project: p, found: found, err: err}
			directories[dir] = discovery
		}
		p, found, err := discovery.project, discovery.found, discovery.err
		known[path] = discovery
		if err != nil || !found {
			continue
		}
		root := p.Root
		if _, ok := observers[root]; ok {
			continue
		}
		// ObserveIndex runs several Git commands. Each retains project.gitTimeout;
		// a shared 2s budget would disable snapshots on slow-starting machines.
		observer, err := (project.Project{Root: root}).ObserveIndex(ctx)
		observers[root] = preparedIndex{observer: observer, err: err}
	}
	return func(ctx context.Context, paths []string) []core.Ownership {
		return queryOwners(ctx, paths, observers, known, names)
	}
}
