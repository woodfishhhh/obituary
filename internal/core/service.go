package core

import (
	"context"
	"errors"
	"time"

	"github.com/woodfishhhh/obituary/internal/docker"
)

var (
	// ErrNotFound means a file system object definitely does not exist.
	ErrNotFound = errors.New("not found")
	// ErrUnsupported preserves tracked entries when a platform cannot honor FS.
	ErrUnsupported = errors.New("operation is not supported on this platform")
	// ErrUnsafe means the object exists but cannot be handled safely (for
	// example it has several hard links, or its name cannot be represented).
	ErrUnsafe = errors.New("unsafe to recycle")
	// ErrProtected means a rule explicitly protects an automatic item.
	ErrProtected = errors.New("a rule now protects it (or something inside it)")
	// ErrSweepBusy means another process is sweeping the same store.
	ErrSweepBusy    = errors.New("another sweep is running, try again")
	errClaimChanged = errors.New("lifespan changed before burial; left in place")
)

// FS is the file system access Obituary needs; the Windows implementation
// lives in internal/winfs and tests substitute their own.
type FS interface {
	// Identify returns the identity of the object at path (without following
	// links). Files with several hard links yield an error wrapping ErrUnsafe.
	Identify(path string) (Identity, error)
	// ObserveWatch reads identity, size and mtime on one handle. With hold set,
	// writers are excluded until release; sharing/metadata failures retry.
	ObserveWatch(path string, hold bool) (WatchSnapshot, func(), error)
	// IdentifyFollowing and Resolve inspect destination ancestors, following
	// reparse points, so resurrection can compare the original parent objects.
	IdentifyFollowing(path string) (Identity, error)
	Resolve(path string) (string, error)
	// LongPath expands existing 8.3 names without resolving lexical junctions;
	// a missing suffix retains its original spelling. Complete is false when
	// an existing ordinary component could not be expanded; never mark such
	// a burial's names as verified long names.
	LongPath(path string) (expanded string, complete bool, err error)
	// SameName compares individual spellings using Windows ordinal case rules,
	// distinguishing an 8.3 alias from a casing change without Unicode folding.
	SameName(a, b string) (bool, error)
	// Locate returns the current path of the object: ErrNotFound if it is
	// gone, ErrUnsafe if it cannot be handled, other errors if unknown.
	Locate(id Identity) (string, error)
	// Recycle moves path to the Recycle Bin after confirming it is still the
	// object want. It must fail rather than delete permanently.
	Recycle(path string, want Identity) error
	// MovePinned opens path once, confirms on that handle that it is still
	// the object want, and renames it to to through the same handle, so the
	// object checked is the object moved. It never replaces anything at to;
	// to's parent must exist and be on the same volume.
	MovePinned(path string, want Identity, to string) error
	// OpenRestoreParent pins a non-reparse directory. Its identity is queried
	// on that handle, for comparison with the checked lexical parent path.
	OpenRestoreParent(path string) (RestoreParent, error)
	// MkdirRestoreParent creates or opens one ordinary child of a pinned parent,
	// without following a concurrently introduced reparse point.
	MkdirRestoreParent(parent RestoreParent, name string) (RestoreParent, error)
	// FindRestored verifies the source identity and its direct parent's identity
	// together, returning the actual location for interrupted-restore recovery.
	FindRestored(parent, source Identity) (string, error)
	// MovePinnedInto renames through that parent handle and returns the source
	// handle's verified final path. An empty path with nil error means the move
	// succeeded but its final spelling is unavailable; callers audit a fallback.
	MovePinnedInto(path string, want Identity, parent RestoreParent, name string) (string, error)
	// MakeGrave creates dir and fails if it already exists, creating its
	// parents and hiding a per-volume `.obituary` folder.
	MakeGrave(dir string) error
	// RealPath resolves path's real location: short names expanded, links
	// in parent folders resolved, the last component not followed.
	RealPath(path string) (string, error)
}

// RestoreParent owns a destination directory handle until Close.
type RestoreParent interface {
	Close() error
	// Rollback removes only this attempt's newly created empty directory, by
	// its creation handle. Existing and nonempty directories are preserved.
	Rollback() error
	// Path is the verified resolved spelling at the time the handle was opened.
	Path() string
	// Identity was queried on the pinned handle, not by reopening Path.
	Identity() Identity
}

// Delete modes.
const (
	ModeGraveyard = "graveyard"
	ModeRecycle   = "recycle"
)

// DefaultRetention is how long burials stay restorable by default.
const DefaultRetention = 7 * 24 * time.Hour

// Service implements the Obituary operations on top of a Store.
type Service struct {
	Docker docker.Client
	// Fault injection belongs to this service and its request-local copies.
	beforePurgeDelete func()             // after durable pre-delete persistence
	afterBurialStep   func(string) error // interruption without cleanup
	gitMoveBudget     time.Duration      // measured outside store.lock for this sweep
	// Only successful durable owner claims in this operation are cached.
	confirmedOwners map[string]bool
	readOnlyOwners  bool // diagnostics validate legacy markers without migrating them
	burialOperation *BurialOperation
	approvedWatch   *Proposal // request-local explicit directory burial approval
	Store           Store
	Guard           Guard
	FS              FS
	Now             func() time.Time
	// AfterChange refreshes optional presentation after locks are released.
	// It must report failures itself and never change the operation's result.
	AfterChange func(*Service)
	// Mode is ModeGraveyard (default) or ModeRecycle.
	Mode string
	// Retention is how long burials stay restorable (DefaultRetention if 0).
	Retention time.Duration
	// WatchStableWindow is the quiet interval for folder policies (10m if zero).
	WatchStableWindow time.Duration
	// SessionSummary lists expiring items and recent burials at session start
	// ([hooks] session_summary); off by default to save agents' tokens.
	SessionSummary bool
	// Owner reports which Git work tree a real path lies in ("" for none)
	// and whether Git tracks it or anything inside it. Items registered
	// automatically are re-checked before burial: one that left its project
	// or is tracked by then is left alone. Nil disables the check.
	Owner func(path string) (root string, tracked bool, err error)
	// OwnerBatch returns one answer per path using bounded Git queries.
	OwnerBatch OwnerBatchFunc
	// PrepareOwners resolves Git metadata outside store.lock for one sweep.
	// The returned query can prefetch snapshots outside the moving transaction.
	PrepareOwners func(context.Context, []string) OwnerBatchFunc
	// Recheck re-applies a project's rules before an automatic item is
	// buried; ErrProtected refuses it, other errors retry. Nil disables it.
	Recheck func(project, path string, isDir bool) error
}

func (s *Service) trackChanges() (*Service, func()) {
	copy := *s
	changed := false
	copy.Store.changed = &changed
	return &copy, func() {
		if changed {
			copy.notifyChange()
		}
	}
}

func (s *Service) notifyChange() {
	if s.AfterChange != nil && s.Store.Context().Err() == nil {
		s.AfterChange(s)
	}
}

type Ownership struct {
	Root    string
	Tracked bool
	Err     error
	// CheckedAt is the Git reply time or start of its final generation check.
	CheckedAt time.Time
	// Current proves the answer's index generation and project still apply.
	// Nil answers cannot be reused for a later move.
	Current func(context.Context) (bool, time.Time, error)
}

type OwnerBatchFunc func(context.Context, []string) []Ownership

// WithContext binds one request without changing the shared service. Moving
// transactions detach cancellation only after their first committed write.
func (s *Service) WithContext(ctx context.Context) *Service {
	copy := *s
	copy.Store = s.Store.WithContext(ctx)
	return &copy
}

// automatic reports whether an item was registered without an explicit
// request for that exact path, and so needs the Git re-check.
func automatic(it Item) bool {
	return it.Source == "watch" || it.Project != "" && automaticSource(it.Source)
}

func automaticSource(source string) bool {
	switch source {
	case "rule", "proposal", "session":
		return true
	}
	return false
}

func (s *Service) retention() time.Duration {
	if s.Retention > 0 {
		return s.Retention
	}
	return DefaultRetention
}

func (s *Service) recycleMode() bool { return s.Mode == ModeRecycle }

func (s *Service) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Result describes what happened to one requested path or reference.
type Result struct {
	Ref    string `json:"ref"`
	Action string `json:"action,omitempty"`
	Item   *Item  `json:"item,omitempty"`
	Error  string `json:"error,omitempty"`
	Err    error  `json:"-"` // inspection cause; ledger lookup failures remain retryable
}

// Failed reports whether any result carries an error.
func Failed(results []Result) bool {
	for _, r := range results {
		if r.Error != "" {
			return true
		}
	}
	return false
}
