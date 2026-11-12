package lifecycle_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/watch"
)

func expiredWatchProposal(t *testing.T, f *fixture) core.Proposal {
	t.Helper()
	st, err := f.svc.Store.Read()
	if err != nil || len(st.Proposals) != 1 || st.Proposals[0].Kind != "watch-burial" {
		t.Fatalf("expired directory did not produce one burial proposal: %+v %v", st.Proposals, err)
	}
	return st.Proposals[0]
}

func approveWatchDirectoryAt(t *testing.T, f *fixture, after time.Duration) []core.Outcome {
	t.Helper()
	f.sweepAt(after)
	p := expiredWatchProposal(t, f)
	p, release, err := f.svc.ClaimProposal(fmt.Sprint(p.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	out, err := f.svc.ApproveWatchBurial(p)
	if err != nil {
		t.Fatal(err)
	}
	return []core.Outcome{out}
}

func TestWatchPendingBurialIsIdle(t *testing.T) {
	f, path, _ := watchFixture(t, "tree/file.zip", core.WatchFirstSeen)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	f.sweepAt(2 * time.Hour)
	before := f.svc.Store.ModTime()
	notifications := 0
	f.svc.AfterChange = func(*core.Service) { notifications++ }
	commits := 0
	for deadline := time.Now().Add(200 * time.Millisecond); time.Now().Before(deadline); {
		f.sweepAt(0)
		if !f.svc.Store.ModTime().Equal(before) {
			commits++
			before = f.svc.Store.ModTime()
		}
	}
	st, err := f.svc.Store.Read()
	_, due := st.NextDue()
	if err != nil || commits != 0 || notifications != 0 || due || !exists(path) || len(st.Burials) != 0 {
		t.Fatalf("pending review must stay idle and preserve content: commits=%d notifications=%d due=%v err=%v", commits, notifications, due, err)
	}
}

func TestWatchDirectoryRequiresBurialApproval(t *testing.T) {
	f, path, _ := watchFixture(t, "tree/file.zip", core.WatchFirstSeen)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	out := f.sweepAt(2 * time.Hour)
	if !exists(path) || len(f.burials()) != 0 || len(out) != 1 || out[0].Action != "proposed" {
		t.Fatalf("unapproved directory moved: %+v", out)
	}
	p := expiredWatchProposal(t, f)
	f.sweepAt(time.Hour)
	if next := expiredWatchProposal(t, f); core.ProposalFingerprint(next) != core.ProposalFingerprint(p) {
		t.Fatal("unchanged directory proposal lost its review")
	}
	rows, err := (&auto.Engine{Svc: f.svc}).Approve([]string{fmt.Sprint(p.ID)})
	if err != nil || len(rows) != 1 || rows[0].Error != "" || exists(path) || len(f.burials()) != 1 {
		t.Fatalf("approved directory was not buried: %+v %v", rows, err)
	}
}

func TestWatchDirectoryRejectionSparesOnlyChild(t *testing.T) {
	f, path, policy := watchFixture(t, "tree/file.zip", core.WatchFirstSeen)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	f.sweepAt(2 * time.Hour)
	p := expiredWatchProposal(t, f)
	rows, err := f.svc.Reject([]string{fmt.Sprint(p.ID)})
	if err != nil || rows[0].Error != "" {
		t.Fatalf("reject: %+v %v", rows, err)
	}
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.Read()
	if err != nil || len(st.Items) != 0 || st.Watches[0].Consent != core.ConsentApproved || len(st.WatchExemptions) != 1 || st.WatchExemptions[0].WatchID != policy.ID || !exists(path) {
		t.Fatalf("directory rejection failed to spare only this child: %+v %v", st, err)
	}
	e := st.WatchExemptions[0]
	if e.Reason != "rejected" || e.Path != filepath.Dir(path) || !e.KeptAt.Equal(f.clock) {
		t.Fatalf("rejection details: %+v", e)
	}
	f.file("downloads/other.txt")
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	if len(f.items()) != 1 {
		t.Fatal("rejection disabled remaining folder policy")
	}
}

func TestWatchNestedExplicitKeepAppearsInProposal(t *testing.T) {
	for _, restore := range []bool{false, true} {
		t.Run(fmt.Sprint(restore), func(t *testing.T) {
			f, path, _ := watchFixture(t, "tree/a/b/keep.txt", core.WatchFirstSeen)
			if err := watch.Reconcile(f.svc); err != nil {
				t.Fatal(err)
			}
			f.add(path, time.Hour)
			if restore {
				rows, err := f.svc.Bury([]string{path}, false, "")
				if err != nil || core.GraveFailed(rows) {
					t.Fatalf("bury: %+v %v", rows, err)
				}
				res, err := f.svc.Resurrect([]string{fmt.Sprint(rows[0].Burial.ID)})
				if err != nil || core.GraveFailed(res) {
					t.Fatalf("restore: %+v %v", res, err)
				}
			} else {
				spareWatchFixture(t, f, path, false)
			}
			st, err := f.svc.Store.Read()
			if err != nil || len(st.WatchExemptions) != 1 {
				t.Fatalf("nested keep did not receive policy exemption: %+v %v", st.WatchExemptions, err)
			}
			f.sweepAt(2 * time.Hour)
			p := expiredWatchProposal(t, f)
			if !exists(path) || !strings.Contains(p.Candidates[0].Rule, filepath.Base(path)) {
				t.Fatalf("nested keep absent from burial warning: %+v", p)
			}
		})
	}
}

func TestWatchBurialApprovalRechecksChanges(t *testing.T) {
	for _, change := range []string{"tracked", "moved", "extended", "kept"} {
		t.Run(change, func(t *testing.T) {
			f, path, _ := watchFixture(t, "tree/file.zip", core.WatchFirstSeen)
			if err := watch.Reconcile(f.svc); err != nil {
				t.Fatal(err)
			}
			f.sweepAt(2 * time.Hour)
			p := expiredWatchProposal(t, f)
			switch change {
			case "tracked":
				if out, err := exec.Command("git", "-C", f.dir, "add", "--", "downloads/tree/file.zip").CombinedOutput(); err != nil {
					t.Fatalf("git: %s %v", out, err)
				}
			case "moved":
				dest := filepath.Join(f.dir, "outside")
				if err := os.Rename(filepath.Dir(path), dest); err != nil {
					t.Fatal(err)
				}
				path = filepath.Join(dest, "file.zip")
			case "extended":
				if rows, err := f.svc.Extend([]string{filepath.Dir(path)}, time.Hour); err != nil || core.Failed(rows) {
					t.Fatalf("extend: %+v %v", rows, err)
				}
			case "kept":
				f.add(path, time.Hour)
				spareWatchFixture(t, f, path, false)
			}
			rows, err := (&auto.Engine{Svc: f.svc}).ApproveReviewed([]string{fmt.Sprint(p.ID)}, []string{core.ProposalFingerprint(p)})
			if err == nil && len(rows) == 1 && rows[0].Error == "" || !exists(path) || len(f.burials()) != 0 {
				t.Fatalf("changed burial approval moved content: %+v %v", rows, err)
			}
		})
	}
}

func TestWatchBurialProposalWithdrawnByLifespanEdit(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(fmt.Sprint(cancel), func(t *testing.T) {
			f, path, _ := watchFixture(t, "tree/file.zip", core.WatchFirstSeen)
			if err := watch.Reconcile(f.svc); err != nil {
				t.Fatal(err)
			}
			f.sweepAt(2 * time.Hour)
			if cancel {
				spareWatchFixture(t, f, filepath.Dir(path), false)
			} else {
				if _, err := f.svc.Extend([]string{filepath.Dir(path)}, time.Hour); err != nil {
					t.Fatal(err)
				}
			}
			st, err := f.svc.Store.Read()
			if err != nil || len(st.Proposals) != 0 || !exists(path) {
				t.Fatalf("obsolete burial proposal survived lifespan edit: %+v %v", st.Proposals, err)
			}
		})
	}
}
