package core

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/internal/fileio"
)

// Extend pushes the expiry of each referenced item back by d (counted from now if already due).
func (s *Service) Extend(refs []string, d time.Duration) ([]Result, error) {
	now := s.now()
	return s.mutate(refs, ActionExtended, func(st *State, idx int) {
		item := &st.Items[idx]
		base := item.ExpiresAt
		if base.Before(now) {
			base = now
		}
		item.ExpiresAt = base.Add(d)
		if limit := now.Add(MaxLifespan); item.ExpiresAt.After(limit) {
			item.ExpiresAt = limit
		}
		item.Attempts, item.NextAttempt, item.LastError = 0, time.Time{}, ""
		item.Rev++
	})
}

// Cancel stops tracking each referenced item; the files themselves are untouched.
func (s *Service) Cancel(refs []string) ([]Result, error) {
	return s.mutate(refs, ActionCanceled, func(st *State, idx int) {
		s.spareWatchObject(st, st.Items[idx].WatchID, st.Items[idx].Identity, st.Items[idx].Path, "spared")
		s.spareAutomaticObject(st, st.Items[idx].Identity, st.Items[idx].Path)
		st.Items = append(st.Items[:idx], st.Items[idx+1:]...)
	})
}

func (s *Service) mutate(refs []string, action string, apply func(*State, int)) ([]Result, error) {
	s, notify := s.trackChanges()
	defer notify()
	now := s.now()
	results := make([]Result, len(refs))
	var events []Event
	err := s.Store.UpdateHot(func(st *State) error {
		roots := s.resolveGraveyardRoots(*st)
		for i, ref := range refs {
			results[i].Ref = ref
			idx, path, err := s.find(st, ref)
			if err != nil {
				results[i].Error, results[i].Err = err.Error(), err
				continue
			}
			if idx < 0 {
				results[i].Error = fmt.Sprintf("%s is not tracked", ref)
				if j, lookupErr := s.findBurial(st, ref); lookupErr == nil && j >= 0 {
					b := st.Burials[j]
					if hint := pendingRestoreHint(b); hint != "" {
						results[i].Error = hint
						continue
					}
				}
				if id, err := strconv.ParseInt(strings.TrimPrefix(ref, "#"), 10, 64); err == nil {
					for _, b := range st.Burials {
						if id == b.ID || slices.Contains(b.Contains, id) {
							results[i].Error = fmt.Sprintf("#%d was buried with #%d; obit resurrect %d", id, b.ID, b.ID)
							break
						}
					}
				}
				continue
			}
			if err := s.checkItemMutation(st, idx, path, roots); err != nil {
				results[i].Error, results[i].Err = err.Error(), err
				continue
			}
			id := st.Items[idx].ID
			st.Items[idx].Path = path
			after := st.Items[idx]
			apply(st, idx)
			st.dropWatchBurial(id)
			if j := st.Index(id); j >= 0 {
				after = st.Items[j]
			}
			results[i].Action, results[i].Item = action, &after
			events = append(events, Event{Time: now, Action: action, ID: after.ID, Path: after.Path,
				Detail: "expires " + after.ExpiresAt.Format(time.RFC3339)})
		}
		return nil
	})
	if commitFailed(err) {
		return nil, lifespanCommitError(err)
	}
	return results, errors.Join(err, s.Store.RecordCommitted(events...))
}

func lifespanCommitError(err error) error {
	if fileio.IsCommitUnconfirmed(err) {
		return fmt.Errorf("commit unconfirmed: the change may already have taken effect; check obit list before retrying: %w", err)
	}
	return err
}
