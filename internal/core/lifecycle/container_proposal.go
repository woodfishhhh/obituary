package lifecycle

import (
	"fmt"

	"github.com/woodfishhhh/obituary/internal/docker"
)

func candidateKey(c Candidate) string {
	if c.Kind == KindContainer && c.Container != nil {
		return c.Container.Key()
	}
	return ProjectKey(c.Path)
}

func sameCandidate(a, b Candidate) bool {
	if a.Kind == KindContainer || b.Kind == KindContainer {
		return a.Kind == b.Kind && candidateKey(a) == candidateKey(b)
	}
	return samePath(a.Path, b.Path)
}

// Called under the ledger lock so overlapping sessions cannot both propose an ID.
func pendingContainer(st *State, c Candidate) bool {
	if c.Kind != KindContainer || c.Container == nil {
		return false
	}
	for _, p := range st.Proposals {
		for _, have := range p.Candidates {
			if have.Container != nil && have.Container.ID == c.Container.ID {
				return true
			}
		}
	}
	return false
}

// ApproveContainer re-inspects the immutable identity saved in the proposal.
// A non-empty class marks a registration Obituary made without asking.
func (s *Service) ApproveContainer(p Proposal, c Candidate, class string) ([]Result, bool, error) {
	result := Result{Ref: c.Path}
	client, err := s.containerClient()
	if err == nil {
		err = containerIdentity(c.Kind, c.Container)
	}
	if err != nil {
		result.Error = err.Error()
		return []Result{result}, true, err
	}
	got, err := client.Inspect(docker.WithTarget(s.Store.Context(), c.Container.Target), c.Container.ID)
	if err == nil && got.AutoRemove {
		err = docker.ErrAutoRemove
	}
	if err == nil && !got.Same(*c.Container) {
		err = docker.ErrIdentity
	}
	if err != nil {
		result.Error = err.Error()
		return []Result{result}, true, err
	}
	lifespan, err := ParseDuration(c.Lifespan)
	if err != nil {
		result.Error = err.Error()
		return []Result{result}, true, err
	}
	note := fmt.Sprintf("approved #%d", p.ID)
	if class != "" {
		note = fmt.Sprintf("auto-approved #%d (%s)", p.ID, class)
	}
	item, err := s.registerContainer(got.Identity, AddOptions{ExpiresAt: s.now().Add(lifespan), Source: "proposal", Project: p.Project, Session: NormalizeSession(p.Session), Note: note, OnlyNew: true, AutoClass: class})
	if err != nil {
		result.Error = err.Error()
		return []Result{result}, true, err
	}
	result.Item = &item
	return []Result{result}, false, nil
}
