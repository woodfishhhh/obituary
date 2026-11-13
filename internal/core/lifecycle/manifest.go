package lifecycle

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// A manifest is independent of Item: normal lifespan mutations cannot destroy
// the information needed after a successful move and a failed ledger commit.
type burialManifest struct {
	Version int `json:"version"`
	Burial
	Resurrect  *restoreIntent `json:"resurrect,omitempty"`
	WatchCheck *WatchSnapshot `json:"watchCheck,omitempty"`
	WatchError string         `json:"watchError,omitempty"`
}

const manifestBytes = 1 << 20

func writeManifest(path string, b Burial) error {
	return writeGraveManifest(path, burialManifest{Version: 1, Burial: b})
}

func writeGraveManifest(path string, m burialManifest) error {
	data, err := json.Marshal(m)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if len(data) > manifestBytes {
		return fmt.Errorf("too many registered entries in this directory for a burial manifest; spare some entries or bury them separately: %w", ErrUnsafe)
	}
	return writeDurably(path, data)
}

// Only delete metadata and the particular empty directory just verified.
// os.Remove refuses a nonempty directory; no recovery path deletes an object.
func (s *Service) discardEmptyGrave(manifest string, b Burial) error {
	dir := filepath.Dir(b.Grave)
	got, err := s.FS.Identify(dir)
	if err != nil {
		return err
	}
	if got != *b.GraveID {
		return fmt.Errorf("grave directory changed: %w", ErrUnsafe)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fmt.Errorf("grave is no longer empty: %w", ErrUnsafe)
	}
	if err := os.Remove(manifest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Remove(dir)
}

// PendingGrave is the read-only receipt for a manifest needing recovery.
type PendingGrave struct {
	Manifest     string  `json:"manifest"`
	Burial       *Burial `json:"burial,omitempty"`
	Status       string  `json:"status"` // burial or restore phase; unverifiable always retains metadata
	Error        string  `json:"error,omitempty"`
	Path         string  `json:"path,omitempty"` // verified restored location
	restore      *restoreIntent
	restoreValid bool
	id           int64   // filename ID also blocks a claim when JSON cannot be decoded
	root         string  // nonempty when enumeration of this entire root failed
	partial      *Burial // partially decoded content can block work, never authorize it
	cleanup      *RestoreCleanup
}

// Number identifies a pending receipt even when its JSON is unreadable.
// Zero denotes an unnumbered scan error, which cannot advance a cursor.
func (p PendingGrave) Number() int64 {
	if p.id > 0 {
		return p.id
	}
	if p.Burial != nil {
		return p.Burial.ID
	}
	return 0
}

func (s *Service) scanManifests(st State, paths ...string) ([]PendingGrave, error) {
	return s.scanGraveyardRoots(st, s.resolveGraveyardRoots(st, paths...))
}

func (s *Service) scanGraveyardRoots(st State, roots []resolvedGraveyardRoot) ([]PendingGrave, error) {
	var pending []PendingGrave
	var scanErr error
	fail := func(root string, err error) {
		err = fmt.Errorf("scan %s: %w", root, err)
		scanErr = errors.Join(scanErr, err)
		pending = append(pending, PendingGrave{Manifest: root, root: root, Status: "unverifiable", Error: err.Error()})
	}
	for _, resolved := range roots {
		root := resolved.path
		if err := s.checkGraveyardOwner(root, false); err != nil {
			fail(root, err)
			continue
		}
		if err := resolved.err; err != nil {
			if !s.missingOnOnlineVolume(root, err, nil) {
				fail(root, err)
			}
			continue
		}
		entries, err := os.ReadDir(root)
		if err != nil {
			fail(root, err)
			continue
		}
		for _, entry := range entries {
			if strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
				pending = append(pending, s.inspectManifest(root, filepath.Join(root, entry.Name()), st))
			}
		}
	}
	return s.appendCleanupReceipts(pending, st), scanErr
}

// inspectManifest records every validation or inspection error in this receipt;
// a bad manifest never aborts inspection of the remaining graveyard entries.
func (s *Service) inspectManifest(root, path string, st State) (p PendingGrave) {
	p.Manifest, p.Status = path, "unverifiable"
	name := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	id, _, _ := strings.Cut(name, "-")
	p.id, _ = strconv.ParseInt(id, 10, 64)
	if err := s.checkGraveyardOwner(root, false); err != nil {
		return p.failedManifest(err)
	}
	m, err := readBurialManifest(path, &p)
	if err != nil {
		return p.failedManifest(err)
	}
	b := m.Burial
	p.Burial = &b
	identities, err := validateManifestMetadata(m, p.id)
	if err != nil {
		return p.failedManifest(err)
	}
	if err := validateManifestName(name, b); err != nil {
		return p.failedManifest(err)
	}
	if err := s.validateManifestRoot(root, b); err != nil {
		return p.failedManifest(err)
	}
	if err := validateManifestIdentities(b, identities); err != nil {
		return p.failedManifest(err)
	}
	return s.inspectManifestState(p, m, st)
}

func (p PendingGrave) failedManifest(err error) PendingGrave {
	p.Error = err.Error()
	return p
}

func (s *Service) inspectManifestState(p PendingGrave, m burialManifest, st State) PendingGrave {
	b := *p.Burial
	if b.IdentityMismatch != "" {
		p.Status, p.Error = "identity-mismatch", b.IdentityMismatch
		return p
	}
	if m.Resurrect != nil {
		p.restore = m.Resurrect
		return s.inspectRestoreIntent(p, st)
	}
	if idx := st.BurialIndex(b.ID); idx >= 0 {
		if !sameBurial(st.Burials[idx], b) {
			return p.failedManifest(errors.New("manifest conflicts with the recorded burial"))
		}
		p.Status = "recorded"
		return p
	}
	if st.ProposalIndex(b.ID) >= 0 {
		return p.failedManifest(errors.New("manifest ID belongs to a proposal"))
	}
	return s.inspectUnrecordedManifest(p, m)
}

func (s *Service) inspectUnrecordedManifest(p PendingGrave, m burialManifest) PendingGrave {
	b := *p.Burial
	dir := filepath.Dir(b.Grave)
	info, err := os.Lstat(dir)
	if err != nil {
		return p.failedManifest(err)
	}
	if !info.IsDir() || isLink(info) {
		return p.failedManifest(errors.New("grave is not a plain directory"))
	}
	got, err := s.FS.Identify(dir)
	if err != nil {
		return p.failedManifest(err)
	}
	if got != *b.GraveID {
		return p.failedManifest(errors.New("grave directory identity does not match the manifest"))
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return p.failedManifest(err)
	}
	if len(entries) == 0 {
		p.Status = "empty"
		return p
	}
	if _, err := s.verifyGrave(b); err != nil {
		return p.failedManifest(err)
	}
	info, err = os.Lstat(b.Grave)
	if err != nil {
		return p.failedManifest(err)
	}
	if info.IsDir() != b.IsDir || isLink(info) {
		return p.failedManifest(errors.New("buried object type does not match the manifest"))
	}
	if m.WatchCheck != nil {
		return p.failedManifest(fmt.Errorf("watch object needs manual attention; content is retained at %s; original location %s; %s", b.Grave, b.Path, m.WatchError))
	}
	p.Status = "pending"
	return p
}

func sameBurial(a, b Burial) bool {
	return a.ID == b.ID && samePath(a.Grave, b.Grave) && a.GraveID != nil && b.GraveID != nil &&
		a.Identity != nil && b.Identity != nil && *a.GraveID == *b.GraveID && *a.Identity == *b.Identity &&
		samePath(a.Path, b.Path) && a.LongNames == b.LongNames && a.IsDir == b.IsDir && a.BuriedAt.Equal(b.BuriedAt) &&
		a.PurgeAt.Equal(b.PurgeAt) && a.Reason == b.Reason && a.IdentityMismatch == b.IdentityMismatch && slices.Equal(a.Parents, b.Parents) && slices.EqualFunc(a.Children, b.Children, sameBuriedChild)
}

// Contains can shrink during recovery; the original identity snapshots agree.
func sameBuriedChild(a, b BuriedChild) bool {
	return a.ID == b.ID && a.Identity != nil && b.Identity != nil && *a.Identity == *b.Identity
}

// Caller owns graveyard.lock. Only a manifest proven to describe this exact
// recorded burial can be removed before resurrection/purge removes its index.
func (s *Service) cleanRecordedManifest(b Burial, st State) error {
	path := filepath.Dir(b.Grave) + ".json"
	if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	p := s.inspectManifest(s.graveyardRoot(b.Path), path, st)
	if p.restore != nil {
		return fmt.Errorf("pending resurrection intent; obit reap or obit resurrect %d retries it", b.ID)
	}
	if p.Status != "recorded" || !sameBurial(b, *p.Burial) {
		return fmt.Errorf("cannot clean burial manifest %s: %s", path, p.Error)
	}
	if _, _, err := s.confirmRecordedBurial(b); err != nil {
		return err
	}
	return os.Remove(path)
}

// Confirm and audit before retiring the independent manifest. Both recovery
// and direct terminal actions use this step; failed auditing retains evidence.
func (s *Service) confirmRecordedBurial(b Burial) (confirmed Burial, checkErr, err error) {
	err = s.Store.Confirm(func(st *State) error {
		idx := st.BurialIndex(b.ID)
		if idx < 0 || !sameBurial(st.Burials[idx], b) {
			checkErr = fmt.Errorf("burial #%d changed before confirmation; retry", b.ID)
			return checkErr
		}
		checkErr = s.reconcileChildren(st, &st.Burials[idx])
		confirmed = st.Burials[idx]
		return checkErr
	})
	if commitFailed(err) {
		return
	}
	err = errors.Join(err, s.Store.RecordCommitted(Event{Time: s.now(), Action: ActionBuried, ID: confirmed.ID, Path: confirmed.Path, Detail: "confirmed after an unconfirmed commit"}))
	return
}

// PendingGraves never creates locks, directories or migrations, so doctor can
// use it even when the ledger or graveyard is inaccessible.
func (s *Service) PendingGraves() ([]PendingGrave, error) {
	s = s.WithContext(s.Store.Context())
	s.readOnlyOwners = true
	st, err := s.Store.Inspect()
	if err != nil {
		return nil, err
	}
	return s.pendingGraves(st)
}

func (s *Service) pendingGraves(st State) ([]PendingGrave, error) {
	roots := s.resolveGraveyardRoots(st)
	all, err := s.scanGraveyardRoots(st, roots)
	pending := make([]PendingGrave, 0, len(all))
	for _, p := range all {
		if p.Status != "recorded" {
			pending = append(pending, p)
		}
	}
	for _, b := range st.Burials {
		if b.Kind == KindContainer {
			continue
		}
		if slices.ContainsFunc(pending, func(p PendingGrave) bool { return p.Number() == b.ID }) {
			continue
		}
		if b.IdentityMismatch != "" {
			pending = append(pending, PendingGrave{Manifest: filepath.Dir(b.Grave), Burial: &b, id: b.ID, Status: "identity-mismatch", Error: b.IdentityMismatch})
			continue
		}
		if b.PurgeIntent != nil {
			pending = append(pending, PendingGrave{Manifest: filepath.Dir(b.Grave), Burial: &b, id: b.ID, Status: "purging", Error: b.LastPurgeError})
			continue
		}
		if _, err := os.Lstat(b.Grave); !errors.Is(err, os.ErrNotExist) {
			continue
		}
		if _, checkErr := s.verifyGrave(b); errors.Is(checkErr, errEmptyGrave) {
			pending = append(pending, PendingGrave{Manifest: filepath.Dir(b.Grave), Burial: &b, id: b.ID, Status: "empty", Error: checkErr.Error()})
		}
	}
	for _, it := range st.Items {
		if it.Identity == nil {
			continue
		}
		path, locateErr := s.FS.Locate(*it.Identity)
		if locateErr == nil && s.insideGraveyard(roots, path) {
			found := false
			for _, p := range all {
				found = found || p.id == it.ID || p.Burial != nil && slices.Contains(p.Burial.Contains, it.ID)
			}
			if !found {
				pending = append(pending, PendingGrave{Manifest: path, id: it.ID, Status: "unverifiable", Error: "tracked object is in the graveyard without a matching manifest"})
			}
		}
	}
	return pending, err
}
