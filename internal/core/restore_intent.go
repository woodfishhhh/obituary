package core

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

// Write restore metadata before moving the object and retain the manifest
// until its ledger commit succeeds, so an interrupted restore can be recovered.
type restoreIntent struct {
	Parent Identity `json:"parent"`
	Name   string   `json:"name"`
}

func validRestoreIntent(intent *restoreIntent, identity *Identity) bool {
	return validRestoreParent(intent, identity) && validRestoreName(intent.Name)
}

func validRestoreParent(intent *restoreIntent, identity *Identity) bool {
	if intent == nil || identity == nil {
		return false
	}
	raw, err := hex.DecodeString(intent.Parent.FileID)
	if err != nil || len(raw) != 16 || intent.Parent.Serial != identity.Serial || !samePath(intent.Parent.Volume, identity.Volume) {
		return false
	}
	return true
}

func validRestoreName(name string) bool {
	// IsLocal reuses Go's Windows device-name rules. It does not enforce a
	// single component, forbidden Win32 characters, or the NTFS UTF-16 limit.
	if !filepath.IsLocal(name) || name == "." || !utf8.ValidString(name) ||
		strings.ContainsAny(name, `\/:<>"|?*`) || strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return false
	}
	units := 0
	for _, r := range name {
		units += utf16.RuneLen(r)
		if r < 32 || units > 255 {
			return false
		}
	}
	return true
}

// Advice under store.lock only reads this small metadata file. Resolving the
// restored identity can enumerate a large parent and must stay outside it.
func pendingRestoreHint(b Burial) string {
	manifest := filepath.Dir(b.Grave) + ".json"
	info, err := os.Lstat(manifest)
	if err != nil || !info.Mode().IsRegular() || info.Size() > manifestBytes {
		return ""
	}
	data, err := readShared(manifest)
	if err != nil {
		return ""
	}
	var m burialManifest
	if json.Unmarshal(data, &m) != nil || m.Version != 2 || m.Resurrect == nil {
		return ""
	}
	return fmt.Sprintf("unfinished resurrection; first use obit resurrect %d or obit reap; if unverifiable, obit purge %d explicitly abandons this record", b.ID, b.ID)
}

func writeRestoreIntent(b Burial, parent RestoreParent, name string) error {
	intent := &restoreIntent{Parent: parent.Identity(), Name: name}
	if !validRestoreIntent(intent, b.Identity) {
		return errors.New("invalid resurrection destination identity or name; the intent was not written")
	}
	b.Size = 0 // accounting can still finish concurrently; the ledger owns it
	return writeGraveManifest(filepath.Dir(b.Grave)+".json", burialManifest{
		Version: 2, Burial: b, Resurrect: intent,
	})
}

func (s *Service) inspectRestoreIntent(p PendingGrave, st State) PendingGrave {
	b, intent := *p.Burial, p.restore
	fail := func(err error) PendingGrave {
		p.Status, p.Error = "unverifiable", "cannot verify resurrection: "+err.Error()+"; "+abandonHint(b.ID)
		return p
	}
	if !validRestoreParent(intent, b.Identity) {
		return fail(errors.New("invalid destination identity or name"))
	}
	idx := st.BurialIndex(b.ID)
	if idx >= 0 {
		if !sameBurial(st.Burials[idx], b) {
			return fail(errors.New("intent conflicts with recorded burial"))
		}
		b = st.Burials[idx]
		p.Burial = &b
	}
	if !validRestoreName(intent.Name) {
		// Older writers could persist an overlong collision name. An exact
		// recorded burial with its original object still inside proves no move
		// happened. Never search a destination or abandon using invalid intent.
		if idx >= 0 {
			if _, err := s.verifyGrave(b); err == nil {
				p.Status = "restore-ready"
				return p
			}
		}
		return fail(errors.New("invalid destination identity or name"))
	}
	p.restoreValid = true
	if idx < 0 {
		// Only metadata cleanup remains after the durable removal of the index.
		if st.Index(b.ID) >= 0 || st.ProposalIndex(b.ID) >= 0 || st.NextID <= b.ID {
			return fail(errors.New("intent ID is not a committed burial"))
		}
		if err := s.checkCommittedGrave(b); err != nil {
			return fail(err)
		}
		p.Status = "restore-committed"
		if i := st.cleanupIndex(b.ID); i >= 0 {
			c := st.RestoreCleanups[i]
			if c.Burial.Grave != "" && !sameBurial(c.Burial, b) {
				return fail(errors.New("cleanup receipt conflicts with manifest"))
			}
			if c.Kind != ActionResurrected && c.Kind != ActionAbandoned {
				return fail(errors.New("unknown cleanup kind"))
			}
			if c.Restore != nil && !validRestoreIntent(c.Restore, b.Identity) {
				return fail(errors.New("invalid destination identity or name"))
			}
			if c.Kind == ActionAbandoned {
				p.Status = "restore-abandoned"
			}
			p.cleanup = &c
		}
		return p
	}
	dir, err := s.graveDir(b)
	if err != nil {
		return fail(err)
	}
	got, err := s.FS.Identify(dir)
	if err != nil {
		return fail(err)
	}
	if got != *b.GraveID {
		return fail(errors.New("grave directory identity changed"))
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fail(err)
	}
	if len(entries) != 0 {
		if _, err := s.verifyGrave(b); err != nil {
			return fail(err)
		}
		p.Status = "restore-ready" // interrupted before the move; safe to retry normally
		return p
	}
	path, err := s.FS.FindRestored(intent.Parent, *b.Identity)
	if err != nil {
		return fail(err)
	}
	p.Status, p.Path = "resurrected", path
	return p
}

func (s *Service) checkCommittedGrave(b Burial) error {
	dir, err := s.graveDir(b)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	got, err := s.FS.Identify(dir)
	if s.missingOnOnlineVolume(dir, err, burialVolume(b)) {
		return nil
	}
	if err != nil {
		return err
	}
	if got != *b.GraveID {
		return errors.New("grave directory identity changed")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return errors.New("ledger contradicts the nonempty grave; resurrection is not proven committed")
	}
	return nil
}

// Cleanup cannot change the outcome of a durable ledger commit.
type restoreResult struct {
	Committed bool
	Path      string
	Warning   error
}

// Caller owns graveyard.lock; count exactly once, in the index-removal commit.
func (s *Service) commitResurrection(b Burial, path, detail string, intent *restoreIntent) (restoreResult, error) {
	committed := false
	err := s.Store.UpdateGraveyard(func(st *State) error {
		if j := st.BurialIndex(b.ID); j >= 0 {
			if !sameBurial(st.Burials[j], b) {
				return fmt.Errorf("burial changed during resurrection: %w", ErrUnsafe)
			}
			st.spareRestoredWatch(st.Burials[j], path, s.now())
			s.spareWatchObject(st, b.WatchID, b.Identity, path, "restored")
			s.spareAutomaticObject(st, b.Identity, path)
			st.Stats.Resurrected++
			st.Stats.ResurrectedBytes += st.Burials[j].Size
			st.Burials = append(st.Burials[:j], st.Burials[j+1:]...)
			st.RestoreCleanups = append(st.RestoreCleanups, RestoreCleanup{Kind: ActionResurrected, Burial: b, Path: path, Restore: intent, CommittedAt: s.now()})
			committed = true
		}
		return nil
	})
	if commitFailed(err) {
		return restoreResult{Path: path}, fmt.Errorf("resurrection moved to %s; ledger commit needs retry: %w", path, err)
	}
	if committed {
		err = errors.Join(err, s.Store.RecordCommitted(Event{Time: s.now(), Action: ActionResurrected, ID: b.ID, Path: path, Detail: detail}))
	}
	if err != nil {
		return restoreResult{Committed: true, Path: path}, err
	}
	return restoreResult{Committed: true, Path: path, Warning: s.cleanRestoreIntent(b)}, err
}

func (s *Service) cleanRestoreIntent(b Burial) error {
	// Keep the empty grave until commit, so recovery can verify its identity.
	dir := filepath.Dir(b.Grave)
	if got, err := s.FS.Identify(dir); err == nil && b.GraveID != nil && got == *b.GraveID {
		if err := os.Remove(dir); err != nil {
			return err
		}
	} else if !s.missingOnOnlineVolume(dir, err, burialVolume(b)) {
		if err == nil {
			return errors.New("cannot clean restored grave: <nil>")
		}
		return fmt.Errorf("cannot clean restored grave: %w", err)
	}
	if err := os.Remove(dir + ".json"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return s.retireCleanup(b)
}

func (s *Service) recoverRestoreIntent(p PendingGrave) (restoreResult, error) {
	switch p.Status {
	case "resurrected":
		return s.commitResurrection(*p.Burial, p.Path, "recovered resurrection from "+p.Manifest, p.restore)
	case "restore-committed":
		path, locateErr := p.Path, error(nil)
		if p.restore != nil {
			path, locateErr = s.FS.FindRestored(p.restore.Parent, *p.Burial.Identity)
		} else {
			locateErr = errors.New("cleanup receipt lacks destination identity")
		}
		if locateErr != nil {
			locateErr = fmt.Errorf("resurrection already committed; actual restored path unavailable: %w", locateErr)
		}
		if err := s.retainCleanup(p, ActionResurrected, path); err != nil {
			return restoreResult{Path: path}, errors.Join(locateErr, err)
		}
		// Audit confirmation before retiring evidence; retries after audit or
		// cleanup failure may repeat this event, without repeating statistics.
		auditPath := path
		if auditPath == "" {
			auditPath = p.Burial.Path
		}
		if err := s.Store.RecordCommitted(Event{Time: s.now(), Action: ActionResurrected, ID: p.Burial.ID, Path: auditPath, Detail: "confirmed after an unconfirmed commit"}); err != nil {
			return restoreResult{Committed: true, Path: path, Warning: locateErr}, err
		}
		return restoreResult{Committed: true, Path: path, Warning: errors.Join(locateErr, s.cleanRestoreIntent(*p.Burial))}, nil
	case "restore-abandoned":
		if err := s.retainCleanup(p, ActionAbandoned, ""); err != nil {
			return restoreResult{}, err
		}
		detail := "confirmed after an unconfirmed commit; reason=restored object was not found in its planned parent"
		if p.restore != nil {
			detail = fmt.Sprintf("restore parent=%+v; planned name=%q; %s", p.restore.Parent, p.restore.Name, detail)
		}
		if err := s.Store.RecordCommitted(Event{Time: s.now(), Action: ActionAbandoned, ID: p.Burial.ID, Path: p.Burial.Path, Detail: detail}); err != nil {
			return restoreResult{}, err
		}
		return restoreResult{Warning: s.cleanAbandonedRestore(*p.Burial)}, nil
	case "restore-ready":
		if _, err := s.verifyGrave(*p.Burial); err != nil {
			return restoreResult{Warning: err}, nil
		}
		return restoreResult{Warning: os.Remove(p.Manifest)}, nil
	}
	return restoreResult{}, errors.New(p.Error)
}
