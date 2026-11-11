package core

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// FileAncestry holds a regular file and its complete directory ancestry against
// rename, on one verified local filesystem with stable IDs. Unsupported, linked,
// case-sensitive or unverifiable paths must not produce this capability.
type FileAncestry interface {
	Path() string
	Identities() []Identity
	Close() error
}

type fileAncestryFS interface {
	OpenFileAncestry(path string, want Identity) (FileAncestry, error)
}

// RegistrationScope only excludes identities proven unrelated to a regular
// file. A nil scope means the original full lookup, including error propagation.
// Each locked registration builds its own scope; a preview never authorizes it.
type RegistrationScope struct {
	ancestry FileAncestry
	ids      map[Identity]bool
	file     Identity
}

func (s *Service) RegistrationScope(path string, id Identity) *RegistrationScope {
	fs, ok := s.FS.(fileAncestryFS)
	if !ok || !completeRegistrationIdentity(&id) {
		return nil
	}
	ancestry, err := fs.OpenFileAncestry(path, id)
	if err != nil {
		return nil // an optional proof failed; retain every original lookup
	}
	scope := &RegistrationScope{ancestry: ancestry, file: id.Normalized(), ids: map[Identity]bool{}}
	for _, ancestor := range ancestry.Identities() {
		scope.ids[ancestor.Normalized()] = true
	}
	return scope
}

func (s *RegistrationScope) Close() {
	if s != nil {
		_ = s.ancestry.Close()
	}
}

func (s *RegistrationScope) Related(it Item) bool {
	if it.Kind == KindContainer {
		return false
	}
	if s == nil || !completeRegistrationIdentity(it.Identity) {
		return true
	}
	id := it.Identity.Normalized()
	// Only the pinned, verified volume is eligible. Other volume spellings,
	// remote roots and incomplete legacy identities remain possible conflicts.
	if id.Volume != s.file.Volume || id.Serial != s.file.Serial || s.ids[id] {
		return true
	}
	// A stale 8.3 name may no longer expand to its original object. Resolve the
	// recorded identity instead of using today's short-name mapping to exclude it.
	path := s.ancestry.Path()
	return strings.Contains(it.Path, "~") || samePath(it.Path, path) || isInside(it.Path, path) || isInside(path, it.Path)
}

// A legacy identity cannot be treated as unrelated merely because its remembered
// path is elsewhere. When the file proof is active, missing identity evidence
// remains a retryable failure; the full-path fallback keeps its existing policy.
func (s *RegistrationScope) CheckIdentity(it Item) error {
	if s != nil && !completeRegistrationIdentity(it.Identity) {
		return fmt.Errorf("could not locate #%d: recorded identity is incomplete", it.ID)
	}
	return nil
}

func completeRegistrationIdentity(id *Identity) bool {
	if id == nil || id.Volume == "" || id.Serial == 0 || len(id.FileID) != 32 ||
		id.FileID != strings.ToLower(id.FileID) || strings.Trim(id.FileID, "0") == "" {
		return false
	}
	_, err := hex.DecodeString(id.FileID)
	return err == nil
}
