package lifecycle

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"slices"
	"time"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
)

var errLifespanBudget = errors.New("continue burial directory inspection outside the ledger lock")

func (s *Service) commitChildLifespans(st *State, out Outcome, reason string) ([]Item, error) {
	item := out.Item
	if err := s.checkChildLifespanState(st, item, out.Path, reason); err != nil {
		return nil, err
	}
	if !item.IsDir {
		return nil, nil
	}
	snap := out.lifespans
	if snap == nil {
		return nil, errors.New("burial lifespan snapshot is unavailable")
	}
	real, err := s.FS.RealPath(out.Path)
	if err != nil {
		return nil, err
	}
	if !samePath(real, snap.real) {
		return nil, errors.New("burial directory changed before commit")
	}
	return s.currentBurialChildren(st, item, snap)
}

func (s *Service) currentBurialChildren(st *State, item Item, snap *lifespanSnapshot) ([]Item, error) {
	active := make(map[string]bool)
	for i := range st.Items {
		other := &st.Items[i]
		if other.ID == item.ID || other.Kind == KindContainer || other.Kind == KindDockerVolume {
			continue
		}
		if other.Identity == nil {
			return nil, fmt.Errorf("cannot verify lifespan #%d without an identity; add its path again", other.ID)
		}
		if other.Identity.Serial == item.Identity.Serial {
			active[other.Identity.FileID] = false
		}
	}
	inside, err := s.containedIdentities(item, snap, active)
	if err != nil {
		return nil, err
	}
	var children []Item
	for i := range st.Items {
		other := &st.Items[i]
		if other.ID == item.ID || other.Identity == nil || other.Identity.Serial != item.Identity.Serial || !inside[other.Identity.FileID] {
			continue
		}
		if other.ExpiresAt.After(snap.expires) {
			return nil, fmt.Errorf("holds #%d, which has a later expiry", other.ID)
		}
		children = append(children, *other)
	}
	slices.SortFunc(children, func(a, b Item) int { return cmp.Compare(a.ID, b.ID) })
	return children, nil
}

func (s *Service) containedIdentities(item Item, snap *lifespanSnapshot, active map[string]bool) (map[string]bool, error) {
	if snap.changes != nil {
		return snap.inventory, snap.changes.Check()
	}
	ctx, cancel := context.WithTimeout(s.Store.Context(), 50*time.Millisecond)
	defer cancel()
	err := s.walkBurialTree(ctx, snap.real, *item.Identity, nil, 256, func(id string) {
		if _, ok := active[id]; ok {
			active[id] = true
		}
	})
	if errors.Is(err, errLifespanBudget) || errors.Is(err, context.DeadlineExceeded) && s.Store.Context().Err() == nil {
		snap.wide = true
		return nil, errLifespanBudget
	}
	return active, err
}

// A large inventory is only authoritative while the kernel still reports an unchanged tree.
// Keep every same-volume ID so registrations between scanning and commit are covered too.
func (s *Service) prepareWideInventory(out *Outcome) {
	snap := out.lifespans
	if snap == nil || !snap.wide || snap.changes != nil || out.Action != "" {
		return
	}
	changes, err := s.FS.NewDirectoryChanges(snap.real)
	if err == nil {
		snap.changes = changes
		snap.inventory = make(map[string]bool)
		err = s.walkBurialTree(s.Store.Context(), snap.real, *out.Item.Identity, changes, -1, func(id string) { snap.inventory[id] = true })
	}
	if err != nil {
		*out = failedMove(*out, err)
	}
}

func (snap *lifespanSnapshot) close() {
	if snap != nil && snap.changes != nil {
		_ = snap.changes.Close()
		snap.changes = nil
		snap.inventory = nil
	}
}

type burialDirectory struct {
	path string
	id   Identity
}

func (s *Service) walkBurialTree(ctx context.Context, root string, id Identity, changes corefs.DirectoryChanges, remaining int, visit func(string)) error {
	dirs := []burialDirectory{{root, id}}
	seen := map[string]bool{id.FileID: true}
	for len(dirs) > 0 {
		dir := dirs[len(dirs)-1]
		dirs = dirs[:len(dirs)-1]
		if err := ctx.Err(); err != nil {
			return err
		}
		if changes != nil {
			if err := changes.WatchDirectory(dir.path); err != nil {
				return err
			}
		}
		f, err := s.FS.OpenBurialDirectory(dir.path, dir.id)
		if err != nil {
			return fmt.Errorf("inspect burial directory %s: %v", dir.path, err) //nolint:errorlint // Uncertain contents must retain the parent for retry.
		}
		err = eachBurialEntry(f, func(e fs.DirEntry) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if remaining == 0 {
				return errLifespanBudget
			}
			remaining--
			path := filepath.Join(dir.path, e.Name())
			child, descend, err := s.FS.BurialEntry(f, path, dir.id)
			if err != nil {
				return fmt.Errorf("inspect burial descendant %s: %v", path, err) //nolint:errorlint // An unsafe child must retry, not retire the parent's registration.
			}
			if child.Serial != id.Serial {
				return fmt.Errorf("burial descendant is on another device: %s", path)
			}
			if descend {
				if seen[child.FileID] {
					return fmt.Errorf("directory occurs twice under %s", root)
				}
				seen[child.FileID] = true
				dirs = append(dirs, burialDirectory{path, child})
			}
			visit(child.FileID)
			return nil
		})
		_ = f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// ReadDir(-1) can read and sort an unbounded directory within a single lock slice.
func eachBurialEntry(f fs.ReadDirFile, visit func(fs.DirEntry) error) error {
	for {
		entries, err := f.ReadDir(32)
		for _, e := range entries {
			if err := visit(e); err != nil {
				return err
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
	}
}
