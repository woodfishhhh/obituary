package lifecycle

import (
	"errors"

	"github.com/woodfishhhh/obituary/internal/docker"
)

func (s *Service) stopContainer(item Item, dryRun bool) (out Outcome) {
	out = Outcome{Item: item, Path: item.Path, Action: ActionRetry}
	p := ContainerIntent{Operation: "stop", Phase: "prepared", Item: &item}
	if dryRun {
		_, err := s.inspectContainer(p)
		if err != nil {
			out.Detail = err.Error()
		} else {
			out.Action = ActionBuried
		}
		return out
	}
	out.err = s.withGraveyardLock(func() error {
		if err := s.bindContainer(&p); err != nil {
			out.Detail = err.Error()
			return nil
		}
		if err := s.beginContainerIntent(&p); err != nil {
			if errors.Is(err, ErrUnsafe) {
				out.Action = ActionSkipped
				return nil
			}
			return err
		}
		got, err := s.inspectContainer(p)
		if err == nil && got.AutoRemove {
			err = docker.ErrAutoRemove
		}
		if err == nil {
			var writeErr error
			err, writeErr = s.sendContainerStop(&p)
			if writeErr != nil {
				return writeErr
			}
		}
		if err != nil {
			out.Detail = err.Error()
			if auditErr := s.containerActionError(&p, err); auditErr != nil {
				return auditErr
			}
			if p.Phase == "sent" {
				return nil
			}
		}
		return s.settleContainerStop(p, &out)
	})
	return out
}

func (s *Service) sendContainerStop(p *ContainerIntent) (error, error) {
	var writeErr error
	beforeSend := func(got docker.Container) error {
		writeErr = s.updateContainerIntent(p, func(p *ContainerIntent) {
			p.Phase = "sent"
			p.WasRunning = &got.Running
			p.StopTimeout = got.StopTimeout
			p.RunningSince = nil
		})
		return writeErr
	}
	err := s.Docker.Stop(s.Store.Context(), *containerIntentIdentity(*p), beforeSend)
	if errors.Is(err, docker.ErrNotSent) && p.Phase == "sent" {
		writeErr = s.updateContainerIntent(p, func(p *ContainerIntent) { p.Phase = "prepared" })
	}
	if err == nil {
		writeErr = s.updateContainerIntent(p, func(p *ContainerIntent) { p.Phase = "confirmed" })
	}
	return err, writeErr
}

func (s *Service) settleContainerStop(p ContainerIntent, out *Outcome) error {
	if err := s.bindContainer(&p); err != nil {
		out.Detail = err.Error()
		return nil
	}
	if p.Diagnostic != "" {
		out.Detail = p.Diagnostic
		return nil
	}
	got, err := s.inspectContainer(p)
	if errors.Is(err, docker.ErrNotFound) {
		return s.loseContainer(p, out)
	}
	if err != nil {
		out.Detail = err.Error()
		return nil
	}
	if got.AutoRemove {
		out.Detail = docker.ErrAutoRemove.Error()
		return nil
	}
	// A running observation cannot disprove an engine request still in flight.
	if got.Running && (p.Phase == "sent" || p.Phase == "") {
		return s.observeRunningStop(p, out)
	}
	return s.commitContainerStop(p, out, got)
}

func (s *Service) commitContainerStop(p ContainerIntent, out *Outcome, got docker.Container) error {
	resume := false
	err := s.Store.UpdateGraveyard(func(st *State) error {
		if err := checkContainerIntent(st, p); err != nil {
			return err
		}
		if !sameContainerItem(st, *p.Item) {
			next := containerIntent(st, containerIntentID(p))
			next.Operation = "resume"
			if !got.Running && p.Phase != "prepared" {
				next.Phase = "confirmed"
			}
			next.StoppedByUs = !got.Running && p.WasRunning != nil && *p.WasRunning && p.Phase != "prepared"
			p = *next
			resume = true
			return nil
		}
		if got.Running {
			dropContainerIntent(st, p)
			out.Detail = "container is still running; retry original lifespan"
			return nil
		}
		s.commitContainerBurial(st, p, out)
		return nil
	})
	if err != nil {
		out.Action, out.Burial = ActionRetry, nil
		out.Detail = "container stop needs ledger recovery: " + err.Error()
		return err
	}
	if resume {
		return s.resumeContainer(p, out)
	}
	return nil
}

func (s *Service) observeRunningStop(p ContainerIntent, out *Outcome) error {
	out.Detail = "container stop result unknown; awaiting engine completion"
	// Infinite grace and legacy requests have no finite completion bound.
	if p.Phase != "sent" || p.WasRunning == nil || p.StopTimeout != nil && *p.StopTimeout < 0 {
		return nil
	}
	if p.RunningSince == nil {
		now := s.now()
		return s.updateContainerIntent(&p, func(p *ContainerIntent) { p.RunningSince = &now })
	}
	wait := (docker.Container{Identity: *containerIntentIdentity(p), StopTimeout: p.StopTimeout}).StopWait()
	if s.now().Sub(*p.RunningSince) < wait {
		return nil
	}
	// Observations across the full grace period permit retry, without proving
	// what happened between observations or excluding a delayed engine request.
	return s.Store.UpdateGraveyard(func(st *State) error {
		if err := checkContainerIntent(st, p); err != nil {
			return err
		}
		dropContainerIntent(st, p)
		queueAudit(st, Event{Time: s.now(), Action: "container-stop-retry", ID: containerIntentID(p), Path: p.Item.Path, Detail: "container observed running across stop grace period plus transport margin; original lifespan may retry"})
		out.Detail = "container still running after stop grace period; retry original lifespan"
		return nil
	})
}

func (s *Service) commitContainerBurial(st *State, p ContainerIntent, out *Outcome) {
	b := s.containerBurial(*p.Item)
	idx := st.Index(containerIntentID(p))
	st.Items = append(st.Items[:idx], st.Items[idx+1:]...)
	st.Burials = append(st.Burials, b)
	st.Stats.Buried++
	dropContainerIntent(st, p)
	queueAudit(st, Event{Time: s.now(), Action: ActionBuried, ID: b.ID, Path: b.Path, Detail: "docker stop confirmed; writable layer retained"})
	out.Action, out.Burial, out.Detail = ActionBuried, &b, ""
}

func (s *Service) containerBurial(item Item) Burial {
	return Burial{Kind: KindContainer, Container: item.Container, ID: item.ID, Path: item.Path, BuriedAt: s.now(), PurgeAt: s.now().Add(s.retention()), Reason: ReasonExpired, Note: item.Note, Source: item.Source, Project: item.Project, Session: item.Session}
}
