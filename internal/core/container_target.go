package core

import (
	"errors"

	"github.com/woodfishhhh/obituary/internal/docker"
)

// Legacy records bind only after the current daemon returns the saved identity.
// Absence on an unbound daemon cannot retire a legacy record.
func (s *Service) bindContainer(p *ContainerIntent) error {
	if p.identity().Target != nil {
		return nil
	}
	got, err := s.inspectContainer(*p)
	if errors.Is(err, docker.ErrNotFound) {
		return errors.New("legacy container has no pinned Docker target; restore its original daemon to migrate")
	}
	if err != nil {
		return err
	}
	if got.Target == nil {
		return nil
	} // In-process clients already own their routing.
	old := *p
	next := *p.identity()
	next.Target = got.Target
	err = s.Store.Update(func(st *State) error {
		if old.Token != "" {
			if err := st.checkContainerIntent(old); err != nil {
				return err
			}
		}
		if i := st.Index(p.id()); i >= 0 {
			if st.Items[i].Container == nil || !st.Items[i].Container.Same(next) {
				return docker.ErrIdentity
			}
			st.Items[i].Container = &next
		}
		if i := st.BurialIndex(p.id()); i >= 0 {
			if st.Burials[i].Container == nil || !st.Burials[i].Container.Same(next) {
				return docker.ErrIdentity
			}
			st.Burials[i].Container = &next
		}
		if current := st.containerIntent(p.id()); current != nil {
			*current.identity() = next
		}
		return nil
	})
	if err == nil {
		*p.identity() = next
	}
	return err
}

func (s *Service) updateContainerIntent(p *ContainerIntent, change func(*ContainerIntent)) error {
	next := *p
	change(&next)
	err := s.Store.Update(func(st *State) error {
		if err := st.checkContainerIntent(*p); err != nil {
			return err
		}
		*st.containerIntent(p.id()) = next
		return nil
	})
	if err == nil {
		*p = next
	}
	return err
}

func (s *Service) containerActionError(p *ContainerIntent, err error) error {
	if !errors.Is(err, docker.ErrAmbiguousAction) {
		return nil
	}
	next := *p
	next.Diagnostic = err.Error()
	auditErr := s.Store.Update(func(st *State) error {
		if err := st.checkContainerIntent(*p); err != nil {
			return err
		}
		*st.containerIntent(p.id()) = next
		st.queueAudit(Event{Time: s.now(), Action: "container-ambiguous", ID: p.id(), Path: p.identity().Display(), Detail: next.Diagnostic})
		return nil
	})
	if auditErr == nil {
		*p = next
	}
	return auditErr
}
