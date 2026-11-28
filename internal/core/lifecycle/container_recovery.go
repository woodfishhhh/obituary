package lifecycle

import (
	"errors"

	"github.com/woodfishhhh/obituary/internal/core/extres"
	"github.com/woodfishhhh/obituary/internal/docker"
)

func (s *Service) recoverContainerStops(state *State) ([]Outcome, map[int64]bool, error) {
	var rows []Outcome
	blocked := map[int64]bool{}
	err := s.withGraveyardLock(func() error {
		current, err := s.Store.ReadSnapshot()
		if err != nil {
			return err
		}
		for _, p := range current.ContainerIntents {
			if p.Item == nil {
				continue
			}
			blocked[containerIntentID(p)] = true
			out := Outcome{Item: *p.Item, Path: p.Item.Path, Action: ActionRetry}
			if p.Operation == "resume" {
				err = s.resumeContainer(p, &out)
			} else {
				err = s.settleContainerStop(p, &out)
			}
			rows = append(rows, out)
			if err != nil {
				return err
			}
			if out.Action == ActionRetry {
				if err := s.commitContainerRetry(out); err != nil {
					return err
				}
			}
		}
		*state, err = s.Store.ReadSnapshot()
		return err
	})
	return rows, blocked, err
}

// Recovery observes the effect before deciding whether an operation can retire.
// An absent container with a removal intent is committed without another rm.
func (s *Service) settleContainerGrave(p ContainerIntent, r *GraveResult) error {
	if err := s.bindContainer(&p); err != nil {
		r.Error = err.Error()
		return nil
	}
	if p.Diagnostic != "" {
		r.Error = p.Diagnostic
		return nil
	}
	if err := s.checkContainerRemoval(p); err != nil {
		r.Error = err.Error()
		return s.containerActionError(&p, err)
	}
	action, detail, message, err := s.inspectSettledContainer(p)
	if err != nil {
		r.Error = err.Error()
		return nil
	}
	r.Error = message
	err = s.Store.UpdateGraveyard(func(st *State) error {
		if err := checkContainerIntent(st, p); err != nil {
			return err
		}
		if err := checkContainerBurial(st, *p.Burial); err != nil {
			return err
		}
		dropContainerIntent(st, p)
		if action != "" {
			finishContainerGrave(st, *p.Burial, action, s.now(), detail)
		}
		return nil
	})
	if err == nil && action != "" {
		r.Action, r.Error = action, ""
		if action == ActionResurrected {
			r.Path = p.Burial.Path
		}
	}
	return err
}

func (s *Service) inspectSettledContainer(p ContainerIntent) (string, string, string, error) {
	obs, got, err := s.verifyContainerProvider(p)
	switch {
	case errors.Is(err, docker.ErrNotFound) && p.Operation == "rm":
		return ActionPurged, "docker rm confirmed; volumes retained", "", nil
	case errors.Is(err, docker.ErrNotFound):
		return ActionLost, "container removed externally on original Docker daemon", "", nil
	case err != nil:
		return "", "", "", err
	case got.Running:
		return ActionResurrected, "container running; resurrection confirmed", "", nil
	case p.Operation == "rm":
		if obs.Verdict == extres.VerdictReclaimed {
			return ActionResurrected, "container restarted since burial; kept stopped", "", nil
		}
		reclaimed, err := got.Reclaimed(*p.Burial.Container)
		if err != nil {
			return "", "", err.Error(), nil
		}
		if reclaimed {
			return ActionResurrected, "container restarted since burial; kept stopped", "", nil
		}
		return "", "", "container remains stopped; operation cleared for retry", nil
	}
	return "", "", "container remains stopped; operation cleared for retry", nil
}

func (s *Service) checkContainerRemoval(p ContainerIntent) error {
	if p.Operation != "rm" || len(p.RemovalIDs) == 0 {
		return nil
	}
	ids, err := s.listContainerIDs(p)
	if err != nil {
		return err
	}
	return docker.CheckRemoval(p.RemovalIDs, ids, containerIntentIdentity(p).ID)
}

func (s *Service) commitContainerRetry(out Outcome) error {
	return s.Store.UpdateHot(func(st *State) error {
		// A pending stop owns the original revision until its effect is known.
		if containerIntent(st, out.Item.ID) == nil {
			s.commitOutcome(st, out)
		}
		return nil
	})
}

func (s *Service) pendingContainerGrave(b Burial) (*ContainerIntent, error) {
	st, err := s.Store.ReadSnapshot()
	if err != nil {
		return nil, err
	}
	return containerIntent(&st, b.ID), nil
}
