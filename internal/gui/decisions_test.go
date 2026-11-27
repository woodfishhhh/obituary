package gui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	cliproposals "github.com/woodfishhhh/obituary/internal/cli/proposals"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/rules"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"github.com/woodfishhhh/obituary/internal/watch"
)

func TestProposalListMatchesCLIAndRejectsReappearingCandidate(t *testing.T) {
	for _, decide := range []string{"approve", "reject"} {
		t.Run(decide, func(t *testing.T) {
			a, root := fixture(t)
			p := propose(t, a, root, "keep.log")
			gone := filepath.Join(root, "gone.log")
			if _, _, _, err := a.deps.Service.Propose(p.Kind, p.Project, p.Session, []core.Candidate{{Path: gone, Lifespan: "2d"}}); err != nil {
				t.Fatal(err)
			}
			rows, err := a.Proposals()
			if err != nil || len(rows.Proposals) != 1 || len(rows.Proposals[0].Candidates) != 1 {
				t.Fatalf("GUI displayed absent candidate: %+v %v", rows, err)
			}
			t.Setenv("OBIT_HOME", a.deps.Service.Store.Home)
			out, _, err := clitest.Execute(t, cliproposals.NewCmd(), "--json")
			var cli []core.Proposal
			if err != nil || json.Unmarshal([]byte(out), &cli) != nil || len(cli) != 1 || !reflect.DeepEqual(cli[0], rows.Proposals[0].Proposal) {
				t.Fatalf("GUI/CLI mismatch: %+v %s %v", rows, out, err)
			}
			write(t, gone, "reappeared without being reviewed")
			action := a.Approve
			if decide == "reject" {
				action = a.Reject
			}
			reply, err := action([]string{ref(p.ID)}, []string{rows.Proposals[0].Fingerprint})
			st, readErr := a.deps.Service.Store.Read()
			if err != nil || readErr != nil || len(reply.Results) != 1 || reply.Results[0].Error == "" || len(st.Items) != 0 || len(st.Projects) != 0 || len(st.Proposals) != 1 || len(st.LeftoverRejections) != 0 {
				t.Fatalf("hidden candidate authorized: %+v %+v %v %v", reply, st, err, readErr)
			}
			if data, err := os.ReadFile(gone); err != nil || string(data) != "reappeared without being reviewed" {
				t.Fatalf("candidate touched: %s %v", data, err)
			}
		})
	}
}

func TestGUIExpiredWatchDirectoryDecisions(t *testing.T) {
	testenv.WindowsOnly(t, "GUI watch burial uses the Windows writer-exclusion capability; POSIX retention is covered by core watch tests")
	if testing.Short() {
		t.Skip("folder approval uses a real Git repository")
	}
	for _, approve := range []bool{true, false} {
		t.Run(refBool(!approve), func(t *testing.T) {
			a, root := fixture(t)
			a.deps.Service.OwnerBatch, a.deps.Service.PrepareOwners = auto.OwnerBatch, auto.PrepareOwners
			repo := filepath.Join(root, "repo")
			testenv.InitRepo(t, repo)
			path := filepath.Join(repo, "downloads", "tree", "file.txt")
			write(t, path, "payload")
			now := time.Now()
			a.deps.Service.Now = func() time.Time { return now }
			if _, err := a.SetWatch(filepath.Dir(filepath.Dir(path)), "1h", core.WatchFirstSeen); err != nil {
				t.Fatal(err)
			}
			guiApproveOnlyProposal(t, a)
			if err := watch.Reconcile(a.deps.Service); err != nil {
				t.Fatal(err)
			}
			now = now.Add(2 * time.Hour)
			if _, err := a.deps.Service.Sweep(false); err != nil {
				t.Fatal(err)
			}
			rows, err := a.Proposals()
			if err != nil || len(rows.Proposals) != 1 || rows.Proposals[0].Kind != core.ProposalWatchBurial || rows.Proposals[0].WatchBurial.Files != 1 {
				t.Fatalf("GUI burial proposal missing: %+v %v", rows, err)
			}
			p := rows.Proposals[0]
			decide := a.Reject
			if approve {
				decide = a.Approve
			}
			reply, err := decide([]string{ref(p.ID)}, []string{p.Fingerprint})
			if err != nil || reply.Error != "" || reply.Results[0].Error != "" {
				t.Fatalf("GUI decision failed: %+v %v", reply, err)
			}
			st, err := a.deps.Service.Store.Read()
			if err != nil || len(st.Proposals) != 0 || len(st.Items) != 0 || (len(st.Burials) == 1) != approve {
				t.Fatalf("GUI decision did not settle folder: %+v %v", st, err)
			}
		})
	}
}

func guiApproveOnlyProposal(t *testing.T, a *API) {
	t.Helper()
	rows, err := a.Proposals()
	if err != nil || len(rows.Proposals) != 1 {
		t.Fatalf("policy proposal: %+v %v", rows, err)
	}
	p := rows.Proposals[0]
	res, err := a.Approve([]string{ref(p.ID)}, []string{p.Fingerprint})
	if err != nil || res.Error != "" || res.Results[0].Error != "" {
		t.Fatalf("approve policy: %+v %v", res, err)
	}
}

func propose(t *testing.T, a *API, repo, name string) core.Proposal {
	t.Helper()
	path := filepath.Join(repo, name)
	write(t, path, "candidate")
	p, _, _, err := a.deps.Service.Propose(core.ProposalOnboard, repo, "gui-test", []core.Candidate{{Path: path, Lifespan: "2d"}})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestProposalListApproveAndReject(t *testing.T) {
	if testing.Short() {
		t.Skip("approval registers candidates in a real Git repository")
	}
	a, root := fixture(t)
	repo := repository(t, root)
	p := propose(t, a, repo, "first.log")
	list, err := a.Proposals()
	if err != nil || len(list.Proposals) != 1 {
		t.Fatalf("proposals: %+v %v", list, err)
	}
	approved, err := a.Approve([]string{ref(p.ID), "999999"}, []string{list.Proposals[0].Fingerprint, "missing"})
	if err != nil || approved.Error != "" || approved.Results[0].Error != "" || len(approved.Results[0].Results) != 1 || approved.Results[0].Results[0].Action != core.ActionAdded || approved.Results[1].Error == "" {
		t.Fatalf("approve: %+v %v", approved, err)
	}
	p = propose(t, a, repo, "second.log")
	rejected, err := a.Reject([]string{ref(p.ID), "999999"}, []string{core.ProposalFingerprint(p), "missing"})
	if err != nil || rejected.Error != "" || rejected.Results[0].Error != "" || rejected.Results[1].Error == "" {
		t.Fatalf("reject: %+v %v", rejected, err)
	}
	if _, err := a.Approve(nil, nil); err == nil {
		t.Fatal("empty approval accepted")
	}
	if _, err := a.Reject(nil, nil); err == nil {
		t.Fatal("empty rejection accepted")
	}
	breakStore(t, a)
	if _, err := a.Proposals(); err == nil {
		t.Fatal("store failure hidden")
	}
}

func TestProposalDecisionRejectsUnseenCandidates(t *testing.T) {
	if testing.Short() {
		t.Skip("approval checks a real Git repository")
	}
	for _, reject := range []bool{false, true} {
		t.Run(refBool(reject), func(t *testing.T) {
			a, root := fixture(t)
			repo := repository(t, root)
			p := propose(t, a, repo, "reviewed.log")
			list, err := a.Proposals()
			if err != nil || len(list.Proposals) != 1 || list.Proposals[0].Fingerprint == "" {
				t.Fatalf("missing proposal fingerprint: %+v %v", list, err)
			}
			fingerprints := []string{list.Proposals[0].Fingerprint}
			propose(t, a, repo, "unseen.log")
			var reply ProposalReply
			if reject {
				reply, err = a.Reject([]string{ref(p.ID)}, fingerprints)
			} else {
				reply, err = a.Approve([]string{ref(p.ID)}, fingerprints)
			}
			state, readErr := a.deps.Service.Store.Read()
			if err != nil || readErr != nil || len(reply.Results) != 1 || reply.Results[0].Error == "" || len(state.Items) != 0 || len(state.Proposals) != 1 || len(state.Projects) != 0 {
				t.Fatalf("unseen proposal changed ledger: %+v %+v %v %v", reply, state, err, readErr)
			}
		})
	}
}

func TestCandidateReceiptMatchesPartialApproval(t *testing.T) {
	a, root := fixture(t)
	repo := repository(t, root)
	p := propose(t, a, repo, "ok.log")
	for _, name := range []string{"gone.log", "retry.log"} {
		path := filepath.Join(repo, name)
		write(t, path, "candidate")
		var err error
		p, _, _, err = a.deps.Service.Propose(p.Kind, repo, p.Session, []core.Candidate{{Path: path, Lifespan: "2d"}})
		if err != nil {
			t.Fatal(err)
		}
	}
	rows, err := a.Proposals()
	if err != nil || len(rows.Proposals) != 1 || len(rows.Proposals[0].Candidates) != 3 {
		t.Fatalf("preview: %+v %v", rows, err)
	}
	if err := os.Rename(filepath.Join(repo, "gone.log"), filepath.Join(repo, "away.log")); err != nil {
		t.Fatal(err)
	}
	retry := filepath.Join(repo, "retry.log")
	a.deps.Service.FS = denyCandidateFS{FS: a.deps.Service.FS, path: retry}
	reply, err := a.Approve([]string{ref(p.ID)}, []string{rows.Proposals[0].Fingerprint})
	if err != nil || len(reply.Results) != 1 {
		t.Fatalf("reply: %+v %v", reply, err)
	}
	state, err := a.deps.Service.Store.Read()
	if err != nil || len(state.Items) != 1 || len(state.Proposals) != 1 || len(state.Proposals[0].Candidates) != 1 || state.Proposals[0].Candidates[0].Path != retry {
		t.Fatalf("state: %+v %v", state, err)
	}
	raw, err := json.Marshal(reply.Results[0])
	if err != nil {
		t.Fatal(err)
	}
	var receipt struct {
		Remaining []string
		Results   []core.Result
	}
	if err := json.Unmarshal(raw, &receipt); err != nil {
		t.Fatal(err)
	}
	if len(receipt.Results) != 3 || receipt.Results[0].Action != core.ActionAdded || receipt.Results[1].Error == "" || receipt.Results[2].Error == "" || !reflect.DeepEqual(receipt.Remaining, []string{retry}) {
		t.Fatalf("candidate receipt: %s", raw)
	}
}

type denyCandidateFS struct {
	core.FS
	path string
}

func (f denyCandidateFS) Identify(path string) (core.Identity, error) {
	if path == f.path {
		return core.Identity{}, os.ErrPermission
	}
	return f.FS.Identify(path)
}

func refBool(b bool) string {
	if b {
		return "reject"
	}
	return "approve"
}

func TestProposalReviewValidation(t *testing.T) {
	a, root := fixture(t)
	p := propose(t, a, root, "reviewed.log")
	propose(t, a, root, "unseen.log")
	for _, fingerprints := range [][]string{nil, {""}, {core.ProposalFingerprint(p)}} {
		approved, err := a.Approve([]string{ref(p.ID)}, fingerprints)
		if err != nil || approved.Error == "" && (len(approved.Results) != 1 || approved.Results[0].Error == "") {
			t.Fatalf("unreviewed approval accepted: %+v %v", approved, err)
		}
		rejected, err := a.Reject([]string{ref(p.ID)}, fingerprints)
		if err != nil || rejected.Error == "" && (len(rejected.Results) != 1 || rejected.Results[0].Error == "") {
			t.Fatalf("unreviewed rejection accepted: %+v %v", rejected, err)
		}
	}
	state, err := a.deps.Service.Store.Read()
	if err != nil || len(state.Items) != 0 || len(state.Proposals) != 1 || len(state.Projects) != 0 {
		t.Fatalf("unreviewed decision changed ledger: %+v %v", state, err)
	}
}

func TestApproveRefreshesRules(t *testing.T) {
	if testing.Short() {
		t.Skip("approval checks a real Git repository")
	}
	for _, change := range []string{"trust", "external-edit", "untrust"} {
		t.Run(change, func(t *testing.T) {
			a, root := fixture(t)
			repo := repository(t, root)
			path := filepath.Join(repo, ".obitrules")
			initial := "*.log 2d\n"
			if change == "untrust" {
				initial += "!blocked.log\n"
			}
			write(t, path, initial)
			file, _ := a.deps.Engine.Trust.Check(path)
			if _, err := a.Trust(path, file.SHA256); err != nil {
				t.Fatal(err)
			}
			p := propose(t, a, repo, "first.log")
			if reply, err := a.Approve([]string{ref(p.ID)}, []string{core.ProposalFingerprint(p)}); err != nil || reply.Results[0].Results[0].Action != core.ActionAdded {
				t.Fatalf("prime rules: %+v %v", reply, err)
			}
			if change == "untrust" {
				if _, err := a.Untrust(path); err != nil {
					t.Fatal(err)
				}
			} else {
				write(t, path, "!blocked.log\n")
				if change == "trust" {
					file, _ = a.deps.Engine.Trust.Check(path)
					if _, err := a.Trust(path, file.SHA256); err != nil {
						t.Fatal(err)
					}
				} else {
					// The supported override makes an external edit effective immediately.
					t.Setenv("OBIT_TRUST_PROJECT_RULES", "1")
				}
			}
			p = propose(t, a, repo, "blocked.log")
			reply, err := a.Approve([]string{ref(p.ID)}, []string{core.ProposalFingerprint(p)})
			state, readErr := a.deps.Service.Store.Read()
			wantItems := 1
			if change == "untrust" {
				wantItems = 2
			}
			if err != nil || readErr != nil || len(state.Items) != wantItems || len(reply.Results) != 1 {
				t.Fatalf("stale rules affected candidate: %+v %+v %v %v", reply, state.Items, err, readErr)
			}
		})
	}
}

func TestApproveRefreshesRulesDuringReview(t *testing.T) {
	if testing.Short() {
		t.Skip("approval checks a real Git repository")
	}
	for _, change := range []string{"trust", "external-edit", "untrust"} {
		t.Run(change, func(t *testing.T) {
			a, root := fixture(t)
			repo := repository(t, root)
			path := filepath.Join(repo, ".obitrules")
			write(t, path, "*.log 2d\n")
			trustRuleFile(t, a, path)
			p := propose(t, a, repo, "blocked.log")
			// Identify follows the first rule snapshot, while approval is still pending.
			a.deps.Service.FS = &approvalRuleChangeFS{FS: a.deps.Service.FS, before: func() {
				if change == "untrust" {
					if _, err := a.deps.Engine.Trust.Untrust(path); err != nil {
						t.Fatal(err)
					}
					return
				}
				write(t, path, "!blocked.log\n")
				if change == "trust" {
					trustRuleFile(t, a, path)
				}
			}}
			reply, err := a.Approve([]string{ref(p.ID)}, []string{core.ProposalFingerprint(p)})
			state, readErr := a.deps.Service.Store.Read()
			if err != nil || readErr != nil || reply.Error == "" || len(reply.Results) != 1 || len(state.Items) != 0 || len(state.Proposals) != 1 {
				t.Fatalf("stale rules affected candidate: %+v %+v %v %v", reply, state, err, readErr)
			}
			results := reply.Results[0].Results
			if len(results) != 1 || !strings.Contains(results[0].Error, "rules") || core.ProposalFingerprint(state.Proposals[0]) != core.ProposalFingerprint(p) {
				t.Fatalf("stale rules affected candidate: %+v %+v", reply, state.Proposals)
			}
		})
	}
}

func trustRuleFile(t *testing.T, a *API, path string) {
	t.Helper()
	file, err := a.deps.Engine.Trust.Check(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.deps.Engine.Trust.Trust(path, file.SHA256); err != nil {
		t.Fatal(err)
	}
}

type approvalRuleChangeFS struct {
	core.FS
	before func()
}

func (f *approvalRuleChangeFS) Identify(path string) (core.Identity, error) {
	if f.before != nil {
		before := f.before
		f.before = nil
		before()
	}
	return f.FS.Identify(path)
}

func TestRulesReviewDigestTrustAndUntrust(t *testing.T) {
	a, root := fixture(t)
	repo := filepath.Join(root, "repo") // rule digests do not execute Git
	propose(t, a, repo, "file.log")
	path := filepath.Join(repo, ".obitrules")
	write(t, path, "*.log 2d\n")
	list, err := a.Rules()
	if err != nil || len(list.Rules) != 1 || list.Rules[0].Status != rules.Untrusted || list.Rules[0].Content == "" {
		t.Fatalf("rules: %+v %v", list, err)
	}
	if _, err := a.Trust(path, ""); err == nil {
		t.Fatal("unreviewed trust accepted")
	}
	if _, err := a.Trust(path, "wrong"); err == nil {
		t.Fatal("stale digest accepted")
	}
	trusted, err := a.Trust(path, list.Rules[0].SHA256)
	if err != nil || trusted.Status != rules.Trusted {
		t.Fatalf("trust: %+v %v", trusted, err)
	}
	receipt, err := a.Untrust(path)
	if err != nil || !receipt.Changed {
		t.Fatalf("untrust: %+v %v", receipt, err)
	}
	write(t, a.deps.Engine.Trust.Path, "broken")
	if _, err := a.Rules(); err == nil {
		t.Fatal("trust store failure hidden")
	}
	if _, err := a.Untrust(path); err == nil {
		t.Fatal("untrust failure hidden")
	}
}

func TestWatchesReusePolicyConsent(t *testing.T) {
	a, root := fixture(t)
	folder := filepath.Join(root, "downloads")
	write(t, filepath.Join(folder, "child.log"), "child")
	set, err := a.SetWatch(folder, "2d", "")
	if err != nil || set.Error != "" || set.Policy.Consent != "pending" {
		t.Fatalf("set: %+v %v", set, err)
	}
	policies, err := a.Watches()
	if err != nil || len(policies.Policies) != 1 {
		t.Fatalf("watches: %+v %v", policies, err)
	}
	items, err := a.Upcoming("")
	if err != nil || len(items.Items) != 0 {
		t.Fatalf("reconcile: %+v %v", items, err)
	}
	initial, err := a.Proposals()
	if err != nil || len(initial.Proposals) != 1 {
		t.Fatalf("first-seen proposal: %+v %v", initial, err)
	}
	if approved, err := a.Approve([]string{ref(initial.Proposals[0].ID)}, []string{initial.Proposals[0].Fingerprint}); err != nil || approved.Error != "" || approved.Results[0].Error != "" {
		t.Fatalf("first-seen approve: %+v %v", approved, err)
	}
	set, err = a.SetWatch(folder, "2d", core.WatchModified)
	if err != nil || set.Error != "" || set.Policy.Consent == core.ConsentApproved {
		t.Fatalf("modified policy needs approval: %+v %v", set, err)
	}
	proposals, err := a.Proposals()
	if err != nil || len(proposals.Proposals) != 1 || proposals.Proposals[0].Kind != core.ProposalWatch {
		t.Fatalf("policy proposal: %+v %v", proposals, err)
	}
	approved, err := a.Approve([]string{ref(proposals.Proposals[0].ID)}, []string{proposals.Proposals[0].Fingerprint})
	if err != nil || approved.Error != "" || approved.Results[0].Error != "" {
		t.Fatalf("policy approve: %+v %v", approved, err)
	}
	removed, err := a.RemoveWatch(folder)
	if err != nil || !removed.Changed {
		t.Fatalf("remove: %+v %v", removed, err)
	}
	items, err = a.Upcoming("")
	if err != nil || len(items.Items) != 0 {
		t.Fatalf("remove left lifespans: %+v %v", items, err)
	}
	if _, err := a.RemoveWatch(folder); err == nil {
		t.Fatal("missing policy accepted")
	}
	invalid, err := a.SetWatch(folder, "bad", "")
	if err == nil && invalid.Error == "" {
		t.Fatal("invalid policy accepted")
	}
	breakStore(t, a)
	if _, err := a.Watches(); err == nil {
		t.Fatal("store failure hidden")
	}
}

func TestRuleListDeduplicatesKnownProjectSpellings(t *testing.T) {
	a, root := fixture(t)
	repo := filepath.Join(root, "repo")
	write(t, filepath.Join(repo, ".obitrules"), "*.log 2d\n")
	propose(t, a, repo, "one.log")
	if err := a.deps.Service.Store.Update(func(st *core.State) error {
		st.Projects = map[string]core.ProjectState{}
		st.Projects[core.ProjectKey(strings.ToUpper(repo))] = core.ProjectState{Consent: core.ConsentApproved, At: time.Now()}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	files, err := a.Rules()
	if err != nil || len(files.Rules) != 1 {
		t.Fatalf("duplicate decisions: %+v %v", files, err)
	}
}

func TestRuleListUsesProjectDirectoryIdentity(t *testing.T) {
	a, root := fixture(t)
	first, second := filepath.Join(root, "Project"), filepath.Join(root, "project")
	for _, repo := range []string{first, second} {
		write(t, filepath.Join(repo, ".obitrules"), "*.log 2d\n")
	}
	left, err := os.Stat(first)
	if err != nil {
		t.Fatal(err)
	}
	right, err := os.Stat(second)
	if err != nil {
		t.Fatal(err)
	}
	want := 2
	if os.SameFile(left, right) {
		want = 1
	}
	if err := a.deps.Service.Store.Update(func(st *core.State) error {
		st.Projects = map[string]core.ProjectState{first: {}, second: {}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	files, err := a.Rules()
	if err != nil || len(files.Rules) != want {
		t.Fatalf("project identities require %d rule decisions: %+v %v", want, files, err)
	}
}

func TestProposalValidationAndRejectionWithoutGit(t *testing.T) {
	a, root := fixture(t)
	p := propose(t, a, root, "candidate.log")
	list, err := a.Proposals()
	if err != nil || len(list.Proposals) != 1 {
		t.Fatalf("proposal list: %+v %v", list, err)
	}
	if _, err := a.Approve(nil, nil); err == nil {
		t.Fatal("empty approval accepted")
	}
	if _, err := a.Reject(nil, nil); err == nil {
		t.Fatal("empty rejection accepted")
	}
	missing, err := a.Approve([]string{"999999"}, []string{"missing"})
	if err != nil || len(missing.Results) != 1 || missing.Results[0].Error == "" {
		t.Fatalf("missing proposal approved: %+v %v", missing, err)
	}
	rejected, err := a.Reject([]string{ref(p.ID)}, []string{list.Proposals[0].Fingerprint})
	if err != nil || rejected.Error != "" || len(rejected.Results) != 1 || rejected.Results[0].Error != "" {
		t.Fatalf("reject: %+v %v", rejected, err)
	}
	list, err = a.Proposals()
	if err != nil || len(list.Proposals) != 0 {
		t.Fatalf("rejected proposal remains: %+v %v", list, err)
	}
	breakStore(t, a)
	if _, err := a.Proposals(); err == nil {
		t.Fatal("proposal list hid store failure")
	}
}

// The Decisions page shows the proposal mode next to the proposals, switches
// it in place, and lists what went in without asking until it is spared.
func TestDecisionsShowModeAndWhatWentInWithoutAsking(t *testing.T) {
	a, root := fixture(t)
	path := filepath.Join(root, "debug.log")
	write(t, path, "x")
	if err := a.deps.Service.Store.Update(func(st *core.State) error {
		st.Items = append(st.Items, core.Item{ID: st.NextID, Rev: 1, Path: path, Source: "rule", Project: root, ExpiresAt: time.Now().Add(time.Hour)})
		st.AddAutoReceipts(core.AutoReceipt{ID: st.NextID, Class: core.AutoCreated, Project: root, At: time.Now()})
		st.NextID++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := a.Proposals()
	if err != nil || rows.Mode != core.ProposalsOff || len(rows.Handled) != 1 || rows.Handled[0].Class != core.AutoCreated || rows.Handled[0].Item.Path != path {
		t.Fatalf("decisions view: %+v %v", rows, err)
	}
	plan, err := a.PlanProposalMode(core.ProposalsAuto)
	if err != nil || len(plan.Changes) != 1 {
		t.Fatalf("preview: %+v %v", plan, err)
	}
	if _, err := a.ApplyPlan(plan.ID); err != nil {
		t.Fatal(err)
	}
	if rows, err = a.Proposals(); err != nil || rows.Mode != core.ProposalsAuto || a.deps.Service.Proposals() != core.ProposalsAuto {
		t.Fatalf("mode not switched: %+v %v", rows, err)
	}
	if _, err := a.PlanProposalMode("everything"); err == nil {
		t.Fatal("unknown mode accepted")
	}
	if reply, err := a.Cancel([]string{ref(rows.Handled[0].Item.ID)}); err != nil || reply.Error != "" {
		t.Fatalf("undo: %+v %v", reply, err)
	}
	if rows, err = a.Proposals(); err != nil || len(rows.Handled) != 0 {
		t.Fatalf("spared item still listed: %+v %v", rows, err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("undo must keep the file: %v", err)
	}
}

func TestGUIQuotaDecisionsPurgeAndExtend(t *testing.T) {
	a, root := fixture(t)
	a.deps.Service.GraveyardMaxBytes = 5000
	oldPath := filepath.Join(root, "work", "old.bin")
	write(t, oldPath, strings.Repeat("o", 4000))
	if _, err := a.deps.Service.Bury([]string{oldPath}, false, ""); err != nil {
		t.Fatal(err)
	}

	// Oversized file #1: permanent delete via GUI leaves old grave intact.
	huge1 := filepath.Join(root, "work", "huge1.bin")
	write(t, huge1, strings.Repeat("h", 8000))
	if _, err := a.deps.Service.Bury([]string{huge1}, false, ""); err != nil {
		t.Fatal(err)
	}
	sum, err := a.Summary()
	if err != nil || sum.QuotaPendingCount != 1 || sum.DecisionCount != 1 {
		t.Fatalf("summary with quota decision: %+v %v", sum, err)
	}
	rows, err := a.Proposals()
	if err != nil || len(rows.Proposals) != 1 || rows.Proposals[0].Kind != core.ProposalGraveyardQuota || rows.Proposals[0].QuotaReview == nil {
		t.Fatalf("quota proposal missing: %+v %v", rows, err)
	}
	p1 := rows.Proposals[0]
	purgeReply, err := a.PurgeQuotaProposal(ref(p1.ID), p1.Fingerprint)
	if err != nil || purgeReply.Error != "" || len(purgeReply.Results) != 1 || purgeReply.Results[0].Error != "" {
		t.Fatalf("PurgeQuotaProposal: %+v %v", purgeReply, err)
	}
	if _, err := os.Stat(huge1); !os.IsNotExist(err) {
		t.Fatalf("huge1 should be permanently deleted, stat=%v", err)
	}
	page, err := a.Graveyard(GraveyardQuery{Limit: 50})
	if err != nil || page.Total != 1 {
		t.Fatalf("old grave must stay intact: %+v %v", page, err)
	}

	// Oversized file #2: extend via GUI tracks it and clears pending decision.
	huge2 := filepath.Join(root, "work", "huge2.bin")
	write(t, huge2, strings.Repeat("k", 7000))
	if _, err := a.deps.Service.Bury([]string{huge2}, false, ""); err != nil {
		t.Fatal(err)
	}
	rows, err = a.Proposals()
	if err != nil || len(rows.Proposals) != 1 {
		t.Fatalf("second quota proposal: %+v %v", rows, err)
	}
	p2 := rows.Proposals[0]
	extReply, err := a.ExtendQuotaProposal(ref(p2.ID), p2.Fingerprint, "3d")
	if err != nil || extReply.Error != "" || len(extReply.Results) != 1 || extReply.Results[0].Error != "" {
		t.Fatalf("ExtendQuotaProposal: %+v %v", extReply, err)
	}
	if _, err := os.Stat(huge2); err != nil {
		t.Fatalf("extended file must stay in place: %v", err)
	}
	rows, err = a.Proposals()
	if err != nil || len(rows.Proposals) != 0 {
		t.Fatalf("extended quota proposal should be cleared: %+v %v", rows, err)
	}
}

func TestGUIRefreshesMaxSizeAndApprovesOversizedAfterLimitRaise(t *testing.T) {
	a, root := fixture(t)
	cfgPath := filepath.Join(a.deps.ConfigDir, "config.toml")
	write(t, cfgPath, "[graveyard]\nmax_size = \"5000\"\n")
	a.deps.Service.GraveyardMaxBytes = 5000

	huge := filepath.Join(root, "work", "huge-raise.bin")
	write(t, huge, strings.Repeat("x", 8000))
	if _, err := a.deps.Service.Bury([]string{huge}, false, ""); err != nil {
		t.Fatal(err)
	}
	rows, err := a.Proposals()
	if err != nil || len(rows.Proposals) != 1 || rows.GraveMaxBytes != 5000 {
		t.Fatalf("initial proposals: %+v %v", rows, err)
	}

	// Edit config.toml externally to raise max_size -> GUI Proposals & Approve pick it up immediately.
	write(t, cfgPath, "[graveyard]\nmax_size = \"20000\"\n")
	rows, err = a.Proposals()
	if err != nil || rows.GraveMaxBytes != 20000 {
		t.Fatalf("Proposals must expose updated GraveMaxBytes=20000, got %+v %v", rows, err)
	}
	p := rows.Proposals[0]
	appReply, err := a.Approve([]string{ref(p.ID)}, []string{p.Fingerprint})
	if err != nil || appReply.Error != "" || len(appReply.Results) != 1 || appReply.Results[0].Error != "" {
		t.Fatalf("Approve after external config raise: %+v %v", appReply, err)
	}
	if _, err := os.Stat(huge); !os.IsNotExist(err) {
		t.Fatalf("approved quota item should be buried, stat=%v", err)
	}

	// Also verify API.Bury picks up updated max_size from config.toml (Item #143 + #5).
	write(t, cfgPath, "[graveyard]\nmax_size = \"4000\"\n")
	f1 := filepath.Join(root, "work", "bury1.bin")
	f2 := filepath.Join(root, "work", "bury2.bin")
	write(t, f1, strings.Repeat("a", 3000))
	write(t, f2, strings.Repeat("b", 3000))
	addRes, err := a.deps.Service.Add([]string{f1, f2}, core.AddOptions{ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil || len(addRes) != 2 || core.Failed(addRes) {
		t.Fatalf("add f1/f2: %+v %v", addRes, err)
	}
	if _, err := a.Bury([]string{"#" + ref(addRes[0].Item.ID)}); err != nil {
		t.Fatal(err)
	}
	reply, err := a.Bury([]string{"#" + ref(addRes[1].Item.ID)})
	if err != nil || len(reply.Results) != 1 || reply.Results[0].QuotaEviction == nil || reply.Results[0].QuotaEviction.PurgedCount < 1 {
		t.Fatalf("API.Bury must enforce refreshed config.toml max_size: %+v %v", reply, err)
	}
}
