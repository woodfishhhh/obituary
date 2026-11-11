package core

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

// RestoreCleanup is committed with removal of the burial index and retains
// the location and identities needed to finish either restore outcome.
type RestoreCleanup struct {
	Kind        string         `json:"kind"`
	Burial      Burial         `json:"burial"`
	Path        string         `json:"path,omitempty"`
	Restore     *restoreIntent `json:"resurrect,omitempty"`
	CommittedAt time.Time      `json:"committedAt"`
}

func (st *State) cleanupIndex(id int64) int {
	return slices.IndexFunc(st.RestoreCleanups, func(c RestoreCleanup) bool { return c.Burial.ID == id })
}

// Only a validated receipt can authorize metadata cleanup. Unlike manifests,
// it can converge when both its manifest and empty grave have already gone.
func (s *Service) inspectCleanup(c RestoreCleanup, st State) PendingGrave {
	b := c.Burial
	p := PendingGrave{Burial: &b, id: b.ID, Status: "unverifiable", restore: c.Restore, cleanup: &c}
	fail := func(err error) PendingGrave { p.Error = "cannot verify cleanup receipt: " + err.Error(); return p }
	p.Manifest = filepath.Dir(b.Grave) + ".json"
	if st.BurialIndex(b.ID) >= 0 || st.Index(b.ID) >= 0 || st.ProposalIndex(b.ID) >= 0 || b.ID < 1 || st.NextID <= b.ID {
		return fail(errors.New("cleanup ID is not an allocated, committed burial"))
	}
	if c.Kind != ActionResurrected && c.Kind != ActionAbandoned {
		return fail(errors.New("unknown cleanup kind"))
	}
	root, rootErr := pathutil.Abs(s.graveyardRoot(b.Path))
	grave, graveErr := pathutil.Abs(b.Grave)
	if rootErr != nil || graveErr != nil {
		return fail(errors.Join(rootErr, graveErr))
	}
	if !filepath.IsAbs(b.Path) || !filepath.IsAbs(b.Grave) ||
		!samePath(filepath.Dir(filepath.Dir(grave)), root) ||
		!strings.HasPrefix(filepath.Base(filepath.Dir(b.Grave)), fmt.Sprintf("%d-", b.ID)) ||
		!samePath(filepath.Base(b.Grave), filepath.Base(b.Path)) || samePath(b.Path, root) || isInside(b.Path, root) {
		return fail(errors.New("cleanup paths do not name this ledger's grave"))
	}
	for _, id := range []*Identity{b.Identity, b.GraveID} {
		if id == nil {
			return fail(errors.New("cleanup lacks object identity"))
		}
		raw, err := hex.DecodeString(id.FileID)
		volume, volumeErr := pathutil.Abs(id.Volume)
		if err != nil || volumeErr != nil || len(raw) != 16 || !samePath(pathutil.VolumeRoot(volume), pathutil.VolumeRoot(grave)) {
			return fail(errors.New("invalid cleanup identity"))
		}
	}
	if c.Restore != nil && !validRestoreIntent(c.Restore, b.Identity) {
		return fail(errors.New("invalid destination identity or name"))
	}
	realRoot, err := s.FS.RealPath(root)
	if err != nil {
		// A validated receipt can outlive both its empty grave and root.
		// checkCommittedGrave below also proves the grave is gone online.
		if !s.missingOnOnlineVolume(root, err, burialVolume(b)) {
			return fail(err)
		}
	} else {
		parent, err := s.FS.RealPath(filepath.Dir(filepath.Dir(grave)))
		if err != nil {
			return fail(err)
		}
		same, err := s.sameDirectory(parent, realRoot)
		if err != nil {
			return fail(err)
		}
		if !same {
			return fail(errors.New("cleanup grave escapes this ledger's graveyard root"))
		}
	}
	if err := s.checkCommittedGrave(b); err != nil {
		return fail(err)
	}
	p.Status = "restore-committed"
	if c.Kind == ActionAbandoned {
		p.Status = "restore-abandoned"
	}
	p.Path = c.Path
	return p
}

// Retain a manifest-only receipt before removing any metadata.
func (s *Service) retainCleanup(p PendingGrave, kind, path string) error {
	if p.cleanup != nil {
		// The previous commit may be readable but not durably confirmed.
		// Reflush it before deleting the last independent recovery evidence.
		return s.Store.Confirm(func(st *State) error {
			if kind == ActionResurrected {
				st.spareRestoredWatch(*p.Burial, path, s.now())
				s.spareWatchObject(st, p.Burial.WatchID, p.Burial.Identity, path, "restored")
			}
			return nil
		})
	}
	c := RestoreCleanup{Kind: kind, Burial: *p.Burial, Path: path, Restore: p.restore, CommittedAt: s.now()}
	return s.Store.Update(func(st *State) error {
		if kind == ActionResurrected {
			st.spareRestoredWatch(c.Burial, path, s.now())
			s.spareWatchObject(st, c.Burial.WatchID, c.Burial.Identity, path, "restored")
			s.spareAutomaticObject(st, c.Burial.Identity, path)
		}
		if i := st.cleanupIndex(c.Burial.ID); i >= 0 {
			st.RestoreCleanups[i] = c
		} else {
			st.RestoreCleanups = append(st.RestoreCleanups, c)
		}
		return nil
	})
}

func (s *Service) retireCleanup(b Burial) error {
	return s.Store.Update(func(st *State) error {
		st.RestoreCleanups = slices.DeleteFunc(st.RestoreCleanups, func(c RestoreCleanup) bool { return c.Burial.ID == b.ID })
		return nil
	})
}

func (s *Service) appendCleanupReceipts(pending []PendingGrave, st State) []PendingGrave {
	for _, c := range st.RestoreCleanups {
		if slices.ContainsFunc(pending, func(p PendingGrave) bool { return p.Number() == c.Burial.ID }) {
			continue
		}
		p := s.inspectCleanup(c, st)
		// A malformed or inaccessible manifest still blocks cleanup; a receipt
		// substitutes only for a manifest proven absent on an online volume.
		if c.Burial.Grave != "" {
			if _, err := os.Lstat(p.Manifest); !s.missingOnOnlineVolume(p.Manifest, err, burialVolume(c.Burial)) {
				p.Status, p.Error = "unverifiable", fmt.Sprintf("cleanup manifest has not been verified: %v", err)
			}
		}
		pending = append(pending, p)
	}
	return pending
}
