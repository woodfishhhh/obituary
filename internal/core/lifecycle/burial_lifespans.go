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
	if s.preparedLifespans != nil {
		if err := s.checkChildLifespanState(st, item, path, reason); err != nil {
			return nil, err
		}
		return s.validateLifespanSnapshot(st, item, path, s.preparedLifespans, s.allowedItemChanges)
	}
	if err := s.checkChildLifespanState(st, item, path, reason); err != nil {
		return nil, err
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
	children, _, _, err := s.scanContainedLifespans(st, item, real, expires)
	return children, err
}

type lifespanSnapshot struct {
	real     string
	expires  time.Time
	children []Item
	entries  map[int64]lifespanEntry
	missing  bool
}

type lifespanEntry struct {
	path       string
	ledgerPath string
	rev        int64
	expiresAt  time.Time
	isDir      bool
	kind       string
	identity   *Identity
}

func (s *Service) preflightChildLifespans(st *State, item Item, path, reason string) (*lifespanSnapshot, error) {
	if err := s.checkChildLifespanState(st, item, path, reason); err != nil {
		return nil, err
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
	children, entries, missing, err := s.scanContainedLifespans(st, item, real, expires)
	if err != nil {
		return nil, err
	}
	return &lifespanSnapshot{real: real, expires: expires, children: children, entries: entries, missing: missing}, nil
}

func (s *Service) checkChildLifespanState(st *State, item Item, path, reason string) error {
	if err := s.checkWatchedFolders(st, path, *item.Identity, item.IsDir); err != nil {
		return err
	}
	if err := s.checkWatchScope(st, item, path, reason); err != nil {
		return err
	}
	if reason == ReasonExpired && automatic(item) {
		if exempt, err := s.AutomaticExempt(st, path, *item.Identity); err != nil {
			return watchExemptionRetry(err)
		} else if exempt {
			return fmt.Errorf("automatic expiry paused: kept by resurrect or spare until its next write")
		}
	}
	return nil
}

func (s *Service) scanContainedLifespans(st *State, item Item, real string, expires time.Time) ([]Item, map[int64]lifespanEntry, bool, error) {
	var children []Item
	entries := make(map[int64]lifespanEntry, len(st.Items))
	missing := false
	var err error
	for _, other := range st.Items {
		if other.Kind == KindContainer {
			continue
		}
		if other.ID == item.ID {
			continue
		}
		current := other.Path
		entries[other.ID] = lifespanEntryOf(other, current)
		if other.Identity != nil {
			if other.Identity.Serial != item.Identity.Serial {
				continue
			}
			current, err = s.LocateKnown(other.Path, *other.Identity)
		} else {
			current, err = s.FS.RealPath(current)
		}
		if errors.Is(err, ErrNotFound) {
			missing = true
			continue
		}
		if err != nil {
			return nil, nil, false, fmt.Errorf("could not locate #%d: %v", other.ID, err) //nolint:errorlint // Another item's ErrUnsafe or RefusedError must retry, not refuse the parent.
		}
		entries[other.ID] = lifespanEntryOf(other, current)
		if isInside(current, real) {
			if other.ExpiresAt.After(expires) {
				return nil, nil, false, fmt.Errorf("holds #%d, which has a later expiry", other.ID)
			}
			if other.Identity == nil {
				return nil, nil, false, fmt.Errorf("contained #%d lacks an identity", other.ID)
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
	return children, entries, missing, nil
}

func cloneIdentity(identity *Identity) *Identity {
	if identity == nil {
		return nil
	}
	copy := *identity
	return &copy
}

func lifespanEntryOf(item Item, path string) lifespanEntry {
	return lifespanEntry{path: path, ledgerPath: item.Path, rev: item.Rev, expiresAt: item.ExpiresAt, isDir: item.IsDir, kind: item.Kind, identity: cloneIdentity(item.Identity)}
}

func (e lifespanEntry) matches(item Item) bool {
	if e.ledgerPath != item.Path || e.rev != item.Rev || !e.expiresAt.Equal(item.ExpiresAt) || e.isDir != item.IsDir || e.kind != item.Kind {
		return false
	}
	if e.identity == nil || item.Identity == nil {
		return e.identity == nil && item.Identity == nil
	}
	return *e.identity == *item.Identity
}

func (s *Service) validateLifespanSnapshot(st *State, item Item, path string, snapshot *lifespanSnapshot, allowedItemChanges map[int64]bool) ([]Item, error) {
	if !item.IsDir {
		return nil, nil
	}
	if snapshot == nil {
		return nil, errors.New("burial lifespan snapshot is unavailable")
	}
	real, err := s.FS.RealPath(path)
	if err != nil {
		return nil, err
	}
	if !samePath(real, snapshot.real) {
		return nil, errors.New("burial directory changed before commit")
	}
	if snapshot.missing {
		children, _, _, err := s.scanContainedLifespans(st, item, real, snapshot.expires)
		return children, err
	}
	for _, current := range st.Items {
		if current.Kind == KindContainer || current.ID == item.ID {
			continue
		}
		entry, ok := snapshot.entries[current.ID]
		if !ok {
			if isInside(current.Path, real) {
				return nil, fmt.Errorf("new lifespan #%d entered the burial directory", current.ID)
			}
			continue
		}
		if allowedItemChanges[current.ID] || entry.matches(current) {
			continue
		}
		if isInside(entry.path, real) || isInside(current.Path, real) {
			return nil, fmt.Errorf("lifespan #%d changed before burial", current.ID)
		}
	}
	return snapshot.children, nil
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
		return &ReferenceError{Kind: "already_buried", Ref: path, BurialID: id, cause: &RefusedError{Path: path, Reason: "already in the graveyard; first use obit resurrect " + id}}
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
