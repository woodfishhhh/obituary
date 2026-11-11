package core

import (
	"errors"
	"path/filepath"
	"slices"
	"time"
)

// WatchExemption records an explicit user keep decision for one policy/object.
type WatchExemption struct {
	WatchID  int64     `json:"watchId"`
	Identity Identity  `json:"identity"`
	Path     string    `json:"path,omitempty"`
	Reason   string    `json:"reason,omitempty"`
	KeptAt   time.Time `json:"keptAt,omitempty"`
}

func sameWatchIdentity(a, b Identity) bool {
	return a.Serial == b.Serial && a.FileID == b.FileID
}

func watchByID(st *State, id int64) WatchPolicy {
	for _, p := range st.Watches {
		if p.ID == id {
			return p
		}
	}
	return WatchPolicy{}
}

func (st *State) watchExempt(id int64, identity Identity) bool {
	return slices.ContainsFunc(st.WatchExemptions, func(e WatchExemption) bool {
		return e.WatchID == id && sameWatchIdentity(e.Identity, identity)
	})
}

func (st *State) spareWatch(id int64, identity *Identity, path, reason string, at time.Time) {
	if id == 0 || watchByID(st, id).ID == 0 || identity == nil || st.watchExempt(id, *identity) {
		return
	}
	st.WatchExemptions = append(st.WatchExemptions, WatchExemption{WatchID: id, Identity: *identity, Path: path, Reason: reason, KeptAt: at})
}

// Additional matching policies cover explicit burials and migrated v9 graves.
// A lexical match is conservative when an ancestor is temporarily unreadable.
func (s *Service) spareWatchObject(st *State, policyID int64, identity *Identity, path, reason string) {
	st.spareWatch(policyID, identity, path, reason, s.now())
	if identity == nil {
		return
	}
	for parent := filepath.Dir(path); ; parent = filepath.Dir(parent) {
		id, err := s.FS.IdentifyFollowing(parent)
		for _, p := range st.Watches {
			if err == nil && sameWatchIdentity(id, p.Identity) || samePath(parent, p.Path) {
				st.spareWatch(p.ID, identity, path, reason, s.now())
			}
		}
		if parent == filepath.Dir(parent) {
			break
		}
	}
}

func watchExemptionRetry(err error) error {
	// Losing the exemption's proof must keep the candidate's lifespan for retry,
	// including ErrUnsafe from an ancestor or a newly hard-linked kept object.
	return errors.New("cannot verify watch exemption; retry: " + err.Error())
}

func (st *State) spareRestoredWatch(b Burial, path string, at time.Time) {
	st.spareWatch(b.WatchID, b.Identity, path, "restored", at)
	for _, child := range b.Children {
		st.spareWatch(child.WatchID, child.Identity, "", "restored", at)
	}
}

// WatchExemptionDetails resolves identity-based legacy entries without changing
// their keep decisions. Missing historical reasons and dates stay unknown.
func (s *Service) WatchExemptionDetails(st State, policyID int64) []WatchExemption {
	rows := make([]WatchExemption, 0)
	for _, e := range st.WatchExemptions {
		if e.WatchID != policyID {
			continue
		}
		if path, err := s.FS.Locate(e.Identity); err == nil {
			e.Path = path
		}
		if e.Reason == "" {
			e.Reason = "unknown (legacy exemption)"
		}
		rows = append(rows, e)
	}
	return rows
}
