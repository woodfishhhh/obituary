package lifecycle

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/docker"
)

type ContainerIntent = ledger.ContainerIntent

func containerIntentIdentity(p ContainerIntent) *docker.Identity {
	return ledger.ContainerIntentIdentity(p)
}

func containerIntentID(p ContainerIntent) int64 { return ledger.ContainerIntentID(p) }

func containerIntent(st *State, id int64) *ContainerIntent { return ledger.FindContainerIntent(st, id) }

func checkContainerIntent(st *State, p ContainerIntent) error {
	current := containerIntent(st, containerIntentID(p))
	if current == nil || !sameContainerIntent(*current, p) {
		return ErrUnsafe
	}
	return nil
}

func sameContainerIntent(a, b ContainerIntent) bool {
	// JSON compares the persisted timestamps without process-local monotonic clocks.
	left, err := json.Marshal(a)
	right, otherErr := json.Marshal(b)
	return err == nil && otherErr == nil && bytes.Equal(left, right)
}

func dropContainerIntent(st *State, p ContainerIntent) {
	for i, current := range st.ContainerIntents {
		if current.Token == p.Token {
			st.ContainerIntents = append(st.ContainerIntents[:i], st.ContainerIntents[i+1:]...)
			return
		}
	}
}

func (s *Service) beginContainerIntent(p *ContainerIntent) error {
	p.Token, p.At = rand.Text(), s.now()
	return s.Store.UpdateGraveyard(func(st *State) error {
		if containerIntent(st, containerIntentID(*p)) != nil {
			return errors.New("container operation pending; run obit reap")
		}
		if pendingAuditCount(st) > maxPendingAudit-2 {
			return errAuditBacklog
		}
		if p.Item != nil {
			if !sameContainerItem(st, *p.Item) || !st.Items[st.Index(containerIntentID(*p))].Due(s.now()) {
				return ErrUnsafe
			}
		} else if err := checkContainerBurial(st, *p.Burial); err != nil {
			return err
		}
		if p.Operation == "rm" && p.Burial.PurgeAt.After(s.now()) {
			return errors.New("container retention has not elapsed")
		}
		st.ContainerIntents = append(st.ContainerIntents, *p)
		return nil
	})
}

func sameContainerItem(st *State, it Item) bool {
	idx := st.Index(it.ID)
	return idx >= 0 && st.Items[idx].Rev == it.Rev && st.Items[idx].Kind == KindContainer && st.Items[idx].Container != nil && st.Items[idx].Container.Same(*it.Container)
}

func (s *Service) inspectContainer(p ContainerIntent) (docker.Container, error) {
	client, err := s.containerClient()
	if err != nil {
		return docker.Container{}, err
	}
	got, err := client.Inspect(docker.WithTarget(s.Store.Context(), containerIntentIdentity(p).Target), containerIntentIdentity(p).ID)
	if err == nil && !got.Same(*containerIntentIdentity(p)) {
		err = docker.ErrIdentity
	}
	return got, err
}
