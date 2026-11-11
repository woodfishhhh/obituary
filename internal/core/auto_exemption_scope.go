package core

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

type objectIdentity struct {
	serial uint64
	fileID string
}

func objectKey(id Identity) objectIdentity { return objectIdentity{id.Serial, id.FileID} }

// Inspect only the candidate's actual ancestry and descendants, using the
// existing read-only identity API (which permits hard links). An unrelated
// kept ID never needs a destructive-operation Locate and cannot block it.
func (s *Service) automaticScope(path string, want Identity) (map[objectIdentity]string, error) {
	real, err := s.FS.RealPath(path)
	if err != nil {
		return nil, err
	}
	got, err := s.FS.IdentifyFollowing(real)
	if err != nil {
		return nil, err
	}
	if !sameWatchIdentity(got, want) {
		return nil, fmt.Errorf("automatic candidate identity changed; retry")
	}
	paths := map[objectIdentity]string{objectKey(want): real}
	for parent := filepath.Dir(real); ; parent = filepath.Dir(parent) {
		id, err := s.FS.IdentifyFollowing(parent)
		if err != nil {
			return nil, err
		}
		paths[objectKey(id)] = parent
		if parent == filepath.Dir(parent) {
			break
		}
	}
	err = filepath.WalkDir(real, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := s.Store.Context().Err(); err != nil {
			return err
		}
		if err := pathutil.Unchanged(path); err != nil {
			return err
		}
		id, err := s.FS.IdentifyFollowing(path)
		if err != nil {
			return err
		}
		paths[objectKey(id)] = path
		return nil
	})
	return paths, err
}
