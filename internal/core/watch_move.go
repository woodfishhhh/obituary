package core

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Persist before the last Git check: a crash after the rename must never let
// ordinary manifest recovery entomb an unverified watch directory.
func (s *Service) prepareWatchVerification(st *State, p *preparedBurial, reason string) error {
	it := p.out.Item
	if !s.watchMove(it, reason) {
		return nil
	}
	if err := s.pinWatchSource(p); err != nil {
		return err
	}
	idx := st.Index(it.ID)
	if idx < 0 || st.Items[idx].Watch == nil {
		return errClaimChanged
	}
	snap := st.Items[idx].Watch.Snapshot
	if it.IsDir {
		var release func()
		var err error
		snap, release, err = s.observeWatch(p.out.Path, *it.Identity, true, true)
		if err != nil {
			return err
		}
		release()
	}
	p.watch = &snap
	return writeGraveManifest(p.manifest, burialManifest{Version: 3, Burial: p.burial, WatchCheck: p.watch})
}

// Windows disallows renaming a directory with open descendant handles. Reuse
// the pinned resurrection primitive to undo any change across that gap. A
// pending v3 manifest retains evidence if either verification or rollback stops.
func (s *Service) verifyWatchMovement(st *State, p *preparedBurial) bool {
	if p.watch == nil {
		return true
	}
	b := p.burial
	err := p.checkWatchOriginalDownload()
	var snap WatchSnapshot
	var release func()
	if err == nil {
		snap, release, err = s.observeWatch(b.Grave, *b.Identity, b.IsDir, true)
	}
	if err == nil {
		release()
		if sameWatchSnapshot(*p.watch, snap) {
			return true
		}
		err = fmt.Errorf("object changed after final observation")
	}
	detail := err.Error()
	if rollbackErr := s.rollbackWatchDirectory(b); rollbackErr != nil {
		p.retained = true
		detail += fmt.Sprintf("; manual attention required: rollback to %s failed: %v; content retained at %s; see obit doctor", b.Path, rollbackErr, b.Grave)
		if saveErr := writeGraveManifest(p.manifest, burialManifest{Version: 3, Burial: b, WatchCheck: p.watch, WatchError: detail}); saveErr != nil {
			detail += "; save rollback diagnostic: " + saveErr.Error()
		}
	} else {
		detail += "; returned to original location; retry after stability"
	}
	p.out.Action, p.out.Detail = ActionRetry, detail
	// The lock is still held; bind retry to any timing refresh in this attempt.
	if idx := st.Index(p.out.Item.ID); idx >= 0 {
		p.out.Item = st.Items[idx]
	}
	return false
}

// The source parent, never the grave parent, owns aria2's resume marker.
// Revalidate the captured parent identity before enumerating the old name.
func (s *Service) pinWatchSource(p *preparedBurial) error {
	b := p.burial
	if len(b.Parents) == 0 {
		return fmt.Errorf("original download parent is unknown")
	}
	// Go's os/root_windows.go pins this directory without FILE_SHARE_DELETE.
	// Use its existing handle-relative ReadDir rather than adding a Win32 API.
	root, err := os.OpenRoot(filepath.Dir(b.Path))
	if err != nil {
		return err
	}
	p.sourceParent = root
	parent, err := s.FS.IdentifyFollowing(filepath.Dir(b.Path))
	if err != nil {
		return err
	}
	if !sameWatchIdentity(parent, b.Parents[0].Identity) {
		return fmt.Errorf("original download parent changed")
	}
	return nil
}

func (p *preparedBurial) checkWatchOriginalDownload() error {
	entries, err := fs.ReadDir(p.sourceParent.FS(), ".")
	if err != nil {
		return err
	}
	return watchSidecar(p.burial.Path, entries)
}

func (s *Service) rollbackWatchDirectory(b Burial) error {
	if _, err := s.verifyGrave(b); err != nil {
		return err
	}
	parent, err := s.prepareRestoreParent(b, b.Path)
	if err != nil {
		return err
	}
	defer parent.Close()
	_, err = s.FS.MovePinnedInto(b.Grave, *b.Identity, parent.RestoreParent, filepath.Base(b.Path))
	if err != nil {
		return errors.Join(err, parent.Rollback())
	}
	return nil
}
