package ledger

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"sort"
)

func newDataID() string { return rand.Text() }

func validateHotLedger(st State) error {
	if st.Version < 17 {
		if st.Graveyard != nil || st.PreviousGraveyard != nil || st.GraveyardIndex != nil {
			return errors.New("legacy store contains a v17 graveyard reference")
		}
		return nil
	}
	if st.DataID == "" || !validGraveyardReference(st.Graveyard) || st.GraveyardIndex == nil {
		return errors.New("v17 store requires an owned graveyard reference and safety index")
	}
	if len(st.Burials) != 0 || len(st.RestoreCleanups) != 0 {
		return errors.New("v17 hot store contains inline graveyard evidence")
	}
	if st.PreviousGraveyard != nil && !validGraveyardReference(st.PreviousGraveyard) {
		return errors.New("invalid previous graveyard reference")
	}
	idx := st.GraveyardIndex
	if idx.MaxID < 0 || idx.ReservedAudit < 0 || !slices.IsSorted(idx.Keys) {
		return errors.New("invalid graveyard safety index")
	}
	return nil
}

// ReadHot omits cold records. Graveyard selection, scheduling and destructive
// operations must use ReadSnapshot/Read; safety-sensitive hot transactions
// load the referenced snapshot under their existing store lock on an index hit.
func (s Store) ReadHot() (State, error) {
	var st State
	err := s.withLock("store.lock", false, func() error {
		var err error
		st, _, err = s.loadHot()
		return err
	})
	return st, err
}

func hotBoundary(st State) ([]byte, error) {
	return json.Marshal(struct {
		Version  int
		DataID   string
		Ref      *GraveyardReference
		Previous *GraveyardReference
		Index    *graveyardIndex
		Intents  []ContainerIntent
		GCCursor string
	}{st.Version, st.DataID, st.Graveyard, st.PreviousGraveyard, st.GraveyardIndex, st.ContainerIntents, st.GraveyardGCCursor})
}

type hotGraveyardGuard struct {
	burial  *Burial
	cleanup *RestoreCleanup
}

func guardHotGraveyard(st *State) hotGraveyardGuard {
	if st.graveyardLoaded {
		return hotGraveyardGuard{}
	}
	// Non-nil empty slices make an attempted clear observable too, without
	// exposing fake records or loading the cold ledger on every hot write.
	st.Burials, st.RestoreCleanups = make([]Burial, 0, 1), make([]RestoreCleanup, 0, 1)
	return hotGraveyardGuard{&st.Burials[:1][0], &st.RestoreCleanups[:1][0]}
}

func checkHotMutation(st State, boundary []byte, guard hotGraveyardGuard) error {
	after, err := hotBoundary(st)
	if err != nil {
		return err
	}
	changed := len(st.Burials) != 0 || cap(st.Burials) != 1 || &st.Burials[:1][0] != guard.burial ||
		len(st.RestoreCleanups) != 0 || cap(st.RestoreCleanups) != 1 || &st.RestoreCleanups[:1][0] != guard.cleanup
	if st.graveyardLoaded {
		cold, err := graveyardFingerprint(st)
		if err != nil {
			return err
		}
		changed = cold != st.graveyardOriginal
	}
	if changed || !bytes.Equal(boundary, after) {
		return errors.New("hot transaction changed graveyard evidence; use UpdateGraveyard")
	}
	return nil
}

func (s Store) updateHot(fn func(*State) error) error {
	return s.withLock("store.lock", true, func() error {
		st, events, err := s.loadHot()
		if err != nil {
			return err
		}
		guard := guardHotGraveyard(&st)
		before, err := marshalHot(st)
		if err != nil {
			return err
		}
		boundary, err := hotBoundary(st)
		if err != nil {
			return err
		}
		if err := fn(&st); err != nil {
			return err
		}
		if err := checkHotMutation(st, boundary, guard); err != nil {
			return err
		}
		return s.commitHot(st, before, events)
	})
}

func (s Store) commitHot(st State, before []byte, events []Event) error {
	after, err := marshalHot(st)
	if err != nil {
		return err
	}
	if st.Version == stateVersion && len(events) == 0 && bytes.Equal(before, after) {
		if _, err := os.Stat(s.statePath()); err == nil {
			return nil
		}
	}
	if err := s.Context().Err(); err != nil {
		return err
	}
	if st.Graveyard == nil || st.Version < 17 {
		err = s.save(st)
	} else {
		sort.Slice(st.Items, func(i, j int) bool { return st.Items[i].ExpiresAt.Before(st.Items[j].ExpiresAt) })
		if s.auditPending != nil && len(st.PendingAudit) > 0 {
			*s.auditPending = true
		}
		err = s.saveHot(st, false)
	}
	if err != nil {
		return err
	}
	return s.RecordCommitted(events...)
}

// Diagnostics create no locks. A retired snapshot can race an unlocked reader
// after several commits; retry only when the hot reference actually changed.
func (s Store) inspectSnapshot() (State, error) {
	for range 4 {
		st, _, err := s.loadHot()
		if err != nil {
			return State{}, err
		}
		err = s.loadGraveyard(&st)
		if err == nil {
			st, _, err = s.normalizeState(st)
			return st, err
		}
		current, _, readErr := s.loadHot()
		if readErr != nil || sameGraveyardReference(st.Graveyard, current.Graveyard) {
			return State{}, errors.Join(err, readErr)
		}
	}
	return State{}, errors.New("graveyard snapshot changed repeatedly; retry inspection")
}

func sameGraveyardReference(a, b *GraveyardReference) bool {
	return a == nil && b == nil || a != nil && b != nil && *a == *b
}
