package core

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"errors"
	"time"

	"github.com/woodfishhhh/obituary/internal/docker"
)

// ContainerIntent survives a canceled lifespan so an interrupted stop can be
// reconciled with the user's latest decision. Each intent reserves completion and
// ambiguous-action audit slots so diagnostics survive a full outbox.
type ContainerIntent struct {
	Token        string     `json:"token"`
	Operation    string     `json:"operation"`
	At           time.Time  `json:"at"`
	Item         *Item      `json:"item,omitempty"`
	Burial       *Burial    `json:"burial,omitempty"`
	Phase        string     `json:"phase,omitempty"`
	WasRunning   *bool      `json:"wasRunning,omitempty"`
	StoppedByUs  bool       `json:"stoppedByUs,omitempty"`
	StopTimeout  *int       `json:"stopTimeout,omitempty"`
	Diagnostic   string     `json:"diagnostic,omitempty"`
	RunningSince *time.Time `json:"runningSince,omitempty"`
	RemovalIDs   []string   `json:"removalIds,omitempty"`
}

func (p ContainerIntent) identity() *docker.Identity {
	if p.Item != nil {
		return p.Item.Container
	}
	if p.Burial != nil {
		return p.Burial.Container
	}
	return nil
}

func (p ContainerIntent) id() int64 {
	if p.Item != nil {
		return p.Item.ID
	}
	if p.Burial != nil {
		return p.Burial.ID
	}
	return 0
}

func (st *State) containerIntent(id int64) *ContainerIntent {
	for i := range st.ContainerIntents {
		if st.ContainerIntents[i].id() == id {
			return &st.ContainerIntents[i]
		}
	}
	return nil
}

func (st *State) checkContainerIntent(p ContainerIntent) error {
	current := st.containerIntent(p.id())
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

func (st *State) dropContainerIntent(p ContainerIntent) {
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
		if st.containerIntent(p.id()) != nil {
			return errors.New("container operation pending; run obit reap")
		}
		if st.pendingAuditCount() > maxPendingAudit-2 {
			return errAuditBacklog
		}
		if p.Item != nil {
			if !sameContainerItem(st, *p.Item) || !st.Items[st.Index(p.id())].Due(s.now()) {
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
	got, err := client.Inspect(docker.WithTarget(s.Store.Context(), p.identity().Target), p.identity().ID)
	if err == nil && !got.Same(*p.identity()) {
		err = docker.ErrIdentity
	}
	return got, err
}

func validateContainerIntents(st State) error {
	seen := map[int64]bool{}
	for _, p := range st.ContainerIntents {
		id := p.identity()
		if p.Token == "" || p.At.IsZero() || p.id() <= 0 || seen[p.id()] || id == nil || !id.Valid() {
			return docker.ErrIdentity
		}
		seen[p.id()] = true
		switch p.Phase {
		case "", "prepared", "sent", "confirmed":
		default:
			return docker.ErrIdentity
		}
		if p.StoppedByUs && (p.WasRunning == nil || !*p.WasRunning) {
			return docker.ErrIdentity
		}
		switch p.Operation {
		case "stop", "resume":
			if p.Item == nil || p.Burial != nil || containerIdentity(p.Item.Kind, id) != nil {
				return docker.ErrIdentity
			}
		case "start", "rm":
			if p.Burial == nil || p.Item != nil || containerIdentity(p.Burial.Kind, id) != nil {
				return docker.ErrIdentity
			}
		default:
			return docker.ErrIdentity
		}
	}
	return nil
}
