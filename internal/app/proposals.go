package app

import (
	"errors"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/docker"
)

// ProposalView adds live availability without persisting it in the ledger.
type ProposalView struct {
	ledger.Proposal
	Blocked string `json:"blocked,omitempty"`
}

func ProposalViews(svc *lifecycle.Service, proposals []ledger.Proposal) []ProposalView {
	views := make([]ProposalView, 0, len(proposals))
	checked := map[docker.Target]string{}
	for _, p := range lifecycle.ExistingProposals(proposals) {
		view := ProposalView{Proposal: p}
		for _, c := range p.Candidates {
			if c.Kind != ledger.KindContainer || c.Container == nil {
				continue
			}
			target := docker.Target{}
			if c.Container.Target != nil {
				target = *c.Container.Target
			}
			blocked, ok := checked[target]
			if !ok {
				err := docker.ErrUnavailable
				if svc.Docker != nil {
					_, err = svc.Docker.ListIDs(docker.WithTarget(svc.Store.Context(), c.Container.Target))
				}
				if errors.Is(err, docker.ErrUnavailable) {
					blocked = "docker-unavailable"
				}
				checked[target] = blocked
			}
			if blocked != "" {
				view.Blocked = blocked
			}
		}
		views = append(views, view)
	}
	return views
}
