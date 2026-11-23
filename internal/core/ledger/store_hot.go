package ledger

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"time"
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

// ReadActive is the read-only view used by commands that only present active
// entries. Legacy stores and pending normalisation still use the full read so
// migration and audit semantics remain unchanged.
func (s Store) ReadActive() (State, error) {
	var st State
	var fallback bool
	err := s.withLock("store.lock", false, func() error {
		var events []Event
		var err error
		st, events, err = s.loadHot()
		if err != nil {
			return err
		}
		fallback = st.Version < stateVersion || len(events) != 0
		if !fallback && st.Graveyard != nil {
			if err := s.verifyGraveyardReference(st); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		full, fullErr := s.Read()
		if fullErr != nil {
			return State{}, fullErr
		}
		return full, nil
	}
	if fallback {
		return s.Read()
	}
	return st, nil
}

// GraveyardSummary contains only the fields needed by stats and summary UI.
type GraveyardSummary struct {
	Count     int
	Bytes     int64
	NextPurge time.Time
}

// ReadSummary avoids unpacking cold burial evidence. It still verifies the
// immutable snapshot and falls back to Read for legacy or migrating stores.
func (s Store) ReadSummary() (State, GraveyardSummary, error) {
	var st State
	var summary GraveyardSummary
	var fallback bool
	err := s.withLock("store.lock", false, func() error {
		var events []Event
		var err error
		st, events, err = s.loadHot()
		if err != nil {
			return err
		}
		fallback = st.Version < stateVersion || len(events) != 0
		if fallback || st.Graveyard == nil {
			return nil
		}
		summary, err = s.readGraveyardSummary(st)
		return err
	})
	if err != nil {
		full, fullErr := s.Read()
		if fullErr != nil {
			return State{}, GraveyardSummary{}, fullErr
		}
		return full, summaryFromState(full), nil
	}
	if fallback {
		full, err := s.Read()
		if err != nil {
			return State{}, GraveyardSummary{}, err
		}
		return full, summaryFromState(full), nil
	}
	return st, summary, nil
}

type graveyardSummarySnapshot struct {
	Owner           string                    `json:"owner"`
	Version         int                       `json:"version"`
	DataID          string                    `json:"dataId"`
	Generation      string                    `json:"generation"`
	Burials         []graveyardSummaryBurial  `json:"burials,omitempty"`
	RestoreCleanups []graveyardSummaryCleanup `json:"restoreCleanups,omitempty"`
	Volumes         []*ledgerVolume           `json:"volumes,omitempty"`
}

type graveyardSummaryBurial struct {
	Grave       string
	IsDir       bool
	BuriedAt    time.Time
	Reason      string
	Note        string
	Source      string
	GraveID     *ledgerIdentity
	Kind        string          `json:"kind,omitempty"`
	ID          int64           `json:"id"`
	Path        string          `json:"path"`
	Identity    *ledgerIdentity `json:"identity,omitempty"`
	Size        int64           `json:"size"`
	PurgeAt     time.Time       `json:"purgeAt"`
	Contains    []int64         `json:"contains,omitempty"`
	PurgeIntent *PurgeIntent    `json:"purgeIntent,omitempty"`
}

type graveyardSummaryCleanup struct {
	Burial graveyardSummaryBurial `json:"burial"`
}

func summaryFromState(st State) GraveyardSummary {
	var summary GraveyardSummary
	for _, b := range st.Burials {
		summary.Count++
		summary.Bytes += b.Size
		if summary.Count == 1 || b.PurgeAt.Before(summary.NextPurge) {
			summary.NextPurge = b.PurgeAt
		}
	}
	return summary
}

func (s Store) readGraveyardSummary(st State) (GraveyardSummary, error) {
	data, err := readGraveyardSnapshot(filepath.Join(s.Home, st.Graveyard.File))
	if err != nil {
		return GraveyardSummary{}, fmt.Errorf("read referenced graveyard snapshot: %w", err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(data)) != st.Graveyard.Hash {
		return GraveyardSummary{}, errors.New("invalid referenced graveyard snapshot: graveyard snapshot hash or reference mismatch")
	}
	var snap graveyardSummarySnapshot
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snap); err != nil {
		return GraveyardSummary{}, fmt.Errorf("invalid referenced graveyard snapshot: %w", err)
	}
	if len(bytes.Trim(data[decoder.InputOffset():], " \t\n\r")) != 0 {
		return GraveyardSummary{}, errors.New("trailing data in graveyard snapshot")
	}
	if snap.Owner != graveyardSnapshotOwner || snap.Version != stateVersion || snap.DataID == "" || snap.DataID != st.DataID || snap.Generation != st.Graveyard.Generation {
		return GraveyardSummary{}, errors.New("invalid referenced graveyard snapshot: graveyard snapshot version, data owner or generation mismatch")
	}
	if err := validateSummaryIDs(s, st, snap); err != nil {
		return GraveyardSummary{}, err
	}
	if err := validateSummaryIndex(st, snap); err != nil {
		return GraveyardSummary{}, err
	}
	var summary GraveyardSummary
	for _, b := range snap.Burials {
		summary.Count++
		summary.Bytes += b.Size
		if summary.Count == 1 || b.PurgeAt.Before(summary.NextPurge) {
			summary.NextPurge = b.PurgeAt
		}
	}
	return summary, nil
}

func validateSummaryIndex(st State, snap graveyardSummarySnapshot) error {
	idx := &graveyardIndex{}
	wire := ledgerState{plainState: &plainState{Version: stateVersion}, Volumes: snap.Volumes}
	add := func(b graveyardSummaryBurial) error {
		if _, err := wire.unpackIdentity(b.GraveID); err != nil {
			return err
		}
		if b.Kind != "" {
			return errors.New("object kind requires full graveyard validation")
		}
		identity, err := wire.unpackIdentity(b.Identity)
		if err != nil {
			return err
		}
		idx.add(Burial{ID: b.ID, Path: b.Path, Identity: identity, Contains: b.Contains})
		return nil
	}
	for _, b := range snap.Burials {
		if err := add(b); err != nil {
			return fmt.Errorf("invalid referenced graveyard snapshot: %w", err)
		}
		if b.PurgeIntent != nil && FindContainerIntent(&st, b.ID) == nil {
			idx.ReservedAudit++
		}
	}
	for _, c := range snap.RestoreCleanups {
		if err := add(c.Burial); err != nil {
			return fmt.Errorf("invalid referenced graveyard snapshot: %w", err)
		}
	}
	idx.compact()
	if !reflect.DeepEqual(idx, st.GraveyardIndex) {
		return errors.New("graveyard safety index does not match committed snapshot")
	}
	return nil
}

func (s Store) verifyGraveyardReference(st State) error {
	_, err := s.readGraveyardSummary(st)
	return err
}

func validateSummaryIDs(s Store, st State, snap graveyardSummarySnapshot) error {
	seen := make(map[int64]struct{}, len(st.Items)+len(st.Proposals)+len(st.Watches))
	add := func(id int64) error {
		if id < 1 {
			return fmt.Errorf("store %s is corrupt: invalid or duplicate id #%d", s.statePath(), id)
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("store %s is corrupt: invalid or duplicate id #%d", s.statePath(), id)
		}
		seen[id] = struct{}{}
		return nil
	}
	for _, item := range st.Items {
		if err := add(item.ID); err != nil {
			return err
		}
	}
	for _, proposal := range st.Proposals {
		if err := add(proposal.ID); err != nil {
			return err
		}
	}
	for _, watch := range st.Watches {
		if err := add(watch.ID); err != nil {
			return err
		}
	}
	for _, burial := range snap.Burials {
		if err := add(burial.ID); err != nil {
			return err
		}
	}
	for _, cleanup := range snap.RestoreCleanups {
		if err := add(cleanup.Burial.ID); err != nil {
			return err
		}
	}
	return nil
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
		if fence, err := s.readSQLiteFence(); err != nil {
			return err
		} else if fence != nil && s.persistentSQLite {
			session, err := s.sqliteHotSession()
			if err != nil && !errors.Is(err, errSQLiteSessionFallback) {
				return err
			}
			if err == nil {
				if err := s.updateHotSQLiteSession(session, *fence, fn); !errors.Is(err, errSQLiteSessionFallback) {
					return err
				}
			}
		}
		st, events, err := s.loadHot()
		if err != nil {
			return err
		}
		if st.Version < stateVersion && !st.graveyardLoaded {
			if err := s.loadGraveyard(&st); err != nil {
				return err
			}
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
	if st.Graveyard == nil || st.Version < stateVersion {
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
