package fs

import (
	"errors"
)

var (
	// ErrNotFound means a file system object definitely does not exist.
	ErrNotFound = errors.New("not found")
	// ErrUnsupported preserves tracked entries when a platform cannot honor FS.
	ErrUnsupported = errors.New("operation is not supported on this platform")
	// ErrUnsafe means the object exists but cannot be handled safely (for
	// example it has several hard links, or its name cannot be represented).
	ErrUnsafe = errors.New("unsafe to recycle")
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
