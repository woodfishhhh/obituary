package core

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// freedesktop's Path metadata and gio-tool-trash.c restore_trash rebuild the
// original parent directory. Ordinary directories are locations, not objects;
// only volume changes and redirected reparse points invalidate that location.
func (s *Service) captureParents(path string) ([]BurialParent, error) {
	var parents []BurialParent
	for dir := pathutil.Parent(path); ; dir = pathutil.Parent(dir) {
		info, err := os.Lstat(dir)
		if err != nil {
			return nil, err
		}
		id, err := s.FS.IdentifyFollowing(dir)
		if err != nil {
			return nil, err
		}
		parents = append(parents, BurialParent{Path: dir, Identity: id, Reparse: isReparse(info)})
		if pathutil.Parent(dir) == dir {
			return parents, nil
		}
	}
}

func (s *Service) preflightRestoreParent(b Burial, dest, nearest string, parent RestoreParent) error {
	if err := s.inspectRestoreParents(b, true); err != nil {
		return err
	}
	current, err := s.FS.IdentifyFollowing(nearest)
	if err != nil {
		return err
	}
	pinned := parent.Identity()
	if pinned.Serial != current.Serial || pinned.FileID != current.FileID {
		return fmt.Errorf("original parent %s no longer names the pinned directory: %w", nearest, ErrUnsafe)
	}
	rel, err := filepath.Rel(nearest, dest)
	if err != nil {
		return err
	}
	path := filepath.Join(parent.Path(), rel)
	if reason := s.Guard.ProtectedReason(path); reason != "" {
		return &RefusedError{Path: path, Reason: reason}
	}
	return nil
}

func (s *Service) checkRestoreParents(b Burial) error {
	return s.inspectRestoreParents(b, false)
}

var shortComponent = regexp.MustCompile(`(?i)^[^ .\\/:]{1,6}~[0-9]{1,6}(\.[^ .\\/:]{1,3})?$`)

func (s *Service) inspectRestoreParents(b Burial, allowMissing bool) error {
	index := 0
	for dir := pathutil.Parent(b.Path); ; dir = pathutil.Parent(dir) {
		var recorded *BurialParent
		if len(b.Parents) > 0 {
			if index >= len(b.Parents) || filepath.Clean(b.Parents[index].Path) != dir {
				return fmt.Errorf("original parent chain is incomplete: %w", ErrUnsafe)
			}
			recorded = &b.Parents[index]
		}
		index++
		info, err := os.Lstat(dir)
		if allowMissing && os.IsNotExist(err) && pathutil.Parent(dir) != dir {
			// Missing components cannot reveal a former alias. Keep the old
			// tilde heuristic, applied only to this component, as a conservative
			// refusal; it must not contaminate ordinary long-name descendants.
			if !b.LongNames && shortComponent.MatchString(filepath.Base(dir)) {
				return fmt.Errorf("legacy short-name parent %s is missing; original long name is unavailable: %w", dir, ErrUnsafe)
			}
			if recorded != nil && recorded.Reparse {
				return fmt.Errorf("recorded reparse parent %s is missing: %w", dir, ErrUnsafe)
			}
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect original parent %s: %w", dir, err)
		}
		if isReparse(info) && (recorded == nil || !recorded.Reparse) {
			return fmt.Errorf("original parent %s is now a link or junction: %w", dir, ErrUnsafe)
		}
		id, err := s.FS.IdentifyFollowing(dir)
		if err != nil {
			return err
		}
		if !b.LongNames {
			expanded, complete, err := s.FS.LongPath(dir)
			if err != nil {
				return err
			}
			equal, err := s.FS.SameName(filepath.Base(expanded), filepath.Base(dir))
			if err != nil {
				return err
			}
			alias := !equal || !complete
			if alias && (recorded == nil || id.Serial != recorded.Identity.Serial || id.FileID != recorded.Identity.FileID) {
				return fmt.Errorf("legacy short-name parent %s cannot be verified; original long name is unavailable: %w", dir, ErrUnsafe)
			}
		}
		if recorded != nil && (id.Serial != recorded.Identity.Serial || recorded.Reparse && (!isReparse(info) || id.FileID != recorded.Identity.FileID)) {
			return fmt.Errorf("original parent %s changed identity: %w", dir, ErrUnsafe)
		}
		if pathutil.Parent(dir) == dir {
			break
		}
	}
	if len(b.Parents) > 0 && index != len(b.Parents) {
		return fmt.Errorf("original parent chain has extra entries: %w", ErrUnsafe)
	}
	return nil
}
