package core_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestLeftoverDecisionRemovesEmptyTwinsOnlyInProject(t *testing.T) {
	for _, decision := range []string{"resolve", "reject"} {
		t.Run(decision, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("research.md")
			proposal := func(id int64, kind, project string) core.Proposal {
				return core.Proposal{ID: id, Kind: kind, Project: project, Session: fmt.Sprint(id),
					Candidates: []core.Candidate{{Path: path, Lifespan: "3d", Rev: uint64(id)}}}
			}
			if err := f.svc.Store.Update(func(st *core.State) error {
				st.NextID = 6
				st.Proposals = []core.Proposal{
					proposal(1, core.ProposalLeftovers, f.dir), proposal(2, core.ProposalLeftovers, f.dir),
					proposal(3, core.ProposalLeftovers, f.dir), proposal(4, core.ProposalLarge, f.dir),
					proposal(5, core.ProposalLeftovers, f.dir+"-other"),
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if decision == "reject" {
				if rows, err := f.svc.Reject([]string{"2"}); err != nil || len(rows) != 1 || rows[0].Error != "" {
					t.Fatalf("reject: %+v %v", rows, err)
				}
			} else {
				p, release, err := f.svc.ClaimProposal("2")
				if err != nil {
					t.Fatal(err)
				}
				defer release()
				if err := f.svc.ResolveProposal(p, nil); err != nil {
					t.Fatal(err)
				}
			}
			st, err := f.svc.Store.Read()
			if err != nil || len(st.Proposals) != 2 || st.Proposals[0].ID != 4 || st.Proposals[1].ID != 5 {
				t.Fatalf("empty twins or unrelated proposals mishandled: %+v %v", st.Proposals, err)
			}
		})
	}
}

func TestLeftoverProposalSkipsStaleFinishAfterApproval(t *testing.T) {
	f := newFixture(t)
	c := core.Candidate{Path: f.file("research.md"), Lifespan: "3d"}
	p, _, _, err := f.svc.Propose(core.ProposalLeftovers, f.dir, "first", []core.Candidate{c})
	if err != nil {
		t.Fatal(err)
	}
	claimed, release, err := f.svc.ClaimProposal(fmt.Sprint(p.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	rows, err := f.svc.Add([]string{c.Path}, core.AddOptions{ExpiresAt: f.clock.Add(24 * time.Hour)})
	if err != nil || core.Failed(rows) {
		t.Fatalf("add approved candidate: %+v %v", rows, err)
	}
	if err := f.svc.ResolveProposal(claimed, nil); err != nil {
		t.Fatal(err)
	}
	p, created, added, err := f.svc.Propose(core.ProposalLeftovers, f.dir, "second", []core.Candidate{c})
	if err != nil || p.ID != 0 || created || added != 0 {
		t.Fatalf("stale finish reproposed registered leftover: %+v %v %d %v", p, created, added, err)
	}
}
