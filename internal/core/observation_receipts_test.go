package core_test

import (
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

func TestObservedRegistrationReceiptCommitsWithItem(t *testing.T) {
	f := newFixture(t)
	path := f.file("debug.log")
	rows, err := f.svc.CaptureObservations([]string{path}, "s")
	if err != nil {
		t.Fatal(err)
	}
	opts := core.AddOptions{ExpiresAt: f.clock.Add(time.Hour), Observed: &rows[0]}
	if _, err := f.svc.Add([]string{path}, opts); err != nil {
		t.Fatal(err)
	}
	opts.ExpiresAt = f.clock.Add(2 * time.Hour)
	if _, err := f.svc.Add([]string{path}, opts); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.Read()
	if err != nil || len(st.Items) != 1 || st.Items[0].Rev != 1 || !st.Items[0].ExpiresAt.Equal(f.clock.Add(time.Hour)) || !st.ObservationCompleted(&rows[0]) {
		t.Fatalf("request replay changed committed lifespan: %+v %v", st, err)
	}
}

func TestObservedProposalReceiptDoesNotRefreshCandidate(t *testing.T) {
	f := newFixture(t)
	path := f.file("debug.log")
	rows, err := f.svc.CaptureObservations([]string{path}, "s")
	if err != nil {
		t.Fatal(err)
	}
	candidates := []core.Candidate{{Path: path, Lifespan: "1d"}}
	p, _, _, err := f.svc.ProposeObserved(core.ProposalOnboard, f.dir, "s", candidates, &rows[0])
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := f.svc.ProposeObserved(core.ProposalOnboard, f.dir, "s", candidates, &rows[0]); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.Read()
	if err != nil || len(st.Proposals) != 1 || st.Proposals[0].CandidateSeq != p.CandidateSeq || !st.ObservationCompleted(&rows[0]) {
		t.Fatalf("request replay refreshed proposal: %+v %v", st, err)
	}
}
