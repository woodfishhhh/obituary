package lifecycle

import (
	"cmp"
	"errors"
)

// SettleVanished closes the items whose object was deleted outside Obituary
// before their lifespan ended. Nothing is left to bury, so they are recorded
// as vanished now instead of being listed and checked until they expire. The
// verdict and the commit are the sweep's own, under the sweep lock: an object
// that moved, or a different object at the path, is never closed, and a
// commit lands only on the revision that was checked. Due items are the
// sweep's.
func (s *Service) SettleVanished() ([]Outcome, error) {
	release, err := s.lockSweep()
	if err != nil {
		return nil, err
	}
	defer release()
	st, err := s.Store.ReadHot()
	if err != nil {
		return nil, err
	}
	var outcomes []Outcome
	var errs []error
	for _, it := range st.Items {
		if err := s.Store.Context().Err(); err != nil {
			return outcomes, errors.Join(append(errs, err)...)
		}
		if it.Kind == KindContainer || it.Identity == nil || it.Due(s.now()) || awaitingWatchBurial(&st, it) {
			continue
		}
		_, _, verdict, detail := s.locate(it)
		if verdict != ActionVanished {
			continue
		}
		out := Outcome{Item: it, Action: ActionVanished, Path: it.Path, Detail: cmp.Or(detail, "deleted outside Obituary before its lifespan ended")}
		if err := s.commit(out); err != nil {
			errs = append(errs, err)
			continue
		}
		outcomes = append(outcomes, out)
		errs = append(errs, s.Store.RecordCommitted(Event{Time: s.now(), Action: ActionVanished, ID: it.ID, Path: it.Path, Detail: out.Detail}))
	}
	return outcomes, errors.Join(errs...)
}
