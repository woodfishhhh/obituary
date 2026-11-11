package core

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// Index returns the position of the item with id, or -1.
func (st *State) Index(id int64) int {
	for i, item := range st.Items {
		if item.ID == id {
			return i
		}
	}
	return -1
}

// find resolves IDs directly and paths by their current identity. Only an
// absent object permits a fallback to the ledger's (possibly stale) path.
func (s *Service) find(st *State, ref string) (int, string, error) {
	if id, err := strconv.ParseInt(strings.TrimPrefix(ref, "#"), 10, 64); err == nil {
		if idx := st.Index(id); idx >= 0 {
			return idx, st.Items[idx].Path, nil
		}
		if strings.HasPrefix(ref, "#") {
			return -1, ref, nil // explicit IDs never fall back to a cwd-relative filename
		}
	}
	abs, err := s.referencePath(ref)
	if err != nil {
		return -1, "", err
	}
	id, err := s.FS.Identify(abs)
	if err == nil {
		return st.FindIdentity(id), filepath.Clean(abs), nil
	}
	if !errors.Is(err, ErrNotFound) {
		return -1, "", fmt.Errorf("identify %s: %w", abs, err)
	}
	idx, err := st.FindPath(abs)
	return idx, filepath.Clean(abs), err
}

// Resolve aliases once at the service boundary; State only matches strings.
// Missing suffixes keep their spelling, while inaccessible aliases need an ID.
func (s *Service) referencePath(ref string) (string, error) {
	path, complete, err := s.FS.LongPath(ref)
	if err != nil {
		return "", fmt.Errorf("resolve reference %s; use an ID if its alias is unavailable: %w", ref, err)
	}
	if !complete {
		return "", fmt.Errorf("cannot verify every name in %s; use an ID", ref)
	}
	return path, nil
}

func (s *Service) findBurial(st *State, ref string) (int, error) {
	if _, numeric, err := BurialReferenceID(ref); numeric || err != nil {
		if err := s.Store.loadGraveyardForReference(st, ref); err != nil {
			return -1, err
		}
		return st.FindBurial(ref)
	}
	path, err := s.referencePath(ref)
	if err != nil {
		return -1, err
	}
	if err := s.Store.loadGraveyardForReference(st, path); err != nil {
		return -1, err
	}
	return st.FindBurial(path)
}

// BurialReferenceID classifies the spelling before parsing its value. A failed
// numeric parse must never turn a destructive grave reference into a filename.
// Lifespan commands deliberately retain their historical fallback in find.
func BurialReferenceID(ref string) (id int64, numeric bool, err error) {
	trimmed := strings.TrimSpace(ref)
	digits := strings.TrimPrefix(trimmed, "#")
	if len(digits) > 0 && (digits[0] == '+' || digits[0] == '-') {
		digits = digits[1:]
	}
	if digits == "" {
		return 0, false, nil
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return 0, false, nil
		}
	}
	id, err = strconv.ParseInt(strings.TrimPrefix(ref, "#"), 10, 64)
	if err != nil || trimmed != ref {
		return 0, true, fmt.Errorf("%q is not a valid burial ID; for a filename use ./%s or an absolute path", ref, ref)
	}
	return id, true, nil
}

// BurialIndex returns the position of the burial with id, or -1.
func (st *State) BurialIndex(id int64) int {
	for i, b := range st.Burials {
		if b.ID == id {
			return i
		}
	}
	return -1
}

// FindBurial matches a numeric ID or an original path (the latest burial wins).
func (st *State) FindBurial(ref string) (int, error) {
	if id, numeric, err := BurialReferenceID(ref); numeric {
		if err != nil {
			return -1, err
		}
		if idx := st.BurialIndex(id); idx >= 0 {
			return idx, nil
		}
		return -1, nil // every numeric grave reference is an ID, even when absent
	}
	abs, err := pathutil.Abs(ref)
	if err != nil {
		return -1, err
	}
	paths := make([]string, len(st.Burials))
	for i, b := range st.Burials {
		if !pathutil.RequiresDirectory(ref) || b.IsDir {
			paths[i] = b.Path
		}
	}
	matches, exact := matchingPaths(paths, abs)
	if !exact && len(matches) > 1 {
		return -1, fmt.Errorf("%s matches multiple burials; use an ID", ref)
	}
	found := -1
	for _, i := range matches {
		if found < 0 || st.Burials[i].BuriedAt.After(st.Burials[found].BuriedAt) {
			found = i
		}
	}
	return found, nil
}

func missingBurial(ref string) error {
	if id, err := strconv.ParseInt(strings.TrimPrefix(ref, "#"), 10, 64); err == nil {
		return fmt.Errorf("burial %d was not found; for a filename use ./%s or an absolute path", id, strings.TrimPrefix(ref, "#"))
	}
	return fmt.Errorf("%s is not in the graveyard; if an alias cannot be resolved, use an ID", ref)
}

// FindPath returns the sole matching record, refusing ambiguous ledger paths.
func (st *State) FindPath(path string) (int, error) {
	found := -1
	var ids []string
	paths := make([]string, len(st.Items))
	for i, item := range st.Items {
		if !pathutil.RequiresDirectory(path) || item.IsDir {
			paths[i] = item.Path
		}
	}
	matches, _ := matchingPaths(paths, path)
	for _, i := range matches {
		found = i
		ids = append(ids, fmt.Sprintf("#%d", st.Items[i].ID))
	}
	if len(ids) > 1 {
		return -1, fmt.Errorf("%s matches multiple records: %s; use an ID", path, strings.Join(ids, ", "))
	}
	return found, nil
}

// Exact names win. Only ASCII is folded for a missing exact match: Unicode
// simple folding aliases names (notably k and Kelvin sign) that NTFS separates.
// Conservative protection checks deliberately continue to use samePath.
func matchingPaths(paths []string, path string) ([]int, bool) {
	if normalized, err := pathutil.Abs(path); err == nil {
		path = normalized
	} else {
		return nil, false
	}
	path = filepath.Clean(path)
	var exact, folded []int
	for i, candidate := range paths {
		if candidate == "" {
			continue
		}
		if normalized, err := pathutil.Abs(candidate); err == nil {
			candidate = normalized
		} else {
			continue
		}
		candidate = filepath.Clean(candidate)
		if candidate == path {
			exact = append(exact, i)
		}
		if asciiPath(candidate) == asciiPath(path) {
			folded = append(folded, i)
		}
	}
	if len(exact) > 0 {
		return exact, true
	}
	return folded, false
}

func asciiPath(path string) string {
	b := []byte(path)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + ('a' - 'A')
		}
	}
	return string(b)
}

// FindIdentity returns the index of the item tracking the object id, or -1.
func (st *State) FindIdentity(id Identity) int {
	id = id.Normalized()
	for i, item := range st.Items {
		if item.Identity != nil && item.Identity.Normalized() == id {
			return i
		}
	}
	return -1
}

// NextDue returns the earliest moment any item or burial needs attention.
func (st *State) NextDue() (time.Time, bool) {
	var next time.Time
	for _, item := range st.Items {
		if st.awaitingWatchBurial(item) {
			continue
		}
		due := item.ExpiresAt
		if item.NextAttempt.After(due) {
			due = item.NextAttempt
		}
		if next.IsZero() || due.Before(next) {
			next = due
		}
	}
	for _, b := range st.Burials {
		due := b.PurgeAt
		if b.PurgeIntent != nil {
			due = b.NextPurgeAttempt
			if due.IsZero() {
				due = b.PurgeIntent.VerifiedAt
			}
		} else if b.NextPurgeAttempt.After(due) {
			due = b.NextPurgeAttempt
		}
		if next.IsZero() || due.Before(next) {
			next = due
		}
	}
	for _, intent := range st.ContainerIntents {
		if next.IsZero() || intent.At.Before(next) {
			next = intent.At
		}
	}
	return next, !next.IsZero()
}
