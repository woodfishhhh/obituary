package core

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// AddOptions configure Add.
type AddOptions struct {
	ExpiresAt    time.Time
	OnlyNew      bool // automatic registration must not alter or overlap a lifespan
	Force        bool
	Note         string
	Source       string // defaults to "cli"
	Project      string
	Session      string
	Observed     *Observation // original hook write, checked inside the transaction
	BeforeCommit func() error // external approval snapshot check under the write lock; must not access Store
}

// MaxMetadataBytes applies equally to CLI, MCP and automatic registrations.
const MaxMetadataBytes = 4 << 10

// NormalizeSession keeps external session identifiers bounded and stable
// across hook events and approval of proposals written by older versions.
func NormalizeSession(session string) string {
	if len(session) <= MaxMetadataBytes {
		return session
	}
	return fmt.Sprintf("sha256:%x", sha256.Sum256([]byte(session)))
}

func ValidateMetadata(field, value string) error {
	if len(value) > MaxMetadataBytes {
		return fmt.Errorf("%s exceeds %d bytes", field, MaxMetadataBytes)
	}
	return nil
}

type candidate struct {
	path     string
	isDir    bool
	identity Identity
}

// Add gives each path a lifespan. The same object (even under an older name)
// updates its record. A replacement only reuses a path's record when the old
// object is definitely gone, and starts with fresh registration metadata.
// Inspection and conflict failures stay in their result slot; metadata,
// expiry, approval and ledger failures abort the batch without result receipts.
// A *AuditError means the changes were saved but not logged.
func (s *Service) Add(paths []string, opts AddOptions) ([]Result, error) {
	s, notify := s.trackChanges()
	defer notify()
	for field, value := range map[string]string{"note": opts.Note, "session": opts.Session, "source": opts.Source} {
		if err := ValidateMetadata(field, value); err != nil {
			return nil, err
		}
	}
	now := s.now()
	if opts.Source == "" {
		opts.Source = "cli"
	}
	if !opts.ExpiresAt.After(now) {
		return nil, fmt.Errorf("expiry must be in the future")
	}
	results, candidates := s.inspectRegistrations(paths, opts.Force)
	var events []Event
	err := s.Store.UpdateHot(func(st *State) error {
		if st.ObservationCompleted(opts.Observed) {
			return nil
		}
		roots := s.resolveGraveyardRoots(*st, paths...)
		for i, c := range candidates {
			var recorded []Event
			results[i], recorded = s.addCandidate(st, results[i], c, opts, now, roots)
			events = append(events, recorded...)
		}
		if opts.BeforeCommit != nil {
			if err := opts.BeforeCommit(); err != nil {
				events = nil
				return err
			}
		}
		for _, result := range results {
			if result.Item != nil {
				st.RememberObservation(opts.Observed, now)
				break
			}
		}
		return nil
	})
	if commitFailed(err) {
		return nil, lifespanCommitError(err)
	}
	return results, errors.Join(err, s.Store.RecordCommitted(events...))
}

func (s *Service) inspectRegistrations(paths []string, force bool) ([]Result, []*candidate) {
	results := make([]Result, len(paths))
	candidates := make([]*candidate, len(paths))
	for i, ref := range paths {
		results[i].Ref = ref
		c, err := s.inspect(ref, force)
		if err != nil {
			results[i].Error = err.Error()
			results[i].Err = err
			continue
		}
		candidates[i] = c
	}
	return results, candidates
}

// addCandidate records one result under the write lock. Its failures do not
// abort the batch; reconciled old locations and their events still commit.
func (s *Service) addCandidate(st *State, result Result, c *candidate, opts AddOptions, now time.Time, roots []resolvedGraveyardRoot) (Result, []Event) {
	path, err := pathutil.Abs(result.Ref)
	if err != nil {
		return result, nil // inspect already recorded the boundary error
	}
	if err := s.checkRegistrationPending(st, path, c); err != nil {
		result.Error, result.Err = err.Error(), err
		return result, nil
	}
	if c == nil {
		return result, nil
	}
	if err := s.checkRegistration(st, c, opts, roots); err != nil {
		result.Error, result.Err = err.Error(), err
		return result, nil
	}
	idx := st.FindIdentity(c.identity)
	replacement := idx < 0
	var events []Event
	if replacement {
		var failure Result
		idx, events, failure = s.registrationTarget(st, c, now)
		if failure.Error != "" {
			result.Error, result.Err = failure.Error, failure.Err
			return result, events
		}
	}
	result = saveRegistration(st, result.Ref, c, idx, replacement, opts, now)
	events = append(events, Event{Time: now, Action: result.Action, ID: result.Item.ID,
		Path: c.path, Detail: "expires " + opts.ExpiresAt.Format(time.RFC3339)})
	return result, events
}

func (s *Service) checkRegistrationPending(st *State, path string, c *candidate) error {
	for _, it := range st.Items {
		if !samePath(it.Path, path) && (c == nil || it.Identity == nil || *it.Identity != c.identity) {
			continue
		}
		if err := s.checkBurying(it); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) checkRegistration(st *State, c *candidate, opts AddOptions, roots []resolvedGraveyardRoot) error {
	if err := s.checkWatchedFolders(st, c.path, c.identity, c.isDir); err != nil {
		return err
	}
	if err := s.checkRegistrationObservation(c, opts.Observed); err != nil {
		return err
	}
	if opts.OnlyNew {
		return s.checkAutomaticNew(st, c, roots)
	}
	return s.checkBuried(st, c.path, &c.identity, roots)
}

func (s *Service) checkRegistrationObservation(c *candidate, observed *Observation) error {
	if observed == nil {
		return nil
	}
	if err := s.CheckObservation(observed); err != nil {
		return err
	}
	got, err := s.FS.Identify(c.path)
	if err == nil && got != c.identity {
		return fmt.Errorf("registration unit changed: %w", ErrUnsafe)
	}
	return err
}

func (s *Service) registrationTarget(st *State, c *candidate, now time.Time) (int, []Event, Result) {
	idx, err := st.FindPath(c.path)
	var events []Event
	if err != nil {
		events, err = s.reconcileRegistrationPaths(st, c.path, now)
		if err != nil {
			return idx, events, Result{Error: err.Error(), Err: err}
		}
	}
	if idx < 0 || st.Items[idx].Identity == nil {
		return idx, events, Result{}
	}
	current, err := s.FS.Locate(*st.Items[idx].Identity)
	switch {
	case err == nil:
		old := &st.Items[idx]
		events = append(events, Event{Time: now, Action: "moved", ID: old.ID, Path: current, Detail: "moved from " + old.Path})
		old.Path, old.Rev = current, old.Rev+1
		idx = -1 // the old object still needs its own lifespan
	case !errors.Is(err, ErrNotFound):
		// An unsafe previous object does not definitively refuse this candidate.
		// Leave Err unset so every lookup failure remains retryable.
		return idx, events, Result{Error: fmt.Sprintf("locate previous object at %s: %v", c.path, err)}
	}
	return idx, events, Result{}
}

func (s *Service) reconcileRegistrationPaths(st *State, path string, now time.Time) ([]Event, error) {
	var events []Event
	var unresolved []string
	for j := len(st.Items) - 1; j >= 0; j-- {
		old := &st.Items[j]
		if !samePath(old.Path, path) {
			continue
		}
		if old.Identity == nil {
			unresolved = append(unresolved, fmt.Sprintf("#%d", old.ID))
			continue
		}
		current, lookupErr := s.FS.Locate(*old.Identity)
		if errors.Is(lookupErr, ErrNotFound) {
			st.Items = append(st.Items[:j], st.Items[j+1:]...)
			continue
		}
		if lookupErr == nil && !samePath(current, path) {
			events = append(events, Event{Time: now, Action: "moved", ID: old.ID, Path: current, Detail: "moved from " + old.Path})
			old.Path, old.Rev = current, old.Rev+1
			continue
		}
		unresolved = append(unresolved, fmt.Sprintf("#%d", old.ID))
	}
	if len(unresolved) != 0 {
		return events, fmt.Errorf("%s has unresolved records %s; inspect obit list, then obit spare <id> before adding this path", path, strings.Join(unresolved, ", "))
	}
	return events, nil
}

func saveRegistration(st *State, ref string, c *candidate, idx int, replacement bool, opts AddOptions, now time.Time) Result {
	identity := c.identity
	if idx < 0 {
		item := Item{
			ID: st.NextID, Rev: 1, Path: c.path, IsDir: c.isDir, Identity: &identity,
			CreatedAt: now, ExpiresAt: opts.ExpiresAt, Force: opts.Force, Note: opts.Note,
			Source: opts.Source, Project: opts.Project, Session: opts.Session,
		}
		st.NextID++
		st.Items = append(st.Items, item)
		return Result{Ref: ref, Action: ActionAdded, Item: &item}
	}
	item := &st.Items[idx]
	if replacement {
		*item = Item{ID: item.ID, Rev: item.Rev, CreatedAt: now,
			Source: opts.Source, Project: opts.Project, Session: opts.Session}
	}
	item.Path, item.IsDir, item.Identity = c.path, c.isDir, &identity
	item.ExpiresAt, item.Force = opts.ExpiresAt, opts.Force
	if opts.Note != "" {
		item.Note = opts.Note
	}
	item.Attempts, item.NextAttempt, item.LastError = 0, time.Time{}, ""
	item.Rev++
	copied := *item
	return Result{Ref: ref, Action: ActionUpdated, Item: &copied}
}

// Recheck keep decisions at commit, after automatic vetting's read snapshot.
func (s *Service) checkAutomaticNew(st *State, c *candidate, roots []resolvedGraveyardRoot) error {
	if exempt, err := s.AutomaticExempt(st, c.path, c.identity); err != nil {
		return err
	} else if exempt {
		return &RefusedError{Path: c.path, Reason: "kept by resurrect or spare until its next write"}
	}
	return s.checkNew(st, c, roots)
}

// checkNew runs under the store write lock. Re-read live locations here:
// a batch snapshot cannot authorize shortening a concurrent manual lifespan.
func (s *Service) checkNew(st *State, c *candidate, roots []resolvedGraveyardRoot) error {
	if err := s.checkBuried(st, c.path, &c.identity, roots); err != nil {
		return err
	}
	var scope *RegistrationScope
	if !c.isDir {
		scope = s.RegistrationScope(c.path, c.identity)
	}
	defer scope.Close()
	for _, it := range st.Items {
		if !scope.Related(it) {
			continue
		}
		if err := scope.CheckIdentity(it); err != nil {
			return err
		}
		refuse := func() error {
			return &RefusedError{Path: c.path, Reason: fmt.Sprintf("conflicts with #%d, which already has a lifespan", it.ID)}
		}
		if it.Identity != nil && *it.Identity == c.identity {
			return refuse()
		}
		path := it.Path
		if it.Identity != nil && (scope != nil || it.Identity.Serial == c.identity.Serial) {
			current, err := s.FS.Locate(*it.Identity)
			if errors.Is(err, ErrNotFound) {
				continue
			}
			if err != nil {
				// This is another record's failure, not a definitive refusal
				// of the candidate being approved.
				return fmt.Errorf("could not locate #%d: %v", it.ID, err) //nolint:errorlint // Another record's ErrUnsafe or RefusedError must not discard this candidate.
			}
			if samePath(it.Path, c.path) {
				return refuse()
			}
			path = current
		}
		if samePath(path, c.path) || isInside(path, c.path) || isInside(c.path, path) {
			return refuse()
		}
	}
	return nil
}

func (s *Service) inspect(ref string, force bool) (*candidate, error) {
	abs, _, err := s.FS.LongPath(ref)
	if err != nil {
		return nil, err
	}
	info, err := os.Lstat(abs)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s does not exist: %w", abs, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	if err := s.Guard.Check(abs, force); err != nil {
		return nil, err
	}
	identity, err := s.FS.Identify(abs)
	if err != nil {
		return nil, fmt.Errorf("refusing %s: %w", abs, err)
	}
	return &candidate{path: filepath.Clean(abs), isDir: info.IsDir(), identity: identity}, nil
}
