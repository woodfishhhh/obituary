package lifecycle

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

// Retry makes each referenced item, whose burial failed, due again now, with
// a fresh count of attempts.
func (s *Service) Retry(refs []string) ([]Result, error) {
	return s.mutate(refs, ActionRetried, func(st *State, idx int) {
		item := &st.Items[idx]
		item.Attempts, item.NextAttempt, item.LastError = 0, time.Time{}, ""
		item.Rev++
	})
}

// Cancel stops tracking each referenced item; the files themselves are untouched.
func (s *Service) Cancel(refs []string) ([]Result, error) {
	return s.mutate(refs, ActionCanceled, func(st *State, idx int) {
		s.spareWatchObject(st, st.Items[idx].WatchID, st.Items[idx].Identity, st.Items[idx].Path, "spared")
		s.spareAutomaticObject(st, st.Items[idx].Identity, st.Items[idx].Path)
		s.forgetAutoReceipt(st, st.Items[idx])
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
				results[i].Err = s.untrackedLifespanHint(st, ref)
				results[i].Error = results[i].Err.Error()
				continue
			}
			if err := s.checkItemMutation(st, idx, path, roots); err != nil {
				results[i].Error, results[i].Err = err.Error(), err
				continue
			}
			id := st.Items[idx].ID
			identity := st.Items[idx].Identity
			st.Items[idx].Path = path
			after := st.Items[idx]
			apply(st, idx)
			dropWatchBurial(st, id)
			dropQuotaProposal(st, id, identity)
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

func (s *Service) untrackedLifespanHint(st *State, ref string) error {
	if j, lookupErr := s.findBurial(st, ref); lookupErr == nil && j >= 0 {
		b := st.Burials[j]
		if hint := pendingRestoreHint(b); hint != "" {
			return errors.New(hint)
		}
	}
	if id, err := strconv.ParseInt(strings.TrimPrefix(ref, "#"), 10, 64); err == nil {
		for _, b := range st.Burials {
			if id == b.ID || slices.Contains(b.Contains, id) {
				return &ReferenceError{Kind: "already_buried", Ref: ref, BurialID: strconv.FormatInt(b.ID, 10), cause: fmt.Errorf("#%d was buried with #%d; obit resurrect %d", id, b.ID, b.ID)}
			}
		}
	}
	return &ReferenceError{Kind: "untracked", Ref: ref, cause: fmt.Errorf("%s is not tracked", ref)}
}

func lifespanCommitError(err error) error {
	if fileio.IsCommitUnconfirmed(err) {
		return fmt.Errorf("commit unconfirmed: the change may already have taken effect; check obit list before retrying: %w", err)
	}
	return err
}
