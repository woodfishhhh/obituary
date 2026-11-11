package core

import (
	"errors"

	"github.com/woodfishhhh/obituary/internal/docker"
)

func (s *Service) resumeContainer(p ContainerIntent, out *Outcome) error {
	if err := s.bindContainer(&p); err != nil {
		out.Detail = err.Error()
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
	if p.Diagnostic != "" {
		out.Detail = p.Diagnostic
		return nil
	}
	if p.Phase == "prepared" {
		return s.finishContainerResume(p, out, false)
	}
	if p.WasRunning == nil {
		out.Detail = "legacy stop lacks initial running state; verify the original container before manual recovery"
		return nil
	}
	if got.Running && (p.Phase == "sent" || p.Phase == "") && !p.StoppedByUs {
		return s.observeRunningStop(p, out)
	}
	// Historical intents without the initial state cannot authorize an automatic start.
	compensate := p.StoppedByUs || p.WasRunning != nil && *p.WasRunning && !got.Running && p.Phase != "prepared"
	if !compensate || got.Running {
		return s.finishContainerResume(p, out, compensate)
	}
	return s.restartContainer(p, out)
}

func (s *Service) restartContainer(p ContainerIntent, out *Outcome) error {
	if err := s.updateContainerIntent(&p, func(p *ContainerIntent) { p.StoppedByUs = true; p.Phase = "confirmed" }); err != nil {
		return err
	}
	err := s.Docker.Start(s.Store.Context(), *p.identity())
	if auditErr := s.containerActionError(&p, err); auditErr != nil {
		return auditErr
	}
	var got docker.Container
	if err == nil {
		got, err = s.inspectContainer(p)
	}
	if errors.Is(err, docker.ErrNotFound) {
		return s.loseContainer(p, out)
	}
	if err != nil || !got.Running {
		out.Detail = "lifespan changed; container restart pending"
		if err != nil {
			out.Detail += ": " + err.Error()
		}
		return nil
	}
	return s.finishContainerResume(p, out, true)
}

func (s *Service) finishContainerResume(p ContainerIntent, out *Outcome, compensated bool) error {
	detail := "latest lifespan decision preserved; initial stopped state retained"
	if compensated {
		detail = "container running after concurrent lifespan change or spare"
	}
	err := s.Store.Update(func(st *State) error {
		if err := st.checkContainerIntent(p); err != nil {
			return err
		}
		st.dropContainerIntent(p)
		st.queueAudit(Event{Time: s.now(), Action: "container-resumed", ID: p.id(), Path: p.Item.Path, Detail: detail})
		return nil
	})
	if err == nil {
		out.Action, out.Detail = ActionSkipped, detail
	}
	return err
}

func (s *Service) loseContainer(p ContainerIntent, out *Outcome) error {
	err := s.Store.Update(func(st *State) error {
		if err := st.checkContainerIntent(p); err != nil {
			return err
		}
		if idx := st.Index(p.id()); idx >= 0 {
			current := st.Items[idx].Container
			if current == nil || !current.Same(*p.identity()) {
				return docker.ErrIdentity
			}
			st.Items = append(st.Items[:idx], st.Items[idx+1:]...)
		}
		st.dropContainerIntent(p)
		st.Stats.ContainersLost++
		st.queueAudit(Event{Time: s.now(), Action: ActionLost, ID: p.id(), Path: p.Item.Path, Detail: "container removed externally on original Docker daemon"})
		return nil
	})
	if err == nil {
		out.Action, out.Detail = ActionLost, "container removed externally"
	}
	return err
}
