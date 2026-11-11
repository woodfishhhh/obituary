package core

import (
	"errors"
	"fmt"
	"reflect"
	"time"

	"github.com/woodfishhhh/obituary/internal/docker"
)

func (s *Service) purgeContainerRequests(run *purgeRun, state State) ([]Burial, error) {
	var files []Burial
	for _, b := range run.requested {
		if b.Kind != KindContainer {
			files = append(files, b)
			continue
		}
		if err := s.purgeRequestedBurial(run, state, b, nil, ""); err != nil {
			return files, err
		}
	}
	return files, nil
}

func (s *Service) resurrectContainer(r *GraveResult, b Burial) error {
	pending, err := s.pendingContainerGrave(b)
	if err != nil {
		return err
	}
	if pending != nil {
		return s.settleContainerGrave(*pending, r)
	}
	if b.PurgeIntent != nil {
		r.Error = fmt.Sprintf("#%d: purge is in progress; retry obit purge %d", b.ID, b.ID)
		return nil
	}
	p := ContainerIntent{Operation: "start", Burial: &b}
	if err := s.bindContainer(&p); err != nil {
		r.Error = err.Error()
		return nil
	}
	if err := s.beginContainerIntent(&p); err != nil {
		return err
	}
	got, err := s.inspectContainer(p)
	if err == nil && !got.Running {
		err = s.Docker.Start(s.Store.Context(), *b.Container)
	}
	if err != nil {
		r.Error = err.Error()
		if auditErr := s.containerActionError(&p, err); auditErr != nil {
			return auditErr
		}
		if !errors.Is(err, docker.ErrNotFound) {
			return nil
		}
	}
	return s.settleContainerGrave(p, r)
}

func finishContainerGrave(st *State, b Burial, action string, at time.Time, detail string) {
	idx := st.BurialIndex(b.ID)
	st.Burials = append(st.Burials[:idx], st.Burials[idx+1:]...)
	switch action {
	case ActionResurrected:
		st.Stats.Resurrected++
	case ActionLost:
		st.Stats.ContainersLost++
	default:
		st.Stats.Purged++
	}
	queueAudit(st, Event{Time: at, Action: action, ID: b.ID, Path: b.Path, Detail: detail})
}

func checkContainerBurial(st *State, b Burial) error {
	idx := st.BurialIndex(b.ID)
	if idx < 0 || b.Container == nil {
		return docker.ErrIdentity
	}
	current := st.Burials[idx]
	if current.Kind != KindContainer || current.Container == nil || !current.Container.Same(*b.Container) ||
		!current.BuriedAt.Equal(b.BuriedAt) || !current.PurgeAt.Equal(b.PurgeAt) || !reflect.DeepEqual(current.PurgeIntent, b.PurgeIntent) {
		return docker.ErrIdentity
	}
	return nil
}

func (s *Service) prepareContainerPurge(r *GraveResult, dryRun bool) (bool, error) {
	b := *r.Burial
	p := ContainerIntent{Operation: "rm", Burial: &b}
	if !dryRun {
		if err := s.bindContainer(&p); err != nil {
			r.Error = err.Error()
			return true, nil
		}
	}
	got, err := s.inspectContainer(p)
	action := ""
	if errors.Is(err, docker.ErrNotFound) {
		action, err = ActionLost, nil
		if b.PurgeIntent != nil {
			action = ActionPurged
		}
	}
	if err != nil {
		r.Error = err.Error()
		return true, nil
	}
	if got.Running {
		action = ActionResurrected
	}
	if action == "" {
		return false, nil
	}
	if dryRun {
		r.Action = action
		return true, nil
	}
	err = s.Store.UpdateGraveyard(func(st *State) error {
		if err := checkContainerBurial(st, b); err != nil {
			return err
		}
		if b.PurgeIntent == nil {
			if err := requireAuditSpace(st); err != nil {
				return err
			}
		}
		finishContainerGrave(st, b, action, s.now(), "container actual state reconciled")
		return nil
	})
	if err == nil {
		r.Action = action
		if action == ActionResurrected {
			r.Path = b.Path
		}
	}
	return true, err
}

func (s *Service) purgeContainer(r *GraveResult, dryRun bool) error {
	b := *r.Burial
	pending, err := s.pendingContainerGrave(b)
	if err != nil {
		return err
	}
	if pending != nil {
		if dryRun {
			r.Error = "container operation pending; run obit reap"
			return nil
		}
		return s.settleContainerGrave(*pending, r)
	}
	if done, err := s.prepareContainerPurge(r, dryRun); done || err != nil {
		return err
	}
	if b.PurgeAt.After(s.now()) {
		r.Error = fmt.Sprintf("#%d: container retention has not elapsed", b.ID)
		return nil
	}
	if dryRun {
		r.Action = ActionPurged
		return nil
	}
	p := ContainerIntent{Operation: "rm", Burial: &b}
	if err := s.bindContainer(&p); err != nil {
		r.Error = err.Error()
		return nil
	}
	if err := s.beginContainerIntent(&p); err != nil {
		return err
	}
	got, err := s.inspectContainer(p)
	if err == nil && !got.Running {
		_, err = s.Docker.Remove(s.Store.Context(), *b.Container, func(ids []string) error {
			return s.updateContainerIntent(&p, func(p *ContainerIntent) { p.RemovalIDs = ids })
		})
	}
	if err != nil {
		r.Error = err.Error()
		if auditErr := s.containerActionError(&p, err); auditErr != nil {
			return auditErr
		}
		if !errors.Is(err, docker.ErrNotFound) {
			return nil
		}
	}
	return s.settleContainerGrave(p, r)
}
