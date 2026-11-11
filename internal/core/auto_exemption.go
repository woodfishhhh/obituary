package core

import (
	"errors"
	"path/filepath"
	"slices"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

type AutoExemption = ledger.AutoExemption

func (s *Service) spareAutomaticObject(st *State, identity *Identity, path string) {
	if identity == nil {
		return
	}
	e := AutoExemption{Identity: *identity, KeptAt: s.now()}
	snap, release, err := s.FS.ObserveWatch(path, false)
	if err == nil {
		release()
		if snap.Directory {
			snap, release, err = s.observeObjectTree(path, *identity, true, false, false)
			if err == nil {
				release()
			}
		}
		if err == nil && sameWatchIdentity(snap.Identity, *identity) {
			e.Snapshot = &snap
		}
	}
	// A metadata failure must preserve the keep decision. Only a confirmed
	// later write can release an exemption whose baseline was unreadable.
	idx := slices.IndexFunc(st.AutoExemptions, func(old AutoExemption) bool {
		return sameWatchIdentity(old.Identity, *identity)
	})
	if idx < 0 {
		st.AutoExemptions = append(st.AutoExemptions, e)
	} else {
		st.AutoExemptions[idx] = e
	}
}

// AutomaticExempt protects the kept object and prevents a containing unit
// from registering it indirectly. Identity lookup also survives renames.
func (s *Service) AutomaticExempt(st *State, path string, id Identity) (bool, error) {
	var scope map[objectIdentity]string
	for _, e := range st.AutoExemptions {
		if e.Identity.Serial != id.Serial {
			continue
		}
		if scope == nil {
			var err error
			scope, err = s.automaticScope(path, id)
			if err != nil {
				return false, err
			}
		}
		keptPath, relevant := scope[objectKey(e.Identity)]
		if !relevant {
			continue
		}
		unchanged, err := s.automaticObjectUnchanged(e, keptPath)
		if err != nil || unchanged {
			return unchanged, err
		}
	}
	return false, nil
}

func (s *Service) automaticObjectUnchanged(e AutoExemption, path string) (bool, error) {
	if e.Snapshot == nil {
		return true, nil
	}
	var snap WatchSnapshot
	var release func()
	var err error
	if e.Snapshot.Directory && e.Snapshot.Tree == "" {
		snap, release, err = s.FS.ObserveWatch(path, false)
	} else {
		snap, release, err = s.observeObjectTree(path, e.Identity, e.Snapshot.Directory, false, false)
	}
	if err != nil {
		return false, err
	}
	defer release()
	return sameWatchSnapshot(snap, *e.Snapshot), nil
}

// v11 has no descendant baseline. Preserve the keep decision while recording
// its first complete tree; later writes can then retire this legacy exemption.
func (s *Service) refreshAutomaticExemption(e AutoExemption, path string) (AutoExemption, bool, error) {
	unchanged, err := s.automaticObjectUnchanged(e, path)
	if err != nil || !unchanged || e.Snapshot == nil || !e.Snapshot.Directory || e.Snapshot.Tree != "" {
		return e, unchanged, err
	}
	snap, release, err := s.observeObjectTree(path, e.Identity, true, false, false)
	if err == nil {
		release()
		e.Snapshot = &snap
	}
	return e, true, err
}

// Maintenance alone locates all exemptions. A missing ID on an online,
// serial-verified volume is definite absence; every other failure keeps it.
func (s *Service) pruneAutomaticExemptions() error {
	st, err := s.Store.ReadHot()
	if err != nil || len(st.AutoExemptions) == 0 {
		return err
	}
	var events []Event
	err = s.Store.UpdateHot(func(st *State) error {
		remaining := st.AutoExemptions[:0]
		for _, e := range st.AutoExemptions {
			path, err := s.FS.Locate(e.Identity)
			if errors.Is(err, ErrNotFound) {
				continue
			}
			unchanged := true
			if err == nil {
				e, unchanged, err = s.refreshAutomaticExemption(e, path)
			}
			if err != nil {
				events = append(events, Event{Time: s.now(), Action: "automatic-exemption-retry", Path: path, Detail: err.Error()})
			}
			if err != nil || unchanged {
				remaining = append(remaining, e)
			}
		}
		st.AutoExemptions = remaining
		return nil
	})
	return errors.Join(err, s.Store.RecordCommitted(events...))
}

// ObserveAutomaticWrite consumes only a successful, identity-verified hook
// event newer than the keep decision, including writes inside kept folders.
func (s *Service) ObserveAutomaticWrite(row Observation) error {
	st, err := s.Store.ReadHot()
	if err != nil || len(st.AutoExemptions) == 0 {
		return err
	}
	return s.Store.UpdateHot(func(st *State) error {
		if err := s.CheckObservation(&row); err != nil {
			return err
		}
		var remaining []AutoExemption
		for _, e := range st.AutoExemptions {
			keep, err := s.retainAutomaticExemption(e, row)
			if err != nil {
				return err
			}
			if keep {
				remaining = append(remaining, e)
			}
		}
		st.AutoExemptions = remaining
		return nil
	})
}

func (s *Service) retainAutomaticExemption(e AutoExemption, row Observation) (bool, error) {
	if !row.ObservedAt.After(e.KeptAt) {
		return true, nil
	}
	if sameWatchIdentity(e.Identity, *row.Identity) {
		return false, nil
	}
	if e.Snapshot != nil && !e.Snapshot.Directory {
		return true, nil
	}
	for path := filepath.Dir(row.Path); ; path = filepath.Dir(path) {
		id, err := s.FS.IdentifyFollowing(path)
		if err != nil {
			return true, err
		}
		if sameWatchIdentity(e.Identity, id) {
			return false, nil
		}
		if filepath.Dir(path) == path {
			return true, nil
		}
	}
}
