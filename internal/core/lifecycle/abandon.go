package lifecycle

import (
	"errors"
	"fmt"
	"os"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
)

const ActionAbandoned = ledger.ActionAbandoned

func abandonHint(id int64) string {
	return fmt.Sprintf("unverifiable resurrection; obit purge %d explicitly abandons this record after verifying its empty grave", id)
}

// Explicit selection is required. Everything outside the verified empty grave
// remains untouched, even when the original object has moved or been replaced.
func (s *Service) purgeRestoreIntent(b Burial, p PendingGrave, dryRun bool) (GraveResult, error) {
	r := GraveResult{Ref: fmt.Sprintf("#%d", b.ID), Burial: &b, Action: ActionBlocked}
	refuse := func(err error) (GraveResult, error) { r.Error = err.Error(); return r, nil }
	if !p.restoreValid || !sameBurial(b, *p.Burial) {
		return refuse(ErrUnsafe)
	}
	dir, err := s.graveDir(b)
	if err != nil {
		return refuse(err)
	}
	got, err := s.FS.Identify(dir)
	if err != nil {
		return refuse(err)
	}
	if got != *b.GraveID {
		return refuse(fmt.Errorf("grave directory identity changed: %w", ErrUnsafe))
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return refuse(err)
	}
	if len(entries) != 0 {
		return refuse(fmt.Errorf("grave is not empty; obit reap or obit resurrect %d retries the pending intent: %w", b.ID, ErrUnsafe))
	}
	path, err := s.FS.FindRestored(p.restore.Parent, *b.Identity)
	if err == nil {
		r.Action, r.Path = ActionResurrected, path
		if dryRun {
			return r, nil
		}
		restored, err := s.commitResurrection(b, path, "recovered during explicit purge", p.restore)
		r.Warning = errorText(restored.Warning)
		return r, err
	}
	if !errors.Is(err, ErrNotFound) {
		return refuse(fmt.Errorf("%s: %w", abandonHint(b.ID), err))
	}
	r.Action = ActionAbandoned
	if dryRun {
		return r, nil
	}
	reason := err.Error()
	err = s.Store.UpdateGraveyard(func(st *State) error {
		idx := st.BurialIndex(b.ID)
		if idx < 0 || !sameBurial(st.Burials[idx], b) {
			return ErrUnsafe
		}
		st.Burials = append(st.Burials[:idx], st.Burials[idx+1:]...)
		st.RestoreCleanups = append(st.RestoreCleanups, RestoreCleanup{Kind: ActionAbandoned, Burial: b, Restore: p.restore, CommittedAt: s.now()})
		return nil
	})
	if commitFailed(err) {
		r.Action = ""
		return r, err
	}
	detail := fmt.Sprintf("restore parent=%+v; planned name=%q; reason=%s", p.restore.Parent, p.restore.Name, reason)
	err = errors.Join(err, s.Store.RecordCommitted(Event{Time: s.now(), Action: ActionAbandoned, ID: b.ID, Path: b.Path, Detail: detail}))
	if err == nil {
		r.Warning = errorText(s.cleanAbandonedRestore(b))
	}
	return r, err
}

func (s *Service) cleanAbandonedRestore(b Burial) error {
	return s.cleanRestoreIntent(b)
}
