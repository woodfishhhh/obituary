package core

import (
	"errors"

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
			blocked[p.id()] = true
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
	got, err := s.inspectContainer(p)
	action, detail := "", ""
	switch {
	case errors.Is(err, docker.ErrNotFound) && p.Operation == "rm":
		action, detail = ActionPurged, "docker rm confirmed; volumes retained"
	case errors.Is(err, docker.ErrNotFound):
		action, detail = ActionLost, "container removed externally on original Docker daemon"
	case err != nil:
		r.Error = err.Error()
		return nil
	case got.Running:
		action, detail = ActionResurrected, "container running; resurrection confirmed"
	default:
		r.Error = "container remains stopped; operation cleared for retry"
	}
	err = s.Store.UpdateGraveyard(func(st *State) error {
		if err := st.checkContainerIntent(p); err != nil {
			return err
		}
		if err := checkContainerBurial(st, *p.Burial); err != nil {
			return err
		}
		st.dropContainerIntent(p)
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

func (s *Service) checkContainerRemoval(p ContainerIntent) error {
	if p.Operation != "rm" || len(p.RemovalIDs) == 0 {
		return nil
	}
	ids, err := s.Docker.ListIDs(docker.WithTarget(s.Store.Context(), p.identity().Target))
	if err != nil {
		return err
	}
	return docker.CheckRemoval(p.RemovalIDs, ids, p.identity().ID)
}

func (s *Service) commitContainerRetry(out Outcome) error {
	return s.Store.UpdateGraveyard(func(st *State) error {
		// A pending stop owns the original revision until its effect is known.
		if st.containerIntent(out.Item.ID) == nil {
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
	return st.containerIntent(b.ID), nil
}
