package core_test

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/watch"
)

func TestReviewedProposalDecisionsPreserveChangedSnapshot(t *testing.T) {
	for _, decision := range []string{"approve", "reject"} {
		for _, change := range []string{"add", "refresh", "prune-readd", "lifespan", "kind", "path", "watch"} {
			t.Run(decision+"/"+change, func(t *testing.T) {
				f := newFixture(t)
				p, _, _, err := f.svc.Propose(core.ProposalOnboard, f.dir, "s", []core.Candidate{{Path: f.file("reviewed.log"), Lifespan: "2d"}})
				if err != nil {
					t.Fatal(err)
				}
				fingerprint := core.ProposalFingerprint(p)
				changeProposalReview(t, f.svc, p, change)
				if decision == "approve" {
					_, release, err := f.svc.ClaimProposalReviewed(fmt.Sprint(p.ID), fingerprint)
					if release != nil {
						release()
					}
					if !errors.Is(err, core.ErrProposalChanged) {
						t.Fatalf("changed review granted consent: %v", err)
					}
				} else {
					out, err := f.svc.RejectReviewed([]string{fmt.Sprint(p.ID)}, []string{fingerprint})
					if err != nil || len(out) != 1 || out[0].Error != core.ErrProposalChanged.Error() {
						t.Fatalf("changed review rejected candidates: %+v %v", out, err)
					}
				}
				st, err := f.svc.Store.Read()
				if err != nil || len(st.Proposals) != 1 || len(st.Projects) != 0 || len(st.LeftoverRejections) != 0 {
					t.Fatalf("changed review mutated ledger: %+v %v", st, err)
				}
			})
		}
	}
}

func changeProposalReview(t *testing.T, svc *core.Service, p core.Proposal, change string) {
	t.Helper()
	if change == "prune-readd" {
		keep := core.Candidate{Path: p.Candidates[0].Path + ".keep", Lifespan: "1d"}
		if _, _, _, err := svc.Propose(p.Kind, p.Project, p.Session, []core.Candidate{keep}); err != nil {
			t.Fatal(err)
		}
		if err := svc.PruneProposal(p, p.Candidates); err != nil {
			t.Fatal(err)
		}
		if _, _, _, err := svc.Propose(p.Kind, p.Project, p.Session, p.Candidates); err != nil {
			t.Fatal(err)
		}
		return
	}
	err := svc.Store.Update(func(st *core.State) error {
		current := &st.Proposals[0]
		switch change {
		case "add":
			current.Candidates = append(current.Candidates, core.Candidate{Path: "unseen.log", Rev: 2, Lifespan: "1h"})
		case "refresh", "prune-readd":
			current.CandidateSeq++
			current.Candidates[0].Rev = current.CandidateSeq
		case "lifespan":
			current.Candidates[0].Lifespan = "1h"
		case "kind":
			current.Kind = core.ProposalLeftovers
		case "path":
			current.Candidates[0].Path += ".other"
		case "watch":
			current.WatchID = 99
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestWatchReviewRejectsDivergentPolicy(t *testing.T) {
	for _, change := range []string{"lifespan-age", "path", "identity", "project"} {
		for _, when := range []string{"before-review", "after-review", "after-claim"} {
			t.Run(change+"/"+when, func(t *testing.T) {
				f := newFixture(t)
				path := f.file("downloads/payload.log")
				if _, err := f.svc.SetWatch(filepath.Dir(path), "30d", core.WatchModified); err != nil {
					t.Fatal(err)
				}
				if when == "before-review" {
					divergeWatchReview(t, f, change)
				}
				st, err := f.svc.Store.Read()
				if err != nil {
					t.Fatal(err)
				}
				p := st.Proposals[0]
				fingerprint := core.ProposalFingerprint(p)
				if when == "after-claim" {
					p, release, err := f.svc.ClaimProposalReviewed(fmt.Sprint(p.ID), fingerprint)
					if err != nil {
						t.Fatal(err)
					}
					defer release()
					divergeWatchReview(t, f, change)
					if err := f.svc.ApproveWatch(p); !errors.Is(err, core.ErrProposalChanged) {
						t.Fatalf("divergent watch policy approved: %v", err)
					}
				} else {
					if when == "after-review" {
						divergeWatchReview(t, f, change)
					}
					out, err := (&auto.Engine{Svc: f.svc}).ApproveReviewed([]string{fmt.Sprint(p.ID)}, []string{fingerprint})
					if err != nil || len(out) != 1 || out[0].Error != core.ErrProposalChanged.Error() {
						t.Fatalf("divergent watch policy approved: %+v %v", out, err)
					}
				}
				assertUnapprovedWatchSafe(t, f, path)
			})
		}
	}
}

func divergeWatchReview(t *testing.T, f *fixture, change string) {
	t.Helper()
	other := filepath.Dir(f.file("other/payload.log"))
	err := f.svc.Store.Update(func(st *core.State) error {
		p := &st.Watches[0]
		switch change {
		case "lifespan-age":
			p.Lifespan, p.Age = "1h", core.WatchFirstSeen
		case "path":
			p.Path = other
			var err error
			p.Identity, err = f.svc.FS.Identify(other)
			return err
		case "identity":
			p.Identity.Serial++
		case "project":
			p.Project = other
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertUnapprovedWatchSafe(t *testing.T, f *fixture, path string) {
	t.Helper()
	st, err := f.svc.Store.Read()
	if err != nil || len(st.Proposals) != 1 || st.Watches[0].Consent != "pending" {
		t.Fatalf("divergent watch changed consent: %+v %v", st, err)
	}
	if err := f.svc.RegisterWatchChild(st.Watches[0], path); err == nil {
		t.Fatal("divergent watch registered payload")
	}
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	f.clock = f.clock.Add(2 * time.Hour)
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	st, err = f.svc.Store.Read()
	if err != nil || len(st.Items) != 0 || len(st.Burials) != 0 || !exists(path) {
		t.Fatalf("divergent watch moved payload: %+v %v", st, err)
	}
}

func TestWatchFingerprintBindsStructuredPolicy(t *testing.T) {
	for _, change := range []func(*core.WatchReview){
		func(p *core.WatchReview) { p.ID++ },
		func(p *core.WatchReview) { p.Rev++ },
		func(p *core.WatchReview) { p.Path += "other" },
		func(p *core.WatchReview) { p.Project += "other" },
		func(p *core.WatchReview) { p.Identity.Serial++ },
		func(p *core.WatchReview) { p.Lifespan = "1h" },
		func(p *core.WatchReview) { p.Age = core.WatchFirstSeen },
	} {
		p := core.Proposal{WatchReview: &core.WatchReview{ID: 1, Rev: 1, Path: "folder", Lifespan: "30d", Age: core.WatchModified}}
		before := core.ProposalFingerprint(p)
		change(p.WatchReview)
		if core.ProposalFingerprint(p) == before {
			t.Fatal("watch fingerprint omitted policy field")
		}
	}
}

func TestWatchReviewRejectsChangedProposalCopy(t *testing.T) {
	for _, claimed := range []bool{false, true} {
		t.Run(fmt.Sprint(claimed), func(t *testing.T) {
			f := newFixture(t)
			path := filepath.Dir(f.file("downloads/payload.log"))
			if _, err := f.svc.SetWatch(path, "30d", core.WatchModified); err != nil {
				t.Fatal(err)
			}
			st, err := f.svc.Store.Read()
			if err != nil {
				t.Fatal(err)
			}
			p := st.Proposals[0]
			if err := f.svc.Store.Update(func(st *core.State) error {
				st.Proposals[0].Candidates[0].Lifespan = "1h"
				if !claimed {
					p = st.Proposals[0]
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if err := f.svc.ApproveWatch(p); !errors.Is(err, core.ErrProposalChanged) {
				t.Fatalf("inconsistent proposal copy approved: %v", err)
			}
		})
	}
}

func TestWatchLegacyReviewRequiresRenewal(t *testing.T) {
	f := newFixture(t)
	path := filepath.Dir(f.file("downloads/payload.log"))
	if _, err := f.svc.SetWatch(path, "30d", core.WatchModified); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Store.Update(func(st *core.State) error {
		st.Proposals[0].WatchReview = nil
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.ApproveWatch(st.Proposals[0]); !errors.Is(err, core.ErrProposalChanged) {
		t.Fatalf("legacy policy bypassed new review: %v", err)
	}
	if _, err := f.svc.SetWatch(path, "30d", core.WatchModified); err != nil {
		t.Fatal(err)
	}
	st, err = f.svc.Store.Read()
	if err != nil || len(st.Proposals) != 1 || st.Proposals[0].WatchReview == nil {
		t.Fatalf("legacy review renewal: %+v %v", st, err)
	}
	if err := f.svc.ApproveWatch(st.Proposals[0]); err != nil {
		t.Fatal(err)
	}
}

func TestProposalFingerprintStableOrderAndNotification(t *testing.T) {
	p := core.Proposal{ID: 1, Kind: core.ProposalLeftovers, Candidates: []core.Candidate{{Path: "a", Rev: 1, Lifespan: "1d"}, {Path: "b", Rev: 2, Lifespan: "2d"}}}
	want := core.ProposalFingerprint(p)
	p.Candidates = slices.Clone(p.Candidates)
	slices.Reverse(p.Candidates)
	p.Notified = []string{"other-session"}
	if core.ProposalFingerprint(p) != want {
		t.Fatal("notification/order invalidated review")
	}
}

func TestReviewedProposalRequiresFingerprint(t *testing.T) {
	f := newFixture(t)
	p, _, _, err := f.svc.Propose(core.ProposalOnboard, f.dir, "s", []core.Candidate{{Path: f.file("candidate"), Lifespan: "1d"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.svc.ClaimProposalReviewed(fmt.Sprint(p.ID), ""); !errors.Is(err, core.ErrProposalChanged) {
		t.Fatalf("empty review accepted: %v", err)
	}
	for _, fingerprints := range [][]string{nil, {""}, {"a", "b"}} {
		if _, err := f.svc.RejectReviewed([]string{fmt.Sprint(p.ID)}, fingerprints); !errors.Is(err, core.ErrProposalChanged) {
			t.Fatalf("missing review accepted: %v", err)
		}
	}
	_, release, err := f.svc.ClaimProposalReviewed(fmt.Sprint(p.ID), core.ProposalFingerprint(p))
	if err != nil {
		t.Fatal(err)
	}
	release()
}
