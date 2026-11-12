package lifecycle

import (
	"errors"
	"fmt"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/docker"
)

const KindContainer = ledger.KindContainer

func (s *Service) containerClient() (docker.Client, error) {
	if s.Docker == nil {
		return nil, errors.New("docker is unavailable")
	}
	return s.Docker, nil
}

// AddContainer resolves names only at registration. Every subsequent action uses the pinned ID.
func (s *Service) AddContainer(ref string, opt AddOptions) (Item, error) {
	client, err := s.containerClient()
	if err != nil {
		return Item{}, err
	}
	got, err := client.Inspect(s.Store.Context(), ref)
	if err != nil {
		return Item{}, err
	}
	if got.AutoRemove {
		return Item{}, docker.ErrAutoRemove
	}
	return s.registerContainer(got.Identity, opt)
}

func (s *Service) registerContainer(id docker.Identity, opt AddOptions) (item Item, err error) {
	if !id.Valid() {
		return item, docker.ErrIdentity
	}
	for field, value := range map[string]string{"note": opt.Note, "session": opt.Session, "source": opt.Source} {
		if err := ValidateMetadata(field, value); err != nil {
			return item, err
		}
	}
	if !opt.ExpiresAt.After(s.now()) || opt.ExpiresAt.After(s.now().Add(MaxLifespan)) {
		return item, errors.New("container lifespan must be positive and within the maximum lifespan")
	}
	s, notify := s.trackChanges()
	defer notify()
	s, flush := s.auditBatch(false)
	defer func() { err = errors.Join(err, flush()) }()
	err = s.Store.UpdateGraveyard(func(st *State) error {
		if err := requireAuditSpace(st); err != nil {
			return err
		}
		for _, b := range st.Burials {
			if b.Container != nil && b.Container.Same(id) {
				return fmt.Errorf("container is buried as #%d; obit resurrect %d", b.ID, b.ID)
			}
		}
		idx := containerIndex(st.Items, id)
		if idx >= 0 && !st.Items[idx].Container.Same(id) {
			return docker.ErrIdentity
		}
		if idx >= 0 && opt.OnlyNew {
			item = st.Items[idx]
			return nil
		}
		item = Item{Kind: KindContainer, Container: &id, ID: st.NextID, Rev: 1, Path: id.Display(), CreatedAt: s.now(), ExpiresAt: opt.ExpiresAt, Note: opt.Note, Source: opt.Source, Project: opt.Project, Session: opt.Session}
		if item.Source == "" {
			item.Source = "cli"
		}
		if idx >= 0 {
			item.ID, item.Rev, item.CreatedAt = st.Items[idx].ID, st.Items[idx].Rev+1, st.Items[idx].CreatedAt
			st.Items[idx] = item
		} else {
			st.NextID++
			st.Items = append(st.Items, item)
		}
		queueAudit(st, Event{Time: s.now(), Action: "added", ID: item.ID, Path: item.Path, Detail: "container expires " + item.ExpiresAt.Format(time.RFC3339)})
		return nil
	})
	return item, err
}

func (s *Service) checkItemMutation(st *State, idx int, path string, roots []resolvedGraveyardRoot) error {
	if st.Items[idx].Kind == KindContainer {
		return containerIdentity(st.Items[idx].Kind, st.Items[idx].Container)
	}
	if err := s.checkBurying(st.Items[idx]); err != nil {
		return err
	}
	return s.checkBuried(st, path, st.Items[idx].Identity, roots)
}

func containerIndex(items []Item, id docker.Identity) int {
	for i, it := range items {
		if it.Container != nil && it.Container.ID == id.ID && (it.Container.Target == nil || id.Target == nil || it.Container.Target.Endpoint == id.Target.Endpoint && it.Container.Target.DaemonID == id.Target.DaemonID) {
			return i
		}
	}
	return -1
}

// Containers are removed from the file sweep snapshot before any filesystem gates.
func (s *Service) sweepContainers(state *State, dryRun bool) ([]Outcome, error) {
	var rows []Outcome
	blocked := map[int64]bool{}
	if !dryRun {
		var err error
		rows, blocked, err = s.recoverContainerStops(state)
		if err != nil {
			return rows, err
		}
	}
	var files []Item
	for _, it := range state.Items {
		if it.Kind != KindContainer {
			files = append(files, it)
			continue
		}
		if blocked[it.ID] || !it.Due(s.now()) {
			continue
		}
		out := s.stopContainer(it, dryRun)
		rows = append(rows, out)
		if out.err != nil {
			return rows, out.err
		}
		if !dryRun && out.Action == ActionRetry {
			if err := s.commitContainerRetry(out); err != nil {
				return rows, err
			}
		}
	}
	state.Items = files
	return rows, nil
}

func containerIdentity(kind string, id *docker.Identity) error {
	return ledger.ContainerIdentity(kind, id)
}
