//go:build windows

package lifecycle_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/rules"
)

func TestLeftoverProposalsDeduplicateAcrossSessions(t *testing.T) {
	f := newFixture(t)
	c := core.Candidate{Path: f.file("research.md"), Lifespan: "3d", Size: 5}
	first, _, _, err := f.svc.Propose(core.ProposalLeftovers, f.dir, "first", []core.Candidate{c})
	if err != nil {
		t.Fatal(err)
	}
	c.Size++
	if _, _, _, err := f.svc.Propose(core.ProposalLeftovers, f.dir, "second", []core.Candidate{c}); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.Read()
	if err != nil || len(st.Proposals) != 1 || !reflect.DeepEqual(st.Proposals[0], first) {
		t.Fatalf("same leftover has multiple pending proposals or changed its owner: %+v %v", st.Proposals, err)
	}
	fresh := core.Candidate{Path: f.file("fresh.md"), Lifespan: "3d"}
	second, _, added, err := f.svc.Propose(core.ProposalLeftovers, f.dir, "second", []core.Candidate{c, fresh})
	if err != nil || added != 1 || len(second.Candidates) != 1 || second.Candidates[0].Path != fresh.Path {
		t.Fatalf("deduplication suppressed distinct leftovers: %+v %d %v", second, added, err)
	}
	if _, _, added, err := f.svc.Propose(core.ProposalLeftovers, f.dir+"-other", "second", []core.Candidate{c}); err != nil || added != 1 {
		t.Fatalf("deduplication crossed projects: %d %v", added, err)
	}
}

func TestRejectedLeftoverStaysHandledAcrossSessions(t *testing.T) {
	f := newFixture(t)
	c := core.Candidate{Path: f.file("research.md"), Lifespan: "3d"}
	p, _, _, err := f.svc.Propose(core.ProposalLeftovers, f.dir, "first", []core.Candidate{c})
	if err != nil {
		t.Fatal(err)
	}
	if out, err := f.svc.Reject([]string{fmt.Sprint(p.ID)}); err != nil || len(out) != 1 || out[0].Error != "" {
		t.Fatalf("reject: %+v %v", out, err)
	}
	p, created, added, err := f.svc.Propose(core.ProposalLeftovers, f.dir, "second", []core.Candidate{c})
	if err != nil || p.ID != 0 || created || added != 0 {
		t.Fatalf("decided leftover was proposed again: %+v %v %d %v", p, created, added, err)
	}
	st, err := f.svc.Store.Read()
	if err != nil || len(st.Items) != 0 || len(st.Burials) != 0 || len(st.Proposals) != 0 {
		t.Fatalf("rejection changed registration: %+v %v", st, err)
	}
}

func TestPendingLeftoverIsProposedOnceAcrossSessions(t *testing.T) {
	f := newFixture(t)
	c := core.Candidate{Path: f.file("research.md"), Lifespan: "3d"}
	if _, _, _, err := f.svc.Propose(core.ProposalLeftovers, f.dir, "first", []core.Candidate{c}); err != nil {
		t.Fatal(err)
	}
	p, created, added, err := f.svc.Propose(core.ProposalLeftovers, f.dir, "second", []core.Candidate{c})
	if err != nil || p.ID != 0 || created || added != 0 {
		t.Fatalf("same leftover has multiple pending proposals: %+v %v %d %v", p, created, added, err)
	}
	if st, err := f.svc.Store.Read(); err != nil || len(st.Proposals) != 1 {
		t.Fatalf("same leftover has multiple pending proposals: %+v %v", st.Proposals, err)
	}
}

func TestLeftoverDecisionClearsLegacyTwins(t *testing.T) {
	for _, decision := range []string{"resolve", "reject", "retry", "refreshed"} {
		t.Run(decision, func(t *testing.T) {
			f := newFixture(t)
			c := core.Candidate{Path: f.file("research.md"), Lifespan: "3d", Rev: 2}
			fresh := core.Candidate{Path: f.file("fresh.md"), Lifespan: "3d", Rev: 33}
			p := core.Proposal{ID: 247, Kind: core.ProposalLeftovers, Project: f.dir, Session: "first", Candidates: []core.Candidate{c}}
			twin := p
			twin.ID, twin.Session = 248, "second"
			twin.Candidates = []core.Candidate{c, fresh}
			twin.Candidates[0].Rev = 32
			if err := f.svc.Store.Update(func(st *core.State) error {
				st.NextID, st.Proposals = 249, []core.Proposal{p, twin}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if decision == "reject" {
				if out, err := f.svc.Reject([]string{"247"}); err != nil || len(out) != 1 || out[0].Error != "" {
					t.Fatalf("reject: %+v %v", out, err)
				}
			} else {
				claimed, release, err := f.svc.ClaimProposal("247")
				if err != nil {
					t.Fatal(err)
				}
				defer release()
				var pending []core.Candidate
				if decision == "retry" {
					pending = claimed.Candidates
				}
				if decision == "refreshed" {
					if _, _, _, err := f.svc.Propose(p.Kind, p.Project, p.Session, []core.Candidate{c}); err != nil {
						t.Fatal(err)
					}
				}
				if err := f.svc.ResolveProposal(claimed, pending); err != nil {
					t.Fatal(err)
				}
			}
			st, err := f.svc.Store.Read()
			if err != nil {
				t.Fatal(err)
			}
			if decision == "retry" || decision == "refreshed" {
				if len(st.Proposals) != 2 || len(st.Proposals[1].Candidates) != 2 {
					t.Fatalf("unresolved leftover lost pending twins: %+v", st.Proposals)
				}
			} else if len(st.Proposals) != 1 || st.Proposals[0].ID != twin.ID || !reflect.DeepEqual(st.Proposals[0].Candidates, []core.Candidate{fresh}) {
				t.Fatalf("decided leftover retained legacy twin: %+v", st.Proposals)
			}
			if len(st.Items) != 0 || len(st.Burials) != 0 || len(st.Projects) != 0 {
				t.Fatalf("settling proposals granted consent or moved content: %+v", st)
			}
		})
	}
}

func TestProposalShortProjectSharesConsent(t *testing.T) {
	f := newFixture(t)
	path := f.file("debug.log")
	short := shortPath(t, f.dir)
	p, created, added, err := f.svc.Propose(core.ProposalOnboard, short, "s", []core.Candidate{{Path: path, Lifespan: "1d"}})
	if err != nil || !created || added != 1 || p.Project != f.dir {
		t.Fatalf("proposal retained short project: %+v %v %d %v", p, created, added, err)
	}
	again, created, _, err := f.svc.Propose(core.ProposalOnboard, f.dir, "s", p.Candidates)
	if err != nil || created || again.ID != p.ID || len(again.Candidates) != 1 {
		t.Fatalf("project alias split proposal: %+v %v %v", again, created, err)
	}
	_, release, err := f.svc.ClaimProposal(fmt.Sprint(p.ID))
	if err != nil {
		t.Fatal(err)
	}
	release()
	consent, err := f.svc.Consent(f.dir)
	if err != nil || consent != core.ConsentApproved {
		t.Fatalf("short project lost consent: %s %v", consent, err)
	}
}

func TestNotificationSurvivesPostRenameFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	core.RequireFsync(t)
	for _, event := range []string{"PostToolUse", "SessionStart"} {
		t.Run(event, func(t *testing.T) {
			f := newFixture(t)
			if out, err := exec.Command("git", "-C", f.dir, "init", "-q").CombinedOutput(); err != nil {
				t.Fatal(string(out), err)
			}
			path := f.file("debug.log")
			engine := &auto.Engine{Svc: f.svc, Trust: rules.TrustStore{Path: filepath.Join(f.svc.Store.Home, "trust.json")}}
			if _, _, _, err := f.svc.Propose(core.ProposalOnboard, f.dir, "s", []core.Candidate{{Path: path, Lifespan: "1d"}}); err != nil {
				t.Fatal(err)
			}
			failed := false
			stop := core.SetReplacementSync(func(file *os.File) error {
				st, err := f.svc.Store.Inspect()
				if err != nil {
					return err
				}
				if !failed && len(st.Proposals) == 1 && len(st.Proposals[0].Notified) > 0 {
					failed = true
					return errors.New("injected notification post-rename sync failure")
				}
				return file.Sync()
			})
			defer stop()
			payload, err := json.Marshal(map[string]any{"hook_event_name": event, "cwd": f.dir, "session_id": "s", "tool_name": "Write", "tool_input": map[string]any{"file_path": path}})
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			hook.Claude(engine, bytes.NewReader(payload), &out, nil)
			if !failed || !strings.Contains(out.String(), "proposal #") || !strings.Contains(out.String(), "debug.log") {
				t.Fatalf("notification vanished after an unconfirmed acknowledgement: %s", out.String())
			}
			data, err := os.ReadFile(filepath.Join(f.svc.Store.Home, "hook.log"))
			if err != nil || !strings.Contains(string(data), "notification post-rename sync failure") {
				t.Fatalf("notification failure was not logged: %s %v", data, err)
			}
		})
	}
}

func TestPruneProposalMatchesExactCandidatePath(t *testing.T) {
	f := newFixture(t)
	a := core.Candidate{Path: filepath.Join(f.dir, "ǅ.txt"), Rev: 7}
	b := core.Candidate{Path: filepath.Join(f.dir, "ǆ.txt"), Rev: 7}
	p := core.Proposal{ID: 1, Project: f.dir, Kind: core.ProposalLeftovers, Candidates: []core.Candidate{a, b}}
	if err := f.svc.Store.Update(func(st *core.State) error {
		st.Proposals = []core.Proposal{p}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.PruneProposal(p, []core.Candidate{a}); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.Read()
	if err != nil || len(st.Proposals) != 1 || !reflect.DeepEqual(st.Proposals[0].Candidates, []core.Candidate{b}) {
		t.Fatalf("pruning conflated distinct Unicode paths: %+v %v", st.Proposals, err)
	}
}

func TestExistingProposalsLeavesLedgerAndReadErrorsIntact(t *testing.T) {
	f := newFixture(t)
	path := f.file("keep.txt")
	p := core.Proposal{Candidates: []core.Candidate{{Path: path}, {Path: filepath.Join(f.dir, "missing")}, {Path: "invalid\x00path"}}} // legacy unreadable metadata
	visible := core.ExistingProposals([]core.Proposal{p})
	if len(visible) != 1 || len(visible[0].Candidates) != 2 || len(p.Candidates) != 3 {
		t.Fatalf("display lost unreadable candidate or mutated snapshot: %+v %+v", visible, p)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if got := core.ExistingProposals([]core.Proposal{{Candidates: p.Candidates[:2]}}); len(got) != 0 {
		t.Fatalf("empty proposal displayed: %+v", got)
	}
}

func TestPruneProposalPreservesConcurrentRevisionsAndConsent(t *testing.T) {
	f := newFixture(t)
	path := f.file("candidate.txt")
	p, _, _, err := f.svc.Propose(core.ProposalOnboard, f.dir, "s", []core.Candidate{{Path: path, Lifespan: "1d"}})
	if err != nil {
		t.Fatal(err)
	}
	updated, _, _, err := f.svc.Propose(core.ProposalOnboard, f.dir, "s", []core.Candidate{{Path: path, Lifespan: "3d"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.svc.PruneProposal(p, p.Candidates); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.Read()
	if err != nil || len(st.Proposals) != 1 || st.Proposals[0].Candidates[0].Lifespan != "3d" {
		t.Fatalf("pruning lost concurrent candidate revision: %+v %v", st.Proposals, err)
	}
	if err := f.svc.PruneProposal(updated, updated.Candidates); err != nil {
		t.Fatal(err)
	}
	st, err = f.svc.Store.Read()
	if err != nil || len(st.Proposals) != 0 || len(st.Projects) != 0 || len(st.LeftoverRejections) != 0 {
		t.Fatalf("pruning made a user decision: %+v %v", st, err)
	}
}

func TestRejectLeftoversLimitEvictsOldest(t *testing.T) {
	for _, limit := range []string{"sessions", "paths"} {
		t.Run(limit, func(t *testing.T) {
			f := newFixture(t)
			path := f.file("new.txt")
			if err := f.svc.Store.Update(func(st *core.State) error {
				if limit == "sessions" {
					for i := range 256 {
						st.LeftoverRejections = append(st.LeftoverRejections, core.LeftoverRejection{Project: f.dir, Session: fmt.Sprint(i), Paths: []string{path + ".old"}})
					}
				} else {
					r := core.LeftoverRejection{Project: f.dir, Session: "current"}
					for i := range 4096 {
						r.Paths = append(r.Paths, filepath.Join(f.dir, fmt.Sprint(i)))
					}
					st.LeftoverRejections = append(st.LeftoverRejections, r)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			p, _, _, err := f.svc.Propose(core.ProposalLeftovers, f.dir, "current", []core.Candidate{{Path: path, Lifespan: "3d"}})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := f.svc.Reject([]string{fmt.Sprint(p.ID)}); err != nil {
				t.Fatalf("new rejection must succeed at capacity: %v", err)
			}
			st, err := f.svc.Store.Read()
			if err != nil || len(st.Proposals) != 0 || !st.LeftoverRejected(f.dir, "current", path) {
				t.Fatalf("new rejection not saved: %+v, %v", st, err)
			}
			if limit == "sessions" {
				if len(st.LeftoverRejections) != 256 || st.LeftoverRejected(f.dir, "0", path+".old") {
					t.Fatal("oldest session was not evicted")
				}
			} else if len(st.LeftoverRejections[0].Paths) != 4096 || st.LeftoverRejected(f.dir, "current", filepath.Join(f.dir, "0")) {
				t.Fatal("oldest path was not evicted")
			}
		})
	}
}

func TestRejectionEvictsOldestTimestamp(t *testing.T) {
	f := newFixture(t)
	path := f.file("new.txt")
	if err := f.svc.Store.Update(func(st *core.State) error {
		for i := range 256 {
			at := f.clock.Add(-time.Minute)
			if i == 128 {
				at = at.Add(-time.Hour)
			}
			st.LeftoverRejections = append(st.LeftoverRejections, core.LeftoverRejection{Project: f.dir, Session: fmt.Sprint(i), Paths: []string{path + ".old"}, At: at})
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	p, _, _, err := f.svc.Propose(core.ProposalLeftovers, f.dir, "new", []core.Candidate{{Path: path, Lifespan: "1d"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Reject([]string{fmt.Sprint(p.ID)}); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.Read()
	if err != nil || len(st.LeftoverRejections) != 256 || st.LeftoverRejected(f.dir, "128", path+".old") || !st.LeftoverRejected(f.dir, "0", path+".old") || !st.LeftoverRejections[255].At.Equal(f.clock) {
		t.Fatalf("did not evict by timestamp or save rejection time: %v", err)
	}
}

func TestProposalSequenceSurvivesLegacyPruning(t *testing.T) {
	f := newFixture(t)
	a, b := f.file("a.log"), f.file("b.log")
	p := core.Proposal{ID: 1, Kind: core.ProposalLarge, Project: f.dir, Candidates: []core.Candidate{{Path: a, Rev: 9}, {Path: b, Rev: 2}}}
	if err := f.svc.Store.Update(func(st *core.State) error { st.Proposals = []core.Proposal{p}; return nil }); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.PruneProposal(p, p.Candidates[:1]); err != nil {
		t.Fatal(err)
	}
	got, _, _, err := f.svc.Propose(core.ProposalLarge, f.dir, "", p.Candidates[:1])
	if err != nil || got.Candidates[1].Rev <= 9 {
		t.Fatalf("legacy candidate sequence was reused: %+v %v", got, err)
	}
}
