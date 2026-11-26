package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
	"github.com/woodfishhhh/obituary/internal/pathutil"
)

func (s *Service) captureParents(path string) (string, []BurialParent, error) {
	var parents []BurialParent
	for dir := pathutil.Parent(path); ; dir = pathutil.Parent(dir) {
		info, err := os.Lstat(dir)
		if err != nil {
			return "", nil, err
		}
		id, err := s.FS.IdentifyFollowing(dir)
		if err != nil {
			return "", nil, err
		}
		reparse := isReparse(info)
		recordedPath := dir
		if runtime.GOOS == "darwin" && !reparse && !hasReparseAncestor(dir) {
			if resolver, ok := s.FS.(corefs.RestoreParentPathResolver); ok {
				recordedPath, err = resolver.RestoreParentPath(dir)
				if err != nil {
					return "", nil, err
				}
			}
		}
		parents = append(parents, BurialParent{Path: recordedPath, Identity: id, Reparse: reparse})
		if pathutil.Parent(dir) == dir {
			if len(parents) > 0 {
				path = filepath.Join(parents[0].Path, filepath.Base(path))
			}
			return path, parents, nil
		}
	}
}

func hasReparseAncestor(path string) bool {
	for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
		info, err := os.Lstat(parent)
		if err != nil {
			return true
		}
		if isReparse(info) {
			return true
		}
		if filepath.Dir(parent) == parent {
			return false
		}
	}
}

func (s *Service) preflightRestoreParent(b Burial, dest, nearest string, parent RestoreParent) error {
	if err := s.inspectRestoreParents(b, true); err != nil {
		return err
	}
	// The initial lookup found missing components. They may have reappeared
	// during inspection, but their stale names must never authorize mkdir on POSIX.
	if runtime.GOOS != "windows" && nearest != filepath.Dir(dest) {
		return changedRestoreParent(filepath.Dir(dest), nil)
	}
	current, err := s.FS.IdentifyFollowing(nearest)
	if err != nil {
		return err
	}
	pinned := parent.Identity()
	if pinned.Serial != current.Serial || pinned.FileID != current.FileID {
		return &corefs.RestoreParentError{Kind: "pinned", Path: nearest, Cause: ErrUnsafe}
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
	if runtime.GOOS != "windows" && len(b.Parents) > 0 {
		if checker, ok := s.FS.(corefs.RestoreParentChainChecker); ok {
			records := make([]corefs.RestoreParentRecord, len(b.Parents))
			for i, parent := range b.Parents {
				records[i] = corefs.RestoreParentRecord{Path: parent.Path, Identity: parent.Identity, Reparse: parent.Reparse}
			}
			return checker.VerifyRestoreParentChain(records)
		}
	}
	return s.inspectRestoreParents(b, false)
}

var shortComponent = regexp.MustCompile(`(?i)^[^ .\\/:]{1,6}~[0-9]{1,6}(\.[^ .\\/:]{1,3})?$`)

func (s *Service) inspectRestoreParents(b Burial, allowMissing bool) error {
	var dirs []string
	for dir := pathutil.Parent(b.Path); ; dir = pathutil.Parent(dir) {
		dirs = append(dirs, dir)
		if pathutil.Parent(dir) == dir {
			break
		}
	}
	if len(b.Parents) != 0 && len(b.Parents) != len(dirs) {
		return fmt.Errorf("original parent chain is incomplete: %w", ErrUnsafe)
	}
	for n := range dirs {
		index := n
		// An inaccessible suffix cannot establish which of its ancestors moved.
		if runtime.GOOS != "windows" {
			index = len(dirs) - 1 - n
		}
		dir := dirs[index]
		var recorded *BurialParent
		if len(b.Parents) > 0 {
			if filepath.Clean(b.Parents[index].Path) != dir {
				return fmt.Errorf("original parent chain is incomplete: %w", ErrUnsafe)
			}
			recorded = &b.Parents[index]
		}
		if err := s.inspectRestoreParent(b, dir, recorded, allowMissing); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) inspectRestoreParent(b Burial, dir string, recorded *BurialParent, allowMissing bool) error {
	info, err := os.Lstat(dir)
	missing := os.IsNotExist(err) && pathutil.Parent(dir) != dir
	if missing && runtime.GOOS != "windows" {
		return changedRestoreParent(dir, recorded)
	}
	if missing && allowMissing {
		// Missing components cannot reveal a former alias. Keep the old
		// tilde heuristic, applied only to this component, as a conservative
		// refusal; it must not contaminate ordinary long-name descendants.
		if !b.LongNames && shortComponent.MatchString(filepath.Base(dir)) {
			return fmt.Errorf("legacy short-name parent %s is missing; original long name is unavailable: %w", dir, ErrUnsafe)
		}
		if recorded != nil && recorded.Reparse {
			return changedRestoreParent(dir, recorded)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect original parent %s: %w", dir, err)
	}
	if isReparse(info) && (recorded == nil || !recorded.Reparse) {
		return &corefs.RestoreParentError{Kind: "link", Path: dir, Cause: ErrUnsafe}
	}
	id, err := s.FS.IdentifyFollowing(dir)
	if err != nil {
		return err
	}
	if !b.LongNames {
		if err := s.checkLegacyRestoreParent(dir, recorded, id); err != nil {
			return err
		}
	}
	if recorded != nil && (recorded.Reparse && !isReparse(info) || !restoreParentIdentityMatches(recorded, id)) {
		return changedRestoreParent(dir, recorded)
	}
	return nil
}

// A case-insensitive lookup can return the recorded inode under an obsolete spelling.
func (s *Service) exactRestoreName(path string) (bool, error) {
	if runtime.GOOS == "linux" {
		return true, nil
	}
	resolver, ok := s.FS.(corefs.RestoreParentPathResolver)
	if !ok {
		return true, nil
	}
	actual, err := resolver.RestoreParentPath(path)
	if err != nil {
		return false, err
	}
	return filepath.Clean(actual) == filepath.Clean(path), nil
}

func restoreParentIdentityMatches(recorded *BurialParent, id Identity) bool {
	if id.Serial != recorded.Identity.Serial {
		return false
	}
	if runtime.GOOS == "windows" && !recorded.Reparse {
		return true
	}
	return recorded.Identity.RequirePersistent() == nil && id.RequirePersistent() == nil && id.FileID == recorded.Identity.FileID
}

func changedRestoreParent(dir string, recorded *BurialParent) error {
	kind := "unverified"
	if recorded != nil && recorded.Reparse {
		kind = "link"
	}
	return &corefs.RestoreParentError{Kind: kind, Path: dir, Cause: ErrUnsafe}
}

func (s *Service) checkLegacyRestoreParent(dir string, recorded *BurialParent, id Identity) error {
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
	return nil
}
