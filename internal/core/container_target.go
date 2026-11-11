package core

import (
	"errors"

	"github.com/woodfishhhh/obituary/internal/docker"
)

// Legacy records bind only after the current daemon returns the saved identity.
// Absence on an unbound daemon cannot retire a legacy record.
func (s *Service) bindContainer(p *ContainerIntent) error {
	if containerIntentIdentity(*p).Target != nil {
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
	next := *containerIntentIdentity(*p)
	next.Target = got.Target
	err = s.Store.UpdateGraveyard(func(st *State) error {
		if old.Token != "" {
			if err := checkContainerIntent(st, old); err != nil {
				return err
			}
		}
		if i := st.Index(containerIntentID(*p)); i >= 0 {
			if st.Items[i].Container == nil || !st.Items[i].Container.Same(next) {
				return docker.ErrIdentity
			}
			st.Items[i].Container = &next
		}
		if i := st.BurialIndex(containerIntentID(*p)); i >= 0 {
			if st.Burials[i].Container == nil || !st.Burials[i].Container.Same(next) {
				return docker.ErrIdentity
			}
			st.Burials[i].Container = &next
		}
		if current := containerIntent(st, containerIntentID(*p)); current != nil {
			*containerIntentIdentity(*current) = next
		}
		return nil
	})
	if err == nil {
		*containerIntentIdentity(*p) = next
	}
	return err
}

func (s *Service) updateContainerIntent(p *ContainerIntent, change func(*ContainerIntent)) error {
	next := *p
	change(&next)
	err := s.Store.UpdateGraveyard(func(st *State) error {
		if err := checkContainerIntent(st, *p); err != nil {
			return err
		}
		*containerIntent(st, containerIntentID(*p)) = next
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
	auditErr := s.Store.UpdateGraveyard(func(st *State) error {
		if err := checkContainerIntent(st, *p); err != nil {
			return err
		}
		*containerIntent(st, containerIntentID(*p)) = next
		queueAudit(st, Event{Time: s.now(), Action: "container-ambiguous", ID: containerIntentID(*p), Path: containerIntentIdentity(*p).Display(), Detail: next.Diagnostic})
		return nil
	})
	if auditErr == nil {
		*p = next
	}
	return auditErr
}
