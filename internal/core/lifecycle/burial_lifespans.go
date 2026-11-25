package lifecycle

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	corefs "github.com/woodfishhhh/obituary/internal/core/fs"
)

type lifespanSnapshot struct {
	real      string
	expires   time.Time
	wide      bool
	weak      bool
	inventory map[string]bool
	changes   corefs.DirectoryChanges
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
	if err := s.preflightContained(st, item, real, expires); err != nil {
		return nil, err
	}
	return &lifespanSnapshot{real: real, expires: expires}, nil
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

// Resolve registered spellings outside store.lock, including legacy records without an identity.
func (s *Service) preflightContained(st *State, item Item, real string, expires time.Time) error {
	for _, other := range st.Items {
		if other.Kind == KindContainer {
			continue
		}
		if other.Kind == KindDockerVolume || other.ID == item.ID {
			continue
		}
		var current string
		var err error
		if other.Identity == nil {
			current, err = s.FS.RealPath(other.Path)
		} else {
			if other.Identity.Serial != item.Identity.Serial {
				continue
			}
			current, err = s.LocateKnown(other.Path, *other.Identity)
		}
		if errors.Is(err, ErrNotFound) || runtime.GOOS != "windows" && other.Identity != nil && errors.Is(err, ErrUnsupported) {
			continue
		}
		if err != nil {
			return fmt.Errorf("could not locate #%d: %v", other.ID, err) //nolint:errorlint // Another item's lookup error must retry this parent.
		}
		if isInside(current, real) {
			if other.ExpiresAt.After(expires) {
				return fmt.Errorf("holds #%d, which has a later expiry", other.ID)
			}
			if other.Identity == nil {
				return fmt.Errorf("contained #%d lacks an identity", other.ID)
			}
		}
	}
	return nil
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
