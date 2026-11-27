package app

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/docker"
)

type proposalDocker struct {
	docker.Client
	err   error
	calls int
}

func (d *proposalDocker) ListIDs(context.Context) ([]string, error) {
	d.calls++
	return nil, d.err
}

func TestProposalViewsReportLiveDockerAvailability(t *testing.T) {
	d := &proposalDocker{err: docker.ErrUnavailable}
	svc := &lifecycle.Service{Docker: d}
	id := &docker.Identity{ID: strings.Repeat("a", 64)}
	proposals := []ledger.Proposal{{ID: 1, Candidates: []ledger.Candidate{{Kind: ledger.KindContainer, Container: id}}}, {ID: 2, Candidates: []ledger.Candidate{{Kind: ledger.KindContainer, Container: id}}}}
	for _, tc := range []struct {
		err     error
		blocked string
	}{{docker.ErrUnavailable, "docker-unavailable"}, {nil, ""}, {docker.ErrTarget, ""}, {errors.New("invalid response"), ""}} {
		d.err, d.calls = tc.err, 0
		views := ProposalViews(svc, proposals)
		if len(views) != 2 || views[0].Blocked != tc.blocked || views[1].Blocked != tc.blocked || d.calls != 1 {
			t.Fatal(views, d.calls)
		}
		data, err := json.Marshal(views)
		if err != nil || strings.Contains(string(data), `"blocked":"docker-unavailable"`) != (tc.blocked != "") {
			t.Fatal(string(data), err)
		}
	}
	svc.Docker = nil
	if views := ProposalViews(svc, proposals); views[0].Blocked != "docker-unavailable" {
		t.Fatal(views)
	}
}
