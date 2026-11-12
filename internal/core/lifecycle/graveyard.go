package lifecycle

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/pathutil"
)

const (
	ActionBuried          = ledger.ActionBuried
	ActionResurrected     = ledger.ActionResurrected
	ActionPurged          = ledger.ActionPurged
	ActionLost            = ledger.ActionLost
	ActionRecoveryPending = ledger.ActionRecoveryPending
	ActionNotExecuted     = ledger.ActionNotExecuted
	ActionBlocked         = ledger.ActionBlocked
	ReasonExpired         = ledger.ReasonExpired
	ReasonBury            = ledger.ReasonBury
)

// GraveResult describes what happened to one burial or requested path.
type GraveResult struct {
	Ref             string  `json:"ref"`
	Action          string  `json:"action,omitempty"`
	Burial          *Burial `json:"burial,omitempty"`
	Path            string  `json:"path,omitempty"`  // where a resurrected item now lies
	Grave           string  `json:"grave,omitempty"` // moved location when burial commit is still pending
	Detail          string  `json:"detail,omitempty"`
	Error           string  `json:"error,omitempty"`
	Warning         string  `json:"warning,omitempty"`
	RecoveryCount   int     `json:"recoveryCount,omitempty"` // supplemental receipts, before display limiting
	RecoveryOmitted int     `json:"recoveryOmitted,omitempty"`
}

func errorText(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

// GraveFailed reports whether any result carries an error.
func GraveFailed(results []GraveResult) bool {
	for _, r := range results {
		if r.Error != "" {
			return true
		}
	}
	return false
}

// ledgerKey tells ledgers apart on volumes they share: two data directories
// must never share a grave directory.
func (s *Service) ledgerKey() string {
	sum := sha256.Sum256([]byte(strings.ToLower(filepath.Clean(s.Store.Home))))
	return hex.EncodeToString(sum[:6])
}

// graveyardRoot is the graveyard on the same volume as path, so a burial is
// always a rename and never a copy: inside the data directory on its own
// volume, `<volume>\.obituary\graveyard\<ledger>` elsewhere.
func (s *Service) graveyardRoot(path string) string {
	sourceRoot, homeRoot := pathutil.VolumeRoot(path), pathutil.VolumeRoot(s.Store.Home)
	// Abs is Win32 string normalization, not an object/volume probe. It also
	// accepts the equivalent extended roots found in older ledger spellings.
	if plain, err := pathutil.Abs(sourceRoot); err == nil {
		sourceRoot = plain
	}
	if plain, err := pathutil.Abs(homeRoot); err == nil {
		homeRoot = plain
	}
	if strings.EqualFold(sourceRoot, homeRoot) {
		return filepath.Join(s.Store.Home, "graveyard")
	}
	return filepath.Join(sourceRoot, ".obituary", "graveyard", s.ledgerKey())
}

// prepareGrave creates an exclusive destination without moving the object.
func (s *Service) prepareGrave(path string, id int64, at time.Time) (string, *Identity, error) {
	root := s.graveyardRoot(path)
	if err := s.checkGraveyardOwner(root, true); err != nil {
		return "", nil, err
	}
	var dir string
	for attempt := 0; ; attempt++ {
		suffix := make([]byte, 3)
		if _, err := rand.Read(suffix); err != nil {
			return "", nil, err
		}
		dir = filepath.Join(root, fmt.Sprintf("%d-%s-%s", id, at.UTC().Format("20060102T150405Z"), hex.EncodeToString(suffix)))
		err := s.FS.MakeGrave(dir)
		if err == nil {
			break
		}
		if !errors.Is(err, fs.ErrExist) || attempt >= 4 {
			return "", nil, fmt.Errorf("prepare grave: %w", err)
		}
	}
	graveID, err := s.FS.Identify(dir)
	if err != nil {
		_ = os.Remove(dir) // the empty grave just made
		return "", nil, err
	}
	grave := filepath.Join(dir, filepath.Base(path))
	return grave, &graveID, nil
}

// graveDir returns the grave directory of b if it lies exactly one level
// under b's graveyard root and carries b's id.
func (s *Service) graveDir(b Burial) (string, error) {
	dir := filepath.Dir(b.Grave)
	if err := s.checkGraveyardOwner(s.graveyardRoot(b.Path), false); err != nil {
		return "", err
	}
	root, err := s.FS.RealPath(s.graveyardRoot(b.Path))
	if err != nil {
		if s.missingOnOnlineVolume(s.graveyardRoot(b.Path), err, burialVolume(b)) {
			_, graveErr := os.Lstat(dir)
			if s.missingOnOnlineVolume(dir, graveErr, burialVolume(b)) {
				return "", fmt.Errorf("graveyard root and grave directory are gone: %w", ErrNotFound)
			}
			return "", fmt.Errorf("#%d: expected graveyard root is missing; burial layout or ownership is unsupported: %w", b.ID, ErrUnsafe)
		}
		return "", fmt.Errorf("resolve graveyard root: %v", err) //nolint:errorlint // An offline volume's ErrNotFound must not discard the burial record.
	}
	parent, err := s.FS.RealPath(filepath.Dir(dir))
	if err != nil {
		return "", fmt.Errorf("resolve grave parent: %v", err) //nolint:errorlint // Parent lookup failure does not prove the grave is gone; retain it for retry.
	}
	sameRoot, err := s.sameDirectory(parent, root)
	if err != nil {
		return "", err
	}
	ok := sameRoot &&
		strings.HasPrefix(filepath.Base(dir), fmt.Sprintf("%d-", b.ID)) &&
		samePath(filepath.Base(b.Grave), filepath.Base(b.Path))
	if !ok {
		return "", fmt.Errorf("#%d: %s is not a grave in the graveyard", b.ID, b.Grave)
	}
	return dir, nil
}

// Callers first resolve RealPath, which preserves a final junction. Both its
// location and identity must match: identity alone follows that junction into
// a foreign root, while Unicode-folded names alone can conflate distinct roots.
func (s *Service) sameDirectory(a, b string) (bool, error) {
	equal, err := s.FS.SameName(a, b)
	if err != nil || !equal {
		return false, err
	}
	first, err := s.FS.Identify(a)
	if err != nil {
		return false, err
	}
	second, err := s.FS.Identify(b)
	if err != nil {
		return false, err
	}
	return first.Serial == second.Serial && first.FileID == second.FileID, nil
}

var errEmptyGrave = errors.New("grave is empty")

func burialVolume(b Burial) *Identity {
	if b.GraveID != nil {
		return b.GraveID
	}
	return b.Identity
}

// verifyGrave is the check every purge and resurrection passes first. The
// grave directory must be where b's graveyard puts it, still be the very
// directory Obituary created (its file ID; a junction or swap changes it),
// hold nothing but the buried object, and that object must be the one buried.
// A tampered or stale ledger therefore cannot aim either at anything else.
func (s *Service) verifyGrave(b Burial) (string, error) {
	dir, err := s.graveDir(b)
	if err != nil {
		return "", err
	}
	if b.GraveID == nil || b.Identity == nil {
		return "", fmt.Errorf("#%d: the burial record lacks the identities needed to verify its grave", b.ID)
	}
	got, err := s.FS.Identify(dir)
	if err != nil {
		return "", fmt.Errorf("#%d: grave %s: %w", b.ID, dir, err)
	}
	if got != *b.GraveID {
		return "", fmt.Errorf("#%d: %s is no longer the grave Obituary made: %w", b.ID, dir, ErrUnsafe)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	if len(entries) == 0 {
		return dir, fmt.Errorf("#%d: %w; its object was removed outside Obituary; obit purge %d clears the record", b.ID, errEmptyGrave, b.ID)
	}
	if len(entries) != 1 || !strings.EqualFold(entries[0].Name(), filepath.Base(b.Grave)) {
		return "", fmt.Errorf("#%d: grave %s holds something besides the buried object: %w", b.ID, dir, ErrUnsafe)
	}
	got, err = s.FS.Identify(b.Grave)
	if err != nil {
		return "", fmt.Errorf("#%d: %w", b.ID, err)
	}
	if got != *b.Identity {
		return "", fmt.Errorf("#%d: %s is not the object that was buried: %w", b.ID, b.Grave, ErrUnsafe)
	}
	return dir, nil
}

func (s *Service) withGraveyardLock(fn func() error) error {
	return s.Store.WithLock("graveyard.lock", true, fn)
}

// GraveyardView combines recorded burials with manifests still needing recovery.
type GraveyardView struct {
	Burials  []Burial
	Pending  []PendingGrave
	Restored []GraveResult
}

// ReadGraveyard prevents recovery from moving a manifest into the ledger between
// the two reads. It reads without migration, then reconciles verified restore
// intents under the exclusive graveyard lock. Lock waits honor context/deadline.
func (s *Service) ReadGraveyard() (view *GraveyardView, err error) {
	s, flushAudit := s.auditBatch(false)
	defer func() { err = errors.Join(err, flushAudit()) }()
	err = s.Store.WithLock("graveyard.lock", true, func() error {
		st, err := s.Store.ReadSnapshot()
		if err != nil {
			return err
		}
		pending, scanErr := s.pendingGraves(st)
		view = &GraveyardView{Burials: st.Burials}
		for _, p := range pending {
			if p.Status != "resurrected" && p.Status != "restore-committed" && p.Status != "restore-ready" && p.Status != "restore-abandoned" {
				view.Pending = append(view.Pending, p)
				continue
			}
			if err := s.Store.Context().Err(); err != nil {
				return err
			}
			tx := s.WithContext(context.WithoutCancel(s.Store.Context()))
			r, err := tx.recoverRestoreIntent(p)
			scanErr = errors.Join(scanErr, err)
			if commitFailed(err) {
				p.Error = err.Error()
				view.Pending = append(view.Pending, p)
				continue
			}
			if r.Warning != nil {
				p.Error = r.Warning.Error()
				if r.Committed {
					p.Status, p.Path = "restore-committed", r.Path
				}
				view.Pending = append(view.Pending, p)
			}
			if r.Committed {
				view.Restored = append(view.Restored, GraveResult{Ref: fmt.Sprintf("#%d", p.Burial.ID), Action: ActionResurrected, Burial: p.Burial, Path: r.Path, Warning: errorText(r.Warning)})
				for i, b := range view.Burials {
					if b.ID == p.Burial.ID {
						view.Burials = append(view.Burials[:i], view.Burials[i+1:]...)
						break
					}
				}
			}
		}
		return scanErr
	})
	return view, err
}

type resolvedGraveyardRoot struct {
	path string
	real string
	err  error
}

func (s *Service) resolveGraveyardRoots(st State, paths ...string) []resolvedGraveyardRoot {
	return s.resolveMissingGraveyardRoots(nil, s.graveyardRoots(st, paths...))
}

func (s *Service) resolveMissingGraveyardRoots(resolved []resolvedGraveyardRoot, roots []string) []resolvedGraveyardRoot {
	for _, root := range roots {
		if slices.ContainsFunc(resolved, func(known resolvedGraveyardRoot) bool { return samePath(known.path, root) }) {
			continue
		}
		real, err := s.FS.RealPath(root)
		resolved = append(resolved, resolvedGraveyardRoot{root, real, err})
	}
	return resolved
}

// Cache successful resolutions only: the first burial can create a root that
// was missing at the start of this very operation.
func (s *Service) insideGraveyard(roots []resolvedGraveyardRoot, path string) bool {
	for i := range roots {
		root := &roots[i]
		if samePath(path, root.path) || isInside(path, root.path) {
			return true
		}
		if root.err != nil {
			root.real, root.err = s.FS.RealPath(root.path)
		}
		if root.err == nil && (samePath(path, root.real) || isInside(path, root.real)) {
			return true
		}
	}
	return false
}

func (s *Service) graveyardRoots(st State, paths ...string) []string {
	roots := []string{filepath.Join(s.Store.Home, "graveyard")}
	if st.GraveyardIndex != nil {
		paths = append(paths, st.GraveyardIndex.Volumes...)
	}
	for _, it := range st.Items {
		if it.Kind == KindContainer {
			continue
		}
		paths = append(paths, it.Path)
		if it.Identity != nil {
			paths = append(paths, it.Identity.Volume)
		}
	}
	for _, b := range st.Burials {
		if b.Kind == KindContainer {
			continue
		}
		paths = append(paths, b.Path)
	}
	for _, c := range st.RestoreCleanups {
		if c.Burial.Grave != "" {
			paths = append(paths, c.Burial.Path)
		}
	}
	for _, path := range paths {
		abs, err := pathutil.Abs(path)
		if err != nil {
			continue // an invalid input is reported by its command/ledger lookup
		}
		root := s.graveyardRoot(abs)
		found := false
		for _, known := range roots {
			found = found || samePath(root, known)
		}
		if !found {
			roots = append(roots, root)
		}
	}
	return roots
}

// Windows may report PATH_NOT_FOUND for a disconnected volume as well.
// Only a separately successful volume probe permits a lost-index conclusion.
func (s *Service) missingOnOnlineVolume(path string, err error, want *Identity) bool {
	if !errors.Is(err, os.ErrNotExist) && !errors.Is(err, ErrNotFound) {
		return false
	}
	volume := pathutil.VolumeRoot(path)
	if _, volumeErr := s.FS.RealPath(volume); volumeErr != nil {
		return false
	}
	got, volumeErr := s.FS.Identify(volume)
	return volumeErr == nil && (want == nil || got.Serial == want.Serial)
}
