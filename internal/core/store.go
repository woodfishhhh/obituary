package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Store persists State as JSON under a home directory. Every mutation runs
// under an exclusive lock file so the CLI and the daemon can share it.
type Store struct {
	changed      *bool // request-local commit tracking for presentation callbacks
	Home         string
	lockDeadline time.Time
	ctx          context.Context
	auditPending *bool
	auditLines   *int                // batch delivery upper bound for recovery log advice
	observeLock  func(time.Duration) // serial test measurement of store.lock ownership
}

func (s Store) WithContext(ctx context.Context) Store { s.ctx = ctx; return s }

func (s Store) Context() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}

// WithDeadline returns a copy with a shared deadline for all lock waits in
// an operation. Reads use a shared lock and defer migration writes to the
// next unrestricted read/update. Already-running transactions still finish.
func (s Store) WithDeadline(deadline time.Time) Store {
	s.lockDeadline = deadline
	return s
}

// Version 5 keeps interrupted burials in independent graveyard manifests.
// Unpublished v4 intent fields are deliberately ignored when decoding.
// Version 6 records original parent identities for safe resurrection. Old
// records remain readable; their unrecorded reparse ancestors are refused.
// Version 7 (unreleased) retains locating cleanup receipts for both restore outcomes.
// Version 8 preserves explicit deletion batches. v7 silently drops unknown
// fields on write, so reject downgrade instead of losing --last's scope.
// Legacy records remain readable and ungrouped; the next write upgrades them.
// Version 9 preserves folder policies and their registration/approval links.
// Version 10 preserves watch stability, original age basis, keep exemptions and
// nested exclusions. Legacy watch items acquire a fresh stability window before
// moving; unknown age uses the current approved policy and never shortens expiry.
// Version 11 preserves keep decisions until a subsequent write. Older stores
// have no automatic exemptions and retain their existing watch exemptions.
// Version 12 binds folder-policy approval to its explicit approved revision.
// v11 may have inherited implicit v9/v10 consent, so no unproven approval survives.
// Version 13 preserves both hook request receipts across queue acknowledgement
// and separate explicit approvals for watch directory burial. Older writers
// would discard these fields, replaying completed requests or moving directories
// automatically. Both v13 additions were unpublished and share this version.
// Version 14 preserves Docker container identities and object kinds. Older
// writers must refuse this version to keep containers out of file operations.
// Version 15 interns parent chains in the ledger; manifests stay self-contained.
// Version 16 interns operations, origins and identity volumes only in the ledger.
// Version 17 commits an immutable graveyard snapshot through the hot ledger.
const stateVersion = 17

func (s Store) statePath() string { return filepath.Join(s.Home, "store.json") }

// Read normally persists legacy identity merges under the write lock.
// WithDeadline uses a shared read and leaves migration writes for later.
func (s Store) Read() (State, error) {
	var state State
	err := s.withLock("store.lock", s.lockDeadline.IsZero(), func() error {
		var err error
		if s.lockDeadline.IsZero() {
			state, err = s.loadMigrated()
		} else {
			state, _, err = s.load()
		}
		return err
	})
	return state, err
}

// ReadSnapshot holds the shared store lock without migrations or writes.
func (s Store) ReadSnapshot() (State, error) {
	var state State
	err := s.withLock("store.lock", false, func() error {
		var err error
		state, _, err = s.load()
		return err
	})
	return state, err
}

// Inspect reads a snapshot without creating a directory or lock file. It is
// intended for read-only diagnostics, never for a read-modify-write operation.
func (s Store) Inspect() (State, error) {
	return s.inspectSnapshot()
}

// Update loads the state, applies fn and saves the result durably. If fn
// returns an error nothing is written.
func (s Store) Update(fn func(*State) error) error {
	return s.UpdateGraveyard(fn)
}

// UpdateHot changes only the active ledger.
func (s Store) UpdateHot(fn func(*State) error) error { return s.updateHot(fn) }

// UpdateGraveyard atomically changes the hot ledger and graveyard together.
func (s Store) UpdateGraveyard(fn func(*State) error) error { return s.update(fn, false) }

// Confirm revalidates and durably rewrites recovery evidence before retiring
// its independent manifest, even when the logical state is unchanged.
func (s Store) Confirm(fn func(*State) error) error {
	return s.update(fn, true)
}

func (s Store) update(fn func(*State) error, confirm bool) error {
	return s.withLock("store.lock", true, func() error {
		state, events, err := s.load()
		if err != nil {
			return err
		}
		before, err := marshalLedger(state)
		if err != nil {
			return err
		}
		originalRef, originalPrevious := copyGraveyardReference(state.Graveyard), copyGraveyardReference(state.PreviousGraveyard)
		originalDigest := state.graveyardOriginal
		if err := fn(&state); err != nil {
			return err
		}
		// References are transaction-owned, including when a caller replaces
		// the whole State while importing a complete snapshot.
		state.Graveyard, state.PreviousGraveyard = originalRef, originalPrevious
		state.graveyardOriginal = originalDigest
		after, err := marshalLedger(state)
		if err != nil {
			return err
		}
		if !confirm && state.Version == stateVersion && len(events) == 0 && bytes.Equal(before, after) {
			if _, err := os.Stat(s.statePath()); err == nil {
				return nil
			}
		}
		if err := s.Context().Err(); err != nil {
			return err
		}
		if confirm {
			if err := s.confirmGraveyard(state); err != nil {
				return err
			}
		}
		if err := s.save(state); err != nil {
			return err
		}
		return s.RecordCommitted(events...)
	})
}

// ModTime reports when the state file last changed (zero if it does not exist).
func (s Store) ModTime() time.Time {
	info, err := os.Stat(s.statePath())
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

func (s Store) withLock(name string, exclusive bool, fn func() error) error {
	ctx := s.Context()
	if !s.lockDeadline.IsZero() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, s.lockDeadline)
		defer cancel()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(s.Home, 0o700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	var unlock func()
	var err error
	if ctx.Done() == nil {
		unlock, err = lockFile(filepath.Join(s.Home, name), exclusive)
	} else {
		unlock, err = lockFileContext(ctx, filepath.Join(s.Home, name), exclusive)
	}
	if err != nil {
		return fmt.Errorf("lock %s: %w", name, err)
	}
	defer unlock()
	if name == "store.lock" && s.observeLock != nil {
		started := time.Now()
		defer func() { s.observeLock(time.Since(started)) }()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return fn()
}

func (s Store) loadMigrated() (State, error) {
	state, events, err := s.load()
	if err != nil || len(events) == 0 {
		return state, err
	}
	if err := s.Context().Err(); err != nil {
		return State{}, err
	}
	if err := s.save(state); err != nil {
		return State{}, err
	}
	return state, s.RecordCommitted(events...)
}

func (s Store) load() (State, []Event, error) {
	state, events, err := s.loadHot()
	if err != nil {
		return State{}, nil, err
	}
	if err := s.loadGraveyard(&state); err != nil {
		return State{}, nil, err
	}
	state, more, err := s.normalizeState(state)
	return state, append(events, more...), err
}

func (s Store) loadHot() (State, []Event, error) {
	data, err := readShared(s.statePath())
	if errors.Is(err, os.ErrNotExist) {
		return State{Version: stateVersion, NextID: 1}, nil, nil
	}
	if err != nil {
		return State{}, nil, fmt.Errorf("read store: %w", err)
	}
	state, err := unmarshalLedger(data)
	if newer := (newerStoreError{}); errors.As(err, &newer) {
		return State{}, nil, err
	}
	if err != nil {
		return State{}, nil, fmt.Errorf("store %s is corrupt: %w", s.statePath(), err)
	}
	if err := validateHotLedger(state); err != nil {
		return State{}, nil, err
	}
	state, events, err := s.normalizeState(state)
	if err != nil {
		return State{}, nil, err
	}
	if state.Version < 17 {
		state.graveyardLoaded = true
		state.graveyardOriginal, err = graveyardFingerprint(state)
		if err != nil {
			return State{}, nil, err
		}
	}
	return state, events, nil
}

func (s Store) normalizeState(state State) (State, []Event, error) {
	if err := validateContainerState(state); err != nil {
		return State{}, nil, fmt.Errorf("store has invalid object kind: %w", err)
	}
	if err := s.normalizeIDs(&state); err != nil {
		return State{}, nil, err
	}
	events := mergeItemIdentities(&state)
	events = append(events, migrateWatchConsent(&state)...)
	if s.auditPending != nil && len(state.PendingAudit) > 0 {
		*s.auditPending = true
	}
	return state, events, nil
}

func (s Store) normalizeIDs(state *State) error {
	seen := make(map[int64]bool, len(state.Items)+len(state.Burials))
	ids := make([]int64, 0, len(state.Items)+len(state.Burials))
	for _, item := range state.Items {
		if item.Identity != nil {
			*item.Identity = item.Identity.Normalized()
		}
		ids = append(ids, item.ID)
	}
	for _, b := range state.Burials {
		for _, id := range []*Identity{b.Identity, b.GraveID} {
			if id != nil {
				*id = id.Normalized()
			}
		}
		ids = append(ids, b.ID)
	}
	for i := range state.Proposals {
		p := &state.Proposals[i]
		for _, c := range p.Candidates {
			p.CandidateSeq = max(p.CandidateSeq, c.Rev)
		}
		ids = append(ids, p.ID)
	}
	for i := range state.Watches {
		state.Watches[i].Identity = state.Watches[i].Identity.Normalized()
		ids = append(ids, state.Watches[i].ID)
	}
	for _, id := range ids {
		if id < 1 || seen[id] {
			return fmt.Errorf("store %s is corrupt: invalid or duplicate id #%d", s.statePath(), id)
		}
		seen[id] = true
		state.NextID = max(state.NextID, id+1)
	}
	state.NextID = max(state.NextID, 1)
	if state.GraveyardIndex != nil {
		state.NextID = max(state.NextID, state.GraveyardIndex.MaxID+1)
	}
	return nil
}

func mergeItemIdentities(state *State) []Event {
	var events []Event
	byIdentity := make(map[Identity]int)
	items := make([]Item, 0, len(state.Items))
	for _, item := range state.Items {
		if item.Identity == nil {
			items = append(items, item)
			continue
		}
		if idx, ok := byIdentity[*item.Identity]; ok {
			keep, event := mergeDuplicateItem(items[idx], item)
			items[idx] = keep
			events = append(events, event)
			continue
		}
		byIdentity[*item.Identity] = len(items)
		items = append(items, item)
	}
	state.Items = items
	return events
}

func mergeDuplicateItem(keep, other Item) (Item, Event) {
	if other.ID < keep.ID {
		keep, other = other, keep
	}
	if other.ExpiresAt.After(keep.ExpiresAt) {
		keep.ExpiresAt = other.ExpiresAt
	}
	if automaticSource(keep.Source) && !automaticSource(other.Source) {
		keep.Source = other.Source
	}
	return keep, Event{Time: time.Now(), Action: "merged", ID: keep.ID, Path: keep.Path,
		Detail: fmt.Sprintf("merged duplicate identity #%d into #%d; expires %s", other.ID, keep.ID, keep.ExpiresAt.Format(time.RFC3339))}
}

func (s Store) save(state State) error {
	migrating := state.Version < stateVersion
	if err := s.prepareLedgerMigration(&state); err != nil {
		return err
	}
	if err := s.prepareDataOwner(&state); err != nil {
		return err
	}
	// Snapshot ownership is mandatory even if the optional uninstall marker
	// could not be written. The marker still independently controls uninstall.
	if state.DataID == "" {
		state.DataID = newDataID()
	}
	if s.auditPending != nil && len(state.PendingAudit) > 0 {
		*s.auditPending = true
	}
	state.Version = stateVersion
	sort.Slice(state.Items, func(i, j int) bool { return state.Items[i].ExpiresAt.Before(state.Items[j].ExpiresAt) })
	changed, err := graveyardChanged(state)
	if err != nil {
		return err
	}
	if changed {
		if err := s.writeGraveyard(&state, migrating); err != nil {
			return err
		}
	} else {
		state.GraveyardIndex = buildGraveyardIndex(state)
	}
	return s.saveHot(state, changed)
}

func (s Store) saveHot(state State, graveyardChanged bool) error {
	data, err := marshalHot(state)
	if err != nil {
		return err
	}
	err = writeDurably(s.statePath(), append(data, '\n'))
	var unconfirmed *unconfirmedCommit
	if err == nil || errors.As(err, &unconfirmed) {
		// The new state is visible on disk even when its flush is unconfirmed.
		if s.changed != nil {
			*s.changed = true
		}
		if graveyardChanged {
			s.writeGraveNoticeIndex(state)
		}
	}
	if err == nil && graveyardChanged {
		s.collectGraveyardSnapshots(state)
	}
	return err
}

// writeDurably writes to a temporary file, flushes it to disk and then
// replaces path, so a crash leaves either the old or the new content.
func writeDurably(path string, data []byte) error {
	// Go's os.Root.Rename uses handle-relative Renameat on Windows, including
	// POSIX replacement and its fallback. Both names use the same parent.
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return fmt.Errorf("write store: %w", err)
	}
	defer root.Close()
	name := filepath.Base(path)
	tmp := name + ".tmp"
	f, err := root.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("write store: %w", err)
	}
	if _, err := writeStoreBytes(f, data); err != nil {
		f.Close()
		return fmt.Errorf("write store: %w", err)
	}
	if err := syncFile(f); err != nil {
		f.Close()
		return fmt.Errorf("flush store: %w", err)
	}
	return replaceDurably(root, tmp, name, f)
}

// syncReplacement is replaceDurably's post-rename failure injection seam.
var syncReplacement = func(file *os.File) error { return syncFile(file) }

var writeStoreBytes = (*os.File).Write

var renameStore = func(root *os.Root, tmp, name string) error { return root.Rename(tmp, name) }

type unconfirmedCommit struct{ cause error }

func (e *unconfirmedCommit) Error() string {
	return "flush replaced file (commit unconfirmed): " + e.cause.Error()
}

func (e *unconfirmedCommit) Unwrap() error { return e.cause }

// replaceDurably owns f. Go 1.26 internal/syscall/windows/at_windows.go
// Renameat does not write through rename metadata. Microsoft File Caching
// (learn.microsoft.com/windows/win32/fileio/file-caching) requires
// FlushFileBuffers for metadata; Sync the same write handle after Root.Rename.
// A failed flush means commit is unconfirmed, so callers retain recovery data.
func replaceDurably(root *os.Root, tmp, name string, f *os.File) (err error) {
	defer func() {
		err = errors.Join(err, f.Close())
	}()
	if err := renameStore(root, tmp, name); err != nil {
		return fmt.Errorf("replace store: %w", err)
	}
	if err := syncReplacement(f); err != nil {
		return &unconfirmedCommit{cause: err}
	}
	if err := syncParentDirectory(root); err != nil {
		return &unconfirmedCommit{cause: err}
	}
	return nil
}
