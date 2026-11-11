package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// Keep the released ledger-key algorithm: changing Unicode folding would
// orphan existing cross-volume graves. A sibling marker disambiguates keys,
// using the data directory's identity rather than any case-folding rule.
// Unmarked legacy roots remain readable and are claimed on the next burial.
func (s *Service) checkGraveyardOwner(root string, create bool) error {
	home, err := pathutil.Abs(s.Store.Home)
	if err != nil {
		return err
	}
	home = filepath.Clean(home)
	root, err = pathutil.Abs(root)
	if err != nil {
		return err
	}
	local, err := s.FS.SameName(filepath.Clean(root), filepath.Join(home, "graveyard"))
	if err != nil || local {
		return err
	}
	marker := root + ".owner"
	if s.confirmedOwners[root] {
		return nil
	}
	_, legacy, err := s.readGraveyardOwner(marker, home)
	if err != nil || !create && (!legacy || s.readOnlyOwners) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(root), 0700); err != nil {
		return err
	}
	// The two ledgers have different store locks. Serialize the first claim
	// on the shared root, then use the existing os.Root.Rename + Sync writer.
	ctx := s.Store.Context()
	if !s.Store.lockDeadline.IsZero() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, s.Store.lockDeadline)
		defer cancel()
	}
	release, err := lockFileContext(ctx, marker+".lock", true)
	if err != nil {
		return err
	}
	defer release()
	owner, _, err := s.readGraveyardOwner(marker, home)
	if err != nil {
		return err
	}
	if owner == nil {
		id, err := s.FS.IdentifyFollowing(home)
		if err != nil {
			return err
		}
		owner = &graveyardOwner{Version: 1, Path: home, Identity: id}
	}
	data, err := json.Marshal(owner)
	if err != nil {
		return err
	}
	// Also reconfirm an existing marker: a previous rename may have been
	// readable after a failed flush. No grave is created until this succeeds.
	if err := writeDurably(marker, append(data, '\n')); err != nil {
		return err
	}
	if s.confirmedOwners != nil {
		s.confirmedOwners[root] = true
	}
	return nil
}

type graveyardOwner struct {
	Version  int      `json:"version"`
	Path     string   `json:"path"`
	Identity Identity `json:"identity"`
}

func (s *Service) readGraveyardOwner(marker, home string) (*graveyardOwner, bool, error) {
	data, err := readShared(marker)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var owner graveyardOwner
	var oldPath string
	legacy := json.Unmarshal(data, &oldPath) == nil
	if legacy {
		owner.Path = oldPath
	} else if err := json.Unmarshal(data, &owner); err != nil || owner.Version != 1 || owner.Identity.FileID == "" {
		return nil, false, fmt.Errorf("invalid graveyard owner marker %s: %w", marker, ErrUnsafe)
	}
	if !filepath.IsAbs(owner.Path) {
		return nil, false, fmt.Errorf("invalid graveyard owner marker %s: %w", marker, ErrUnsafe)
	}
	id, err := s.FS.IdentifyFollowing(home)
	if err != nil {
		return nil, false, err
	}
	same := owner.Identity.Serial == id.Serial && owner.Identity.FileID == id.FileID
	if legacy {
		same, err = s.FS.SameName(owner.Path, home)
		if err != nil {
			return nil, false, err
		}
		// SameName cannot distinguish Data/data in a case-sensitive parent.
		// Resolve the marker's original spelling before assigning it an identity.
		recorded, err := s.FS.IdentifyFollowing(owner.Path)
		if err != nil {
			return nil, false, err
		}
		same = same && recorded.Serial == id.Serial && recorded.FileID == id.FileID
		owner.Version, owner.Identity = 1, recorded
	}
	if !same {
		return nil, false, fmt.Errorf("graveyard ledger key collision or data directory identity changed at %s: recorded home %q, current OBIT_HOME %q; after confirming ownership, remove %q to reclaim this root: %w", marker, owner.Path, home, marker, ErrUnsafe)
	}
	return &owner, legacy, nil
}
