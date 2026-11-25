package lifecycle

import (
	"fmt"

	"github.com/woodfishhhh/obituary/internal/core/fs"
)

// An observed directory owns its watcher until the current registrations are checked.
type purgeInventory struct {
	*lifespanSnapshot
	root Identity
}

func (s *Service) inspectPurgeTree(b Burial, state State) (*purgeInventory, error) {
	id, mode, err := s.FS.Inspect(b.Grave)
	if err != nil {
		return nil, err
	}
	if id != *b.Identity {
		return nil, ErrUnsafe
	}
	if !b.IsDir && !mode.IsDir() {
		return nil, nil
	}
	tree := &purgeInventory{root: id}
	if len(state.Items) > 0 {
		tree.lifespanSnapshot = &lifespanSnapshot{real: b.Grave}
		if err := s.scanInventory(tree.lifespanSnapshot, id); err != nil {
			tree.close()
			return nil, err
		}
	}
	return tree, nil
}

func (tree *purgeInventory) protect(items []Item) (fs.ProtectedIDs, error) {
	if tree == nil {
		return nil, nil
	}
	protected := make(fs.ProtectedIDs)
	for _, it := range items {
		if it.Kind == KindContainer || it.Kind == KindDockerVolume {
			continue
		}
		if it.Identity == nil {
			return nil, fmt.Errorf("cannot exclude active item #%d without an identity", it.ID)
		}
		if it.Identity.Serial != tree.root.Serial {
			continue
		}
		if tree.lifespanSnapshot == nil {
			return nil, fmt.Errorf("active lifespans changed; retry directory inspection")
		}
		protected[it.Identity.FileID] = true
		if tree.inventory[it.Identity.FileID] || it.Identity.FileID == tree.root.FileID {
			return nil, fmt.Errorf("grave contains protected active item #%d", it.ID)
		}
	}
	return protected, tree.check(len(protected) > 0)
}

func (s *Service) purgeProtection(b Burial, state State) (fs.ProtectedIDs, error) {
	tree, err := s.inspectPurgeTree(b, state)
	if err != nil || tree == nil {
		return nil, err
	}
	defer tree.close()
	var protected fs.ProtectedIDs
	err = s.Store.ViewItems(func(items []Item) error {
		var checkErr error
		protected, checkErr = tree.protect(items)
		return checkErr
	})
	return protected, err
}
