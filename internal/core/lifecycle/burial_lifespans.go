package lifecycle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Check against the current transaction, including manual and newly added
// children. A lookup error is uncertainty, never permission to move a parent.
func (s *Service) checkChildLifespans(st *State, item Item, path, reason string) error {
	_, err := s.childLifespans(st, item, path, reason)
	return err
}

func (s *Service) childLifespans(st *State, item Item, path, reason string) ([]Item, error) {
	if err := s.checkWatchedFolders(st, path, *item.Identity, item.IsDir); err != nil {
		return nil, err
	}
	if err := s.checkWatchScope(st, item, path, reason); err != nil {
		return nil, err
	}
	if reason == ReasonExpired && automatic(item) {
		if exempt, err := s.AutomaticExempt(st, path, *item.Identity); err != nil {
			return nil, watchExemptionRetry(err)
		} else if exempt {
			return nil, fmt.Errorf("automatic expiry paused: kept by resurrect or spare until its next write")
		}
	}
	if !item.IsDir {
		return nil, nil
	}
	real, err := s.FS.RealPath(path)
	if err != nil {
		return nil, err
	}
	expires := item.ExpiresAt
	if reason == ReasonBury {
		expires = s.now()
	}
	return s.containedLifespans(st, item, real, expires)
}

func (s *Service) containedLifespans(st *State, item Item, real string, expires time.Time) ([]Item, error) {
	var children []Item
	var err error
	for _, other := range st.Items {
		if other.Kind == KindContainer {
			continue
		}
		if other.ID == item.ID {
			continue
		}
		current := other.Path
		if other.Identity != nil {
			if other.Identity.Serial != item.Identity.Serial {
				continue
			}
			current, err = s.FS.Locate(*other.Identity)
		} else {
			current, err = s.FS.RealPath(current)
		}
		if errors.Is(err, ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("could not locate #%d: %v", other.ID, err) //nolint:errorlint // Another item's ErrUnsafe or RefusedError must retry, not refuse the parent.
		}
		if isInside(current, real) {
			if other.ExpiresAt.After(expires) {
				return nil, fmt.Errorf("holds #%d, which has a later expiry", other.ID)
			}
			if other.Identity == nil {
				return nil, fmt.Errorf("contained #%d lacks an identity", other.ID)
			}
			children = append(children, other)
		}
	}
	slices.SortFunc(children, func(a, b Item) int {
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	return children, nil
}

// Called under store.lock, before changing any matching registration. Listing
// just this volume's metadata also catches unreadable or malformed manifests.
func (s *Service) checkBurying(it Item) error {
	path := it.Path
	if it.Identity != nil {
		path = it.Identity.Volume
	}
	root := s.graveyardRoot(path)
	entries, err := os.ReadDir(root)
	if err != nil {
		if s.missingOnOnlineVolume(root, err, it.Identity) {
			return nil
		}
		return fmt.Errorf("check burial for #%d: %w", it.ID, err)
	}
	prefix := fmt.Sprintf("%d-", it.ID)
	for _, entry := range entries {
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		if strings.HasPrefix(entry.Name(), prefix) {
			return fmt.Errorf("#%d is being buried; obit reap finishes it, then obit resurrect %d", it.ID, it.ID)
		}
		p := s.inspectManifest(root, filepath.Join(root, entry.Name()), State{})
		if p.Burial == nil {
			return fmt.Errorf("check burial for #%d: %s", it.ID, p.Error)
		}
		for _, child := range p.Burial.Children {
			if child.ID == it.ID || child.Identity != nil && it.Identity != nil && *child.Identity == *it.Identity {
				return fmt.Errorf("#%d is being buried with #%d; obit reap finishes it, then obit resurrect %d", it.ID, p.Burial.ID, p.Burial.ID)
			}
		}
	}
	return nil
}

// Called under store.lock: an inspection before the lock may predate another
// process's completed burial, including a move along with a parent directory.
func (s *Service) checkBuried(st *State, path string, identity *Identity, roots []resolvedGraveyardRoot) error {
	if graveyardMayMatch(st, path, identity) {
		if err := s.Store.LoadGraveyard(st); err != nil {
			return err
		}
	}
	refuse := func(id string) error {
		return &RefusedError{Path: path, Reason: "already in the graveyard; first use obit resurrect " + id}
	}
	for _, b := range st.Burials {
		sameObject := identity != nil && b.Identity != nil && *identity == *b.Identity
		if sameObject || samePath(b.Path, path) {
			if hint := pendingRestoreHint(b); hint != "" {
				return &RefusedError{Path: path, Reason: hint}
			}
		}
		if sameObject {
			return refuse(fmt.Sprint(b.ID))
		}
	}
	var current string
	var err error
	if identity != nil {
		if got, identifyErr := s.FS.Identify(path); identifyErr == nil && got == *identity {
			current, err = s.FS.RealPath(path)
		} else {
			current, err = s.FS.Locate(*identity)
		}
	} else {
		current, err = s.FS.RealPath(path)
	}
	if errors.Is(err, ErrNotFound) || identity == nil && errors.Is(err, os.ErrNotExist) {
		return nil // missing records may still be canceled
	}
	if err != nil {
		return fmt.Errorf("check current burial location: %w", err)
	}
	if !s.insideGraveyard(roots, current) {
		return nil
	}
	if err := s.Store.LoadGraveyard(st); err != nil {
		return err
	}
	for _, b := range st.Burials {
		grave, err := s.FS.RealPath(b.Grave)
		if err == nil && (samePath(current, grave) || isInside(current, grave)) {
			return refuse(fmt.Sprint(b.ID))
		}
	}
	return refuse("<id> (obit reap and obit graveyard show pending recovery)")
}
