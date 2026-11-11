package gui

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/core"
)

func TestSummaryOmitsAbsentProposalCandidates(t *testing.T) {
	a, root := fixture(t)
	if _, _, _, err := a.deps.Service.Propose("leftovers", root, "s", []core.Candidate{{Path: filepath.Join(root, "absent.log"), Lifespan: "1d"}}); err != nil {
		t.Fatal(err)
	}
	list, err := a.Proposals()
	if err != nil || len(list.Proposals) != 0 {
		t.Fatalf("review displayed an absent candidate: %+v %v", list, err)
	}
	summary, err := a.Summary()
	if err != nil || summary.DecisionCount != len(list.Proposals) {
		t.Fatalf("summary disagrees with reviewed proposals: %+v %v", summary, err)
	}
}

func TestSummaryCountsAndErrors(t *testing.T) {
	a, root := fixture(t)
	add(t, a, filepath.Join(root, "tracked"), time.Now().Add(time.Hour))
	b := bury(t, a, filepath.Join(root, "buried"))
	repo := repository(t, root)
	propose(t, a, repo, "proposal.log")
	write(t, filepath.Join(repo, ".obitrules"), "*.log 2d\n")
	if err := app.SetPaused(a.deps.Service.Store.Home, true); err != nil {
		t.Fatal(err)
	}
	summary, err := a.Summary()
	if err != nil || summary.Error != "" || summary.TrackedCount != 1 || summary.GraveCount != 1 || summary.GraveBytes != b.Size || summary.DecisionCount != 2 || !summary.Paused {
		t.Fatalf("summary: %+v %v", summary, err)
	}
	breakStore(t, a)
	if _, err := a.Summary(); err == nil {
		t.Fatal("summary hid corrupt ledger")
	}
}
