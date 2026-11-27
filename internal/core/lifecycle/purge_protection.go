package lifecycle

import (
	"fmt"
	"os"
	"path/filepath"

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

func (tree *purgeInventory) protect(items []Item) error {
	if tree == nil {
		return nil
	}
	active := false
	for _, it := range items {
		if it.Kind == KindContainer || it.Kind == KindDockerVolume {
			continue
		}
		if it.Identity == nil {
			return fmt.Errorf("cannot exclude active item #%d without an identity", it.ID)
		}
		if it.Identity.Serial != tree.root.Serial {
			continue
		}
		if tree.lifespanSnapshot == nil {
			return fmt.Errorf("active lifespans changed; retry directory inspection")
		}
		active = true
		if tree.inventory[it.Identity.FileID] || it.Identity.FileID == tree.root.FileID {
			return fmt.Errorf("grave contains protected active item #%d", it.ID)
		}
	}
	return tree.check(active)
}

func (s *Service) purgeProtection(b Burial, state State) error {
	tree, err := s.inspectPurgeTree(b, state)
	if err != nil || tree == nil {
		return err
	}
	defer tree.close()
	return s.Store.ViewItems(tree.protect)
}

func (s *Service) purgeVerified(b Burial) error {
	var ids fs.ProtectedIDs
	guard := s.Store.GuardItems(func(items []Item) error {
		ids = make(fs.ProtectedIDs)
		for _, it := range items {
			if it.Kind == KindContainer || it.Kind == KindDockerVolume {
				continue
			}
			if it.Identity == nil {
				return fmt.Errorf("cannot exclude active item #%d without an identity", it.ID)
			}
			if it.Identity.Serial == b.GraveID.Serial {
				ids[it.Identity.FileID] = true
			}
		}
		return nil
	})
	stage := ""
	if b.PurgeIntent != nil {
		stage = b.PurgeIntent.Stage
	}
	plan := fs.PurgePlan{Name: filepath.Base(b.Grave), Object: *b.Identity, Stage: stage,
		Guard: func(check func(fs.ProtectedIDs) error) error { return guard(func() error { return check(ids) }) },
	}
	if err := s.resumePurgePlan(b, &plan); err != nil {
		return err
	}
	if err := plan.WithObject(b.Identity.FileID, false, nil); err != nil {
		return err
	}
	return s.FS.PurgeGrave(filepath.Dir(b.Grave), *b.GraveID, plan)
}

func (s *Service) resumePurgePlan(b Burial, plan *fs.PurgePlan) error {
	if plan.Stage != "" {
		return nil
	}
	entries, err := os.ReadDir(filepath.Dir(b.Grave))
	if err != nil {
		return err
	}
	plan.Stage, _ = gravePurgeStage(b, entries)
	if plan.Stage == "" {
		plan.Stage = s.newPurgeIntent("").Stage
		return nil
	}
	plan.LegacyMembers = make(map[string]bool, len(b.Children))
	for _, c := range b.Children {
		if c.Identity != nil {
			plan.LegacyMembers[c.Identity.FileID] = true
		}
	}
	return nil
}
