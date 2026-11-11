//go:build windows

package core_test

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
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/rules"
)

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
