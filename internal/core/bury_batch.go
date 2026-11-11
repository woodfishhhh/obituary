package core

import (
	"context"
	"errors"

	"github.com/woodfishhhh/obituary/internal/pathutil"
)

type burialRequest struct {
	index     int
	candidate *candidate
	item      Item
}

func (s *Service) buryBatches(paths []string, results []GraveResult, force bool, note, source string, recovery recovery) (finalErr error) {
	var batch []burialRequest
	flush := func() error {
		err := s.buryRequestBatch(batch, paths, results, force, note, source, recovery)
		batch = nil
		finalErr = errors.Join(finalErr, err)
		return err
	}
	for i, ref := range paths {
		abs, absErr := pathutil.Abs(ref)
		if len(batch) == sweepBatchSize || absErr == nil && s.burialOverlap(abs, batch) {
			if commitFailed(flush()) {
				return finalErr
			}
		}
		if err := s.Store.Context().Err(); err != nil {
			return errors.Join(finalErr, flush(), err)
		}
		c, row := s.inspectBuryRequest(ref, force, recovery)
		if c == nil {
			results[i] = row
			continue
		}
		batch = append(batch, burialRequest{index: i, candidate: c})
	}
	flush()
	return finalErr
}

// Ancestor and alias operands must observe earlier moves, including rmdir
// becoming empty after its child. Real paths also catch junction aliases.
func (s *Service) burialOverlap(path string, batch []burialRequest) bool {
	real, err := s.FS.RealPath(path)
	if err != nil {
		real = path
	}
	for _, request := range batch {
		previous, err := s.FS.RealPath(request.candidate.path)
		if err != nil {
			previous = request.candidate.path
		}
		if samePath(real, previous) || isInside(real, previous) || isInside(previous, real) {
			return true
		}
	}
	return false
}

func (s *Service) inspectBuryRequest(ref string, force bool, recovery recovery) (*candidate, GraveResult) {
	r := GraveResult{Ref: ref}
	if s.deletionCheck != nil {
		ok, err := s.deletionCheck(ref)
		if err != nil {
			r.Error = err.Error()
		}
		if !ok || err != nil {
			return nil, r
		}
	}
	abs, err := pathutil.Abs(ref)
	if err != nil {
		r.Error = err.Error()
		return nil, r
	}
	if detail := recovery.roots[ProjectKey(s.graveyardRoot(abs))]; detail != "" {
		r.Action, r.Error = ActionBlocked, detail
		return nil, r
	}
	c, err := s.inspect(ref, force)
	if err != nil {
		return nil, s.recoveredBuryRef(ref, abs, recovery.burials, err)
	}
	return c, r
}

func (s *Service) reserveBuryBatch(batch []burialRequest, force bool, note, source string) (state State, err error) {
	err = s.Store.UpdateHot(func(st *State) error {
		for i := range batch {
			r := &batch[i]
			idx := st.FindIdentity(r.candidate.identity)
			if idx >= 0 {
				r.item = st.Items[idx]
			} else {
				r.item = Item{ID: st.NextID, CreatedAt: s.now(), ExpiresAt: s.now(), Source: "cli"}
				st.NextID++
			}
			c := r.candidate
			r.item.Path, r.item.IsDir, r.item.Identity, r.item.Force = c.path, c.isDir, &c.identity, force
			if note != "" {
				r.item.Note = note
			}
			if source != "" {
				r.item.Source = source
			}
		}
		state = *st
		return nil
	})
	return state, err
}

func (s *Service) buryRequestBatch(batch []burialRequest, paths []string, results []GraveResult, force bool, note, source string, recovery recovery) error {
	if len(batch) == 0 {
		return nil
	}
	state, err := s.reserveBuryBatch(batch, force, note, source)
	if commitFailed(err) {
		results[batch[0].index].Error = err.Error()
		return err
	}
	pending := make([]*preparedBurial, 0, len(batch))
	var prepErr error
	for _, r := range batch {
		if recovery.blocked[r.item.ID] {
			pending = append(pending, &preparedBurial{out: Outcome{Item: r.item, Path: r.item.Path, Action: ActionRetry, Detail: "unverifiable burial manifest; see obit doctor"}})
			continue
		}
		out := s.prepareSettlement(r.item, false, ReasonBury, recovery.locations, &state)
		p := &preparedBurial{out: out}
		if out.Action == "" && out.err == nil {
			p = s.prepareBurial(out, ReasonBury)
		}
		if p.out.err != nil {
			results[r.index], prepErr = s.finishBuryRef(paths[r.index], r.item, p.out, true)
			break
		}
		pending = append(pending, p)
	}
	return errors.Join(err, s.commitBuryRequests(pending, batch, paths, results), prepErr)
}

func (s *Service) commitBuryRequests(pending []*preparedBurial, batch []burialRequest, paths []string, results []GraveResult) error {
	rows, err := s.commitBurialBatch(pending, nil, ReasonBury)
	var measured []Burial
	for i, out := range rows {
		r := batch[i]
		results[r.index], _ = s.finishBuryRef(paths[r.index], r.item, out, true)
		if out.Burial != nil {
			measured = append(measured, *out.Burial)
		}
	}
	if len(measured) > 0 {
		// Finish accounting for committed moves even after caller cancellation.
		tx := s.WithContext(context.WithoutCancel(s.Store.Context()))
		err = errors.Join(err, tx.saveBurialSizes(measured))
	}
	return err
}
