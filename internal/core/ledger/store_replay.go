package ledger

import (
	"bytes"
	"errors"
	"os"
	"sort"
	"sync"
)

// replayCache is the hot ledger as the resident last committed it, with the
// exact bytes it wrote. A later batch reuses the decoded state only while
// store.json still holds those bytes: other processes may replace a v18 file, so
// a modification time alone cannot prove a hit.
type replayCache struct {
	mu    sync.Mutex
	wire  []byte
	state State
}

func (c *replayCache) put(wire []byte, st State) {
	c.mu.Lock()
	c.wire, c.state = wire, st
	c.mu.Unlock()
}

// take hands the cached state to exactly one caller, and only when raw is the
// very bytes it was committed as. A failed or conflicting callback may already
// have changed a map or slice the cached copy shares, so the cache is spent
// whatever the answer.
func (c *replayCache) take(raw []byte) (State, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	st, ok := c.state, c.wire != nil && bytes.Equal(raw, c.wire)
	c.wire, c.state = nil, State{}
	return st, ok
}

// WithResidentReplay returns a Store whose ReplayBatch keeps the last committed
// state in memory. Only the resident's single observation consumer uses it.
func (s Store) WithResidentReplay() Store { s.resident = new(replayCache); return s }

// ResidentReplay reports whether s was made by WithResidentReplay.
func (s Store) ResidentReplay() bool { return s.resident != nil }

var (
	// errReplayConflict: another process committed while a batch ran unlocked.
	errReplayConflict = errors.New("ledger changed during observation replay")
	// errReplayLocked: the ledger needs migration or a first graveyard snapshot,
	// which only run under store.lock.
	errReplayLocked = errors.New("ledger needs a locked replay")
)

// replayOptimisticTries bounds how often a batch is redone after a conflict
// before it takes the lock for the whole batch, which always makes progress.
const replayOptimisticTries = 3

// ReplayBatch runs fn against a private copy of the hot ledger and commits all
// of its changes with one durable write. fn does the slow work (Git, identity
// checks) with no lock held, so it never queues another process; only the
// commit takes store.lock, and it is refused if store.json changed since the
// copy was read. fn may therefore run more than once and must only change the
// Store it is given. On error nothing is committed and the caller must not
// acknowledge any request in the batch.
func (s Store) ReplayBatch(fn func(Store) error) error {
	for range replayOptimisticTries {
		err := s.replayUnlocked(fn)
		if errors.Is(err, errReplayLocked) {
			break
		}
		if !errors.Is(err, errReplayConflict) {
			return err
		}
	}
	return s.withLock("store.lock", true, func() error { return s.replayLocked(fn) })
}

func (s Store) replayUnlocked(fn func(Store) error) error {
	raw, err := readShared(s.statePath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	st, ok, err := s.replayBase(raw)
	if err != nil {
		return err
	}
	if !ok {
		return errReplayLocked
	}
	if err := s.replayRun(&st, fn); err != nil {
		return err
	}
	return s.withLock("store.lock", true, func() error {
		current, err := readShared(s.statePath())
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		if !bytes.Equal(current, raw) {
			return errReplayConflict
		}
		return s.replayCommit(st, nil)
	})
}

func (s Store) replayLocked(fn func(Store) error) error {
	st, events, err := s.loadHot()
	if err != nil {
		return err
	}
	s.resident.take(nil)
	if err := s.replayRun(&st, fn); err != nil {
		return err
	}
	return s.replayCommit(st, events)
}

// replayBase decodes the bytes just read, from the cache when they are the
// bytes the resident itself committed. ok is false when the ledger would need
// migration or has no graveyard snapshot yet.
func (s Store) replayBase(raw []byte) (State, bool, error) {
	if len(raw) == 0 {
		return State{}, false, nil // no ledger yet: the first write runs locked
	}
	st, hit := s.resident.take(raw)
	if !hit {
		var events []Event
		var err error
		if st, events, err = s.decodeHot(raw); err != nil || len(events) != 0 {
			return State{}, false, err
		}
	}
	return st, st.Version == stateVersion && st.Graveyard != nil, nil
}

func (s Store) replayRun(st *State, fn func(Store) error) error {
	if st.Version < stateVersion && !st.graveyardLoaded {
		if err := s.loadGraveyard(st); err != nil {
			return err
		}
	}
	guard := guardHotGraveyard(st)
	boundary, err := hotBoundary(*st)
	if err != nil {
		return err
	}
	tx := s
	tx.replayState = st
	if err := fn(tx); err != nil {
		return err
	}
	return checkHotMutation(*st, boundary, guard)
}

func (s Store) replayCommit(st State, events []Event) error {
	if err := s.Context().Err(); err != nil {
		return err
	}
	if st.Graveyard == nil || st.Version < stateVersion {
		if err := s.save(st); err != nil {
			return err
		}
	} else {
		sort.Slice(st.Items, func(i, j int) bool { return st.Items[i].ExpiresAt.Before(st.Items[j].ExpiresAt) })
		s.rememberReplay = true
		if err := s.saveHot(st, false); err != nil {
			return err
		}
	}
	return s.RecordCommitted(events...)
}
