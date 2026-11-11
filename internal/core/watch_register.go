package core

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// RegisterWatchChild retains first-seen age and refreshes modified deadlines.
// Registration and observations share Add's identity/conflict checks and the
// policy revision under the existing store lock.
func (s *Service) RegisterWatchChild(policy WatchPolicy, path string) error {
	if pathutil.IsExplorerMetadata(filepath.Base(path)) {
		return &RefusedError{Path: path, Reason: "Explorer label infrastructure is never registered automatically"}
	}
	if watchDownload(path) {
		return nil
	}
	if insideGitMetadata(path) {
		return &RefusedError{Path: path, Reason: "Git metadata is never registered automatically"}
	}
	c, err := s.inspect(path, false)
	if err != nil {
		return err
	}
	var event *Event
	err = s.Store.UpdateHot(func(st *State) error {
		p, err := s.watchRegistrationPolicy(st, policy, c)
		if err != nil {
			return err
		}
		if st.watchExempt(p.ID, c.identity) {
			return errWatchUnchanged
		}
		if idx := st.FindIdentity(c.identity); idx >= 0 {
			return s.refreshWatchItem(st, idx, p, c)
		}
		roots := s.resolveGraveyardRoots(*st, path)
		if err := s.checkWatchNew(st, c, roots); err != nil {
			return err
		}
		item, err := s.newWatchItem(p, c)
		if err != nil {
			return err
		}
		item.ID = st.NextID
		st.NextID++
		st.Items = append(st.Items, item)
		event = &Event{Time: s.now(), Action: ActionAdded, ID: item.ID, Path: item.Path, Detail: "folder policy " + p.Lifespan + " from " + p.Age}
		return nil
	})
	if errors.Is(err, errWatchUnchanged) {
		return nil
	}
	if err != nil || event == nil {
		return err
	}
	return s.Store.RecordCommitted(*event)
}

func (s *Service) checkWatchNew(st *State, c *candidate, roots []resolvedGraveyardRoot) error {
	if c.isDir {
		// Directories only receive dates; descendant keep decisions are shown
		// in their burial proposal instead of suppressing that proposal.
		return s.checkNew(st, c, roots)
	}
	return s.checkAutomaticNew(st, c, roots)
}

func (s *Service) watchRegistrationPolicy(st *State, snapshot WatchPolicy, c *candidate) (WatchPolicy, error) {
	idx := slices.IndexFunc(st.Watches, func(p WatchPolicy) bool { return p.ID == snapshot.ID && p.Rev == snapshot.Rev })
	if idx < 0 || st.Watches[idx].Consent != ConsentApproved {
		return WatchPolicy{}, &RefusedError{Path: c.path, Reason: "folder policy changed or awaits approval"}
	}
	p := st.Watches[idx]
	_, err := s.WatchPath(p)
	if err != nil {
		return p, err
	}
	real, err := s.FS.RealPath(c.path)
	if err != nil {
		return p, err
	}
	parent, err := s.FS.IdentifyFollowing(filepath.Dir(real))
	if err != nil {
		return p, err
	}
	if !sameWatchIdentity(parent, p.Identity) {
		return p, &RefusedError{Path: c.path, Reason: "not a top-level child of the watched folder"}
	}
	if err := s.checkWatchExclusions(p, c.path, c.identity); err != nil {
		return p, err
	}
	if err := s.checkWatchedFolders(st, c.path, c.identity, c.isDir); err != nil {
		return p, err
	}
	return p, nil
}

func (s *Service) newWatchItem(p WatchPolicy, c *candidate) (Item, error) {
	if s.Owner == nil {
		return Item{}, fmt.Errorf("folder policies require Git ownership checks")
	}
	root, tracked, err := s.Owner(c.path)
	if err != nil {
		return Item{}, err
	}
	if tracked {
		return Item{}, &RefusedError{Path: c.path, Reason: "Git tracks it or something inside it"}
	}
	if !samePath(root, p.Project) {
		return Item{}, &RefusedError{Path: c.path, Reason: "the folder's Git ownership changed; set its policy again"}
	}
	if root != "" && s.Recheck != nil {
		if err := s.Recheck(root, c.path, c.isDir); err != nil {
			return Item{}, err
		}
	}
	snap, release, err := s.observeWatchChild(c.path, c.identity, c.isDir, p.Age)
	if err != nil {
		return Item{}, err
	}
	defer release()
	got, err := s.FS.Identify(c.path)
	if err != nil {
		return Item{}, err
	}
	if got != c.identity {
		return Item{}, ErrUnsafe
	}
	it := Item{Path: c.path, IsDir: c.isDir, Identity: &c.identity,
		CreatedAt: s.now(), Source: "watch", WatchID: p.ID, Project: root}
	err = updateWatchTiming(&it, p, snap, s.now())
	return it, err
}

var errWatchUnchanged = errors.New("watch observation unchanged")

func (s *Service) refreshWatchItem(st *State, idx int, p WatchPolicy, c *candidate) error {
	it := &st.Items[idx]
	if it.Source != "watch" || it.WatchID != p.ID {
		return errWatchUnchanged
	}
	age := p.Age
	if it.Watch != nil {
		age = it.Watch.Age
	}
	snap, release, err := s.observeWatchChild(c.path, c.identity, c.isDir, age)
	if err != nil {
		return err
	}
	defer release()
	rev := it.Rev
	if err := updateWatchTiming(it, p, snap, s.now()); err != nil {
		return err
	}
	if it.Rev == rev {
		return errWatchUnchanged
	}
	st.dropWatchBurial(it.ID)
	return nil
}
