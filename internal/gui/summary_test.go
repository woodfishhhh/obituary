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

func TestSummaryAndUpcomingTrackEffectiveServiceMode(t *testing.T) {
	a, root := fixture(t)
	s, err := a.Summary()
	if err != nil || s.Mode != core.ModeGraveyard {
		t.Fatalf("default summary mode = %q, err = %v", s.Mode, err)
	}
	up, err := a.Upcoming("")
	if err != nil || up.Mode != core.ModeGraveyard {
		t.Fatalf("default upcoming mode = %q, err = %v", up.Mode, err)
	}

	// External disk edits to config.toml must not diverge Summary/Upcoming from
	// the live Service.Mode that Bury actually executes.
	cfgPath := filepath.Join(a.deps.ConfigDir, "config.toml")
	a.deps.Service.Mode = core.ModeRecycle
	write(t, cfgPath, "[graveyard]\nmode = \"graveyard\"\n")
	if s, err := a.Summary(); err != nil || s.Mode != core.ModeRecycle {
		t.Fatalf("summary drifted to disk config: mode=%q err=%v", s.Mode, err)
	}
	if up, err := a.Upcoming(""); err != nil || up.Mode != core.ModeRecycle {
		t.Fatalf("upcoming drifted to disk config: mode=%q err=%v", up.Mode, err)
	}

	// Reverse direction: live graveyard mode stays graveyard even if disk says recycle.
	a.deps.Service.Mode = core.ModeGraveyard
	write(t, cfgPath, "[graveyard]\nmode = \"recycle\"\n")
	it := add(t, a, filepath.Join(root, "plain.txt"), time.Now().Add(time.Hour))
	if s, err := a.Summary(); err != nil || s.Mode != core.ModeGraveyard {
		t.Fatalf("summary drifted to recycle: mode=%q err=%v", s.Mode, err)
	}
	if up, err := a.Upcoming(""); err != nil || up.Mode != core.ModeGraveyard {
		t.Fatalf("upcoming drifted to recycle: mode=%q err=%v", up.Mode, err)
	}
	reply, err := a.Bury([]string{"#" + ref(it.ID)})
	if err != nil || len(reply.BuriedIDs) != 1 || reply.Results[0].Action != core.ActionBuried {
		t.Fatalf("bury should follow live graveyard mode: %+v %v", reply, err)
	}

	// Even when Summary fails due to an unrelated corrupt rule trust file,
	// Upcoming still succeeds and delivers the live recycle mode.
	repo := repository(t, root)
	propose(t, a, repo, "note.log")
	write(t, filepath.Join(repo, ".obitrules"), "*.log 2d\n")
	write(t, a.deps.Engine.Trust.Path, "{not-valid-json")
	a.deps.Service.Mode = core.ModeRecycle
	if _, err := a.Summary(); err == nil {
		t.Fatal("expected Summary to fail on corrupt trust file")
	}
	up, err = a.Upcoming("")
	if err != nil || up.Mode != core.ModeRecycle {
		t.Fatalf("Upcoming must still report live recycle mode when Summary fails: %+v %v", up, err)
	}
}
