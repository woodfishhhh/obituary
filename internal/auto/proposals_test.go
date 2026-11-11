package auto

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/rules"
)

func TestApproveRefreshesRulesWithinBatch(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository")
	}
	for _, change := range []string{"protect", "untrust", "changed"} {
		t.Run(change, func(t *testing.T) {
			r := newRepo(t)
			first, blocked := r.write("first.log", "first"), r.write("blocked.log", "keep")
			rule := r.write(rules.FileName, "*.log 30d\n")
			if change == "untrust" {
				r.write(rules.FileName, "!blocked.log\n")
			}
			if _, err := r.engine.Trust.Trust(rule, ""); err != nil {
				t.Fatal(err)
			}
			p, _, _, err := r.engine.Svc.Propose(core.ProposalLeftovers, r.root, "s", []core.Candidate{
				{Path: first, Lifespan: "30d"}, {Path: blocked, Lifespan: "30d"},
			})
			if err != nil {
				t.Fatal(err)
			}
			r.engine.Svc.FS = &approvalRulesFS{FS: r.engine.Svc.FS, before: func() {
				if change == "untrust" {
					if _, err := r.engine.Trust.Untrust(rule); err != nil {
						t.Fatal(err)
					}
					return
				}
				r.write(rules.FileName, "!blocked.log\n")
				if change == "protect" {
					if _, err := r.engine.Trust.Trust(rule, ""); err != nil {
						t.Fatal(err)
					}
				}
			}}
			out, approveErr := r.engine.ApproveReviewed([]string{fmt.Sprint(p.ID)}, []string{core.ProposalFingerprint(p)})
			st, err := r.engine.Svc.Store.Read()
			if err != nil || len(out) != 1 || len(out[0].Results) != 2 {
				t.Fatalf("batch result: %+v %v %v", out, approveErr, err)
			}
			if approveErr == nil || len(st.Items) != 0 || len(st.Proposals) != 1 || len(st.Proposals[0].Candidates) != 2 {
				t.Fatalf("batch used stale rules or settled changed candidates: %+v %+v %v", st, out, approveErr)
			}
			for _, result := range out[0].Results {
				if !strings.Contains(result.Error, "rules") || result.Action == core.ActionAdded {
					t.Fatalf("batch used stale rules or settled changed candidates: %+v", out)
				}
			}
		})
	}
}

type approvalRulesFS struct {
	core.FS
	before func()
}

func (f *approvalRulesFS) Identify(path string) (core.Identity, error) {
	if f.before != nil {
		before := f.before
		f.before = nil
		before()
	}
	return f.FS.Identify(path)
}

func TestApprovalRulesSnapshot(t *testing.T) {
	r := newEngineFixture(t)
	rule := r.write(rules.FileName, "*.log 30d\n")
	if _, err := r.engine.Trust.Trust(rule, ""); err != nil {
		t.Fatal(err)
	}
	if err := r.engine.checkApprovalRules(r.root); err != nil {
		t.Fatal(err)
	}
	r.write(rules.FileName, "!blocked.log\n")
	if _, err := r.engine.Trust.Trust(rule, ""); err != nil {
		t.Fatal(err)
	}
	if err := r.engine.checkApprovalRules(r.root); err == nil {
		t.Fatal("changed approval rule snapshot accepted")
	}
	r.engine.sets = nil
	if err := r.engine.checkApprovalRules(r.root); err != nil {
		t.Fatal(err)
	}
	if _, err := r.engine.Trust.Untrust(rule); err != nil {
		t.Fatal(err)
	}
	if err := r.engine.checkApprovalRules(r.root); err == nil {
		t.Fatal("revoked approval rule snapshot accepted")
	}
}

func TestApproveRechecksRulesAtCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository")
	}
	r := newRepo(t)
	path := r.write("blocked.log", "keep")
	p, _, _, err := r.engine.Svc.Propose(core.ProposalLeftovers, r.root, "s", []core.Candidate{{Path: path, Lifespan: "30d"}})
	if err != nil {
		t.Fatal(err)
	}
	r.engine.Svc.FS = &interleaveAddFS{beforeAdd: func() {
		rule := r.write(rules.FileName, "!blocked.log\n")
		if _, err := r.engine.Trust.Trust(rule, ""); err != nil {
			t.Fatal(err)
		}
	}}
	out, approveErr := r.engine.Approve([]string{fmt.Sprint(p.ID)})
	st, err := r.engine.Svc.Store.Read()
	if err != nil || approveErr == nil || len(st.Items) != 0 || len(st.Proposals) != 1 || len(st.Proposals[0].Candidates) != 1 {
		t.Fatalf("commit used stale approval rules: %+v %+v %v %v", st, out, approveErr, err)
	}
}

func TestApproveShortProjectKeepsCanonicalRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	r := newRepo(t)
	path := r.write("debug.log", "pending")
	short := shortPath(t, r.root)
	p, _, _, err := r.engine.Svc.Propose(core.ProposalLeftovers, short, "s", []core.Candidate{{Path: filepath.Join(short, "debug.log"), Lifespan: "1d"}})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := r.engine.Approve([]string{fmt.Sprint(p.ID)})
	if err != nil || len(rows) != 1 || len(rows[0].Results) != 1 || core.Failed(rows[0].Results) {
		t.Fatalf("short project refused approval: %+v %v", rows, err)
	}
	st, err := r.engine.Svc.Store.ReadSnapshot()
	if err != nil || len(st.Items) != 1 || st.Items[0].Path != path || st.Items[0].Project != r.root || len(st.Proposals) != 0 {
		t.Fatalf("approval retained project alias: %+v %v", st, err)
	}
}

func TestApproveLongSession(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprint(legacy), func(t *testing.T) {
			t.Parallel()
			r := newRepo(t)
			path := r.write("debug.log", "pending")
			long := strings.Repeat("s", 4097)
			_, _, err := r.engine.Observe([]string{path}, long)
			if err != nil {
				t.Fatal(err)
			}
			st, err := r.engine.Svc.Store.Read()
			if err != nil || len(st.Proposals) != 1 {
				t.Fatalf("observe: %+v %v", st, err)
			}
			if legacy {
				if err := r.engine.Svc.Store.Update(func(st *core.State) error { st.Proposals[0].Session = long; return nil }); err != nil {
					t.Fatal(err)
				}
			}
			got, err := r.engine.Approve([]string{fmt.Sprint(st.Proposals[0].ID)})
			if err != nil || len(got) != 1 || got[0].Error != "" || core.Failed(got[0].Results) {
				t.Fatalf("long session blocked approval: %+v %v", got, err)
			}
			st, err = r.engine.Svc.Store.Read()
			if err != nil || len(st.Items) != 1 || len(st.Proposals) != 0 || len(st.Items[0].Session) > 4096 || !strings.HasPrefix(st.Items[0].Session, "sha256:") {
				t.Fatalf("long session registration: %+v %v", st, err)
			}
		})
	}
}

func TestRefreshProposalsRetainsUnavailableWorktreeMetadata(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, missing := range []int{1, 2} {
		t.Run(fmt.Sprint(missing), func(t *testing.T) {
			t.Parallel()
			r := newRepo(t)
			r.write("tracked.txt", "tracked")
			r.commit("tracked.txt")
			worktree := filepath.Join(t.TempDir(), "linked")
			r.git("worktree", "add", "--detach", worktree)
			var candidates []core.Candidate
			for _, name := range []string{"a.log", "b.log"} {
				path := filepath.Join(worktree, name)
				if err := os.WriteFile(path, []byte("pending"), 0o600); err != nil {
					t.Fatal(err)
				}
				candidates = append(candidates, core.Candidate{Path: path, Lifespan: "1d"})
			}
			if _, _, _, err := r.engine.Svc.Propose(core.ProposalLeftovers, worktree, "s", candidates); err != nil {
				t.Fatal(err)
			}
			before, err := r.engine.Svc.Store.Read()
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range candidates[:missing] {
				if err := os.Remove(c.Path); err != nil {
					t.Fatal(err)
				}
			}
			metadata := filepath.Join(r.root, ".git")
			if err := os.Rename(metadata, metadata+"-offline"); err != nil {
				t.Fatal(err)
			}
			refreshErr := r.engine.RefreshProposals()
			if err := os.Rename(metadata+"-offline", metadata); err != nil {
				t.Fatal(err)
			}
			after, err := r.engine.Svc.Store.Read()
			if err != nil || refreshErr == nil || !reflect.DeepEqual(before.Proposals, after.Proposals) {
				t.Fatalf("unavailable worktree metadata pruned proposals: %+v %v %v", after.Proposals, refreshErr, err)
			}
		})
	}
}

func TestRefreshProposalsRetainsUnavailableProject(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, mode := range []string{"renamed", "not-directory", "missing-git"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			r := newRepo(t)
			path := r.write("candidate.log", "pending")
			if _, _, _, err := r.engine.Svc.Propose(core.ProposalLeftovers, r.root, "s", []core.Candidate{{Path: path, Lifespan: "3d"}}); err != nil {
				t.Fatal(err)
			}
			before, err := r.engine.Svc.Store.Read()
			if err != nil {
				t.Fatal(err)
			}
			from, to := r.root, r.root+"-offline"
			if mode == "missing-git" {
				from, to = filepath.Join(r.root, ".git"), filepath.Join(r.root, ".git-offline")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Rename(from, to); err != nil {
				t.Fatal(err)
			}
			if mode == "not-directory" {
				if err := os.WriteFile(from, []byte("temporarily occupied"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			refreshErr := r.engine.RefreshProposals()
			if mode == "not-directory" {
				if err := os.Remove(from); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.Rename(to, from); err != nil {
				t.Fatal(err)
			}
			after, err := r.engine.Svc.Store.Read()
			if err != nil || refreshErr == nil || !strings.Contains(refreshErr.Error(), "proposal project") || !reflect.DeepEqual(after.Proposals, before.Proposals) {
				t.Fatalf("unavailable project lost pending proposals: before=%+v after=%+v refresh=%v read=%v", before.Proposals, after.Proposals, refreshErr, err)
			}
		})
	}
}

func TestRefreshProposalsBatchesProjectsAndRetriesGitFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	r := newRepo(t)
	a, b := r.write("a.txt", "a"), r.write("b.txt", "b")
	for i, path := range []string{a, b} {
		if _, _, _, err := r.engine.Svc.Propose(core.ProposalLeftovers, r.root, fmt.Sprint(i), []core.Candidate{{Path: path, Lifespan: "1d"}}); err != nil {
			t.Fatal(err)
		}
	}
	r.commit("a.txt", "b.txt")
	trace := filepath.Join(t.TempDir(), "git-trace.log")
	t.Setenv("GIT_TRACE", filepath.ToSlash(trace))
	oldPath := os.Getenv("PATH")
	t.Setenv("PATH", t.TempDir())
	if err := r.engine.RefreshProposals(); err == nil {
		t.Fatal("Git failure was not reported")
	}
	st, err := r.engine.Svc.Store.Read()
	if err != nil || len(st.Proposals) != 2 {
		t.Fatalf("Git failure withdrew uncertain proposals: %+v %v", st.Proposals, err)
	}
	t.Setenv("PATH", oldPath)
	if err := r.engine.RefreshProposals(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(trace)
	if err != nil || strings.Count(string(data), "built-in: git ls-files") != 1 {
		t.Fatalf("proposal refresh did not batch project query: %s %v", data, err)
	}
	st, err = r.engine.Svc.Store.Read()
	if err != nil || len(st.Proposals) != 0 || len(st.LeftoverRejections) != 0 {
		t.Fatalf("tracked proposals not withdrawn: %+v %v", st.Proposals, err)
	}
}

func TestApprovalVetsCandidatesAgain(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	log := r.write("debug.log", "x")
	if _, _, err := r.engine.Observe([]string{log}, "s1"); err != nil {
		t.Fatal(err)
	}
	r.commit("debug.log") // committed after the proposal was made
	st, _ := r.engine.Svc.Store.Read()
	results, err := r.engine.Approve([]string{fmt.Sprint(st.Proposals[0].ID)})
	if err != nil || len(results) != 1 || len(results[0].Results) != 1 || results[0].Results[0].Error == "" {
		t.Fatalf("a candidate tracked since must be skipped, got %+v %v", results, err)
	}
	if st, _ := r.engine.Svc.Store.Read(); len(st.Items) != 0 {
		t.Fatal("nothing may be registered")
	}
	if again, _ := r.engine.Svc.Reject([]string{fmt.Sprint(st.Proposals[0].ID)}); again[0].Error == "" {
		t.Fatal("a claimed proposal cannot also be rejected")
	}
}

func TestProposalClaimPreservesPendingAndConcurrentCandidates(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	first, second, third := r.write("first.log", "1"), r.write("second.log", "2"), r.write("third.log", "3")
	p, _, _, err := r.engine.Svc.Propose(core.ProposalLeftovers, r.root, "session", []core.Candidate{
		{Path: first, Lifespan: "1d"}, {Path: second, Lifespan: "1d"},
	})
	if err != nil {
		t.Fatal(err)
	}
	ref := fmt.Sprint(p.ID)
	claimed, release, err := r.engine.Svc.ClaimProposal(ref)
	if err != nil {
		t.Fatal(err)
	}
	func() {
		defer release()
		st, err := r.engine.Svc.Store.Read()
		if err != nil || len(st.Proposals) != 1 || len(st.Proposals[0].Candidates) != 2 {
			t.Fatalf("claim must leave candidates durable: %+v, %v", st.Proposals, err)
		}
		if got, err := r.engine.Approve([]string{ref}); err == nil && (len(got) != 1 || got[0].Error == "") {
			t.Fatalf("concurrent approve must not claim again: %+v, %v", got, err)
		}
		if got, err := r.engine.Svc.Reject([]string{ref}); err == nil {
			t.Fatalf("reject must not overtake an approval: %+v", got)
		}
		if _, _, _, err := r.engine.Svc.Propose(p.Kind, p.Project, p.Session, []core.Candidate{{Path: third, Lifespan: "1d"}}); err != nil {
			t.Fatal(err)
		}
		results, err := r.engine.Svc.Add([]string{first}, core.AddOptions{ExpiresAt: time.Now().Add(24 * time.Hour)})
		if err != nil || core.Failed(results) {
			t.Fatalf("add: %+v, %v", results, err)
		}
		if err := r.engine.Svc.ResolveProposal(claimed, claimed.Candidates[1:]); err != nil {
			t.Fatal(err)
		}
	}()
	st, err := r.engine.Svc.Store.Read()
	if err != nil || len(st.Proposals) != 1 || st.Proposals[0].ID != p.ID || len(st.Proposals[0].Candidates) != 2 || st.Proposals[0].Candidates[0].Path != second || st.Proposals[0].Candidates[1].Path != third {
		t.Fatalf("pending and concurrently added candidates must retain their ID: %+v, %v", st.Proposals, err)
	}
	got, err := r.engine.Approve([]string{ref})
	if err != nil || len(got) != 1 || core.Failed(got[0].Results) {
		t.Fatalf("approval after release: %+v, %v", got, err)
	}
}

func TestApproveRetainsGuardIOFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	path := r.write("keep.txt", "keep")
	p, _, _, err := r.engine.Svc.Propose(core.ProposalLeftovers, r.root, "s", []core.Candidate{{Path: path, Lifespan: "3d"}})
	if err != nil {
		t.Fatal(err)
	}
	r.engine.Svc.Guard.Canonical = func(string) (string, error) { return "", errors.New("temporary I/O failure") }
	got, err := r.engine.Approve([]string{fmt.Sprint(p.ID)})
	if len(got) != 1 || !core.Failed(got[0].Results) {
		t.Fatalf("expected guard failure: %+v, %v", got, err)
	}
	st, err := r.engine.Svc.Store.Read()
	if err != nil || len(st.Items) != 0 || len(st.Proposals) != 1 || len(st.Proposals[0].Candidates) != 1 || st.Proposals[0].ID != p.ID {
		t.Fatalf("retryable candidate lost: %+v, %v", st, err)
	}
}

func TestApproveReplacementOfGoneObject(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	path := r.write("debug.log", "old")
	old, err := r.engine.Svc.Add([]string{path}, core.AddOptions{ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil || core.Failed(old) {
		t.Fatalf("add: %+v, %v", old, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	r.write("debug.log", "new")
	p, _, _, err := r.engine.Svc.Propose(core.ProposalLeftovers, r.root, "s", []core.Candidate{{Path: path, Lifespan: "3d"}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.engine.Approve([]string{fmt.Sprint(p.ID)})
	if err != nil || len(got) != 1 || len(got[0].Results) != 1 || core.Failed(got[0].Results) {
		t.Fatalf("gone object must not refuse replacement: %+v, %v", got, err)
	}
	st, err := r.engine.Svc.Store.Read()
	if err != nil || len(st.Items) != 1 || st.Items[0].ID != old[0].Item.ID || *st.Items[0].Identity == *old[0].Item.Identity || len(st.Proposals) != 0 {
		t.Fatalf("replacement was not registered: %+v, %v", st, err)
	}
}

func TestReproposedClaimedPathRemainsPending(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	now := time.Now()
	r.engine.Svc.Now = func() time.Time { return now }
	path := r.write("debug.log", "x")
	p, _, _, err := r.engine.Svc.Propose(core.ProposalLeftovers, r.root, "s", []core.Candidate{{Path: path, Lifespan: "1d"}})
	if err != nil {
		t.Fatal(err)
	}
	claimed, release, err := r.engine.Svc.ClaimProposal(fmt.Sprint(p.ID))
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	done := make(chan error, 1)
	go func() {
		_, _, _, err := r.engine.Svc.Propose(p.Kind, p.Project, p.Session, []core.Candidate{{Path: strings.ToUpper(path), Lifespan: "2d", Size: 42}})
		done <- err
	}()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := r.engine.Svc.ResolveProposal(claimed, nil); err != nil {
		t.Fatal(err)
	}
	st, err := r.engine.Svc.Store.Read()
	if err != nil || len(st.Proposals) != 1 || len(st.Proposals[0].Candidates) != 1 {
		t.Fatalf("same-path reproposal lost: %+v, %v", st.Proposals, err)
	}
	c := st.Proposals[0].Candidates[0]
	if c.Lifespan != "2d" || c.Size != 42 {
		t.Fatalf("new candidate metadata lost: %+v", c)
	}
}

func TestApproveSettlesDefinitiveAddFailures(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, kind := range []string{"guard", "hardlinks", "missing", "not-found", "transient"} {
		t.Run(kind, func(t *testing.T) {
			t.Parallel()
			r := newRepo(t)
			path := r.write("debug.log", "keep")
			p, _, _, err := r.engine.Svc.Propose(core.ProposalLeftovers, r.root, "s", []core.Candidate{{Path: path, Lifespan: "1d"}})
			if err != nil {
				t.Fatal(err)
			}
			fs := &approvalFS{}
			r.engine.Svc.FS = fs
			switch kind {
			case "guard":
				r.engine.Svc.Guard.Keep = []string{path}
			case "hardlinks":
				fs.first = func() {
					if err := os.Link(path, path+".link"); err != nil {
						t.Fatal(err)
					}
				}
			case "missing":
				fs.first = func() {
					if err := os.Rename(path, path+".saved"); err != nil {
						t.Fatal(err)
					}
				}
			case "not-found":
				fs.err = core.ErrNotFound
			case "transient":
				fs.err = errors.New("temporarily busy")
			}
			got, err := r.engine.Approve([]string{fmt.Sprint(p.ID)})
			if len(got) != 1 || len(got[0].Results) != 1 || !core.Failed(got[0].Results) {
				t.Fatalf("expected Add failure: %+v, %v", got, err)
			}
			st, err := r.engine.Svc.Store.Read()
			want := 0
			if kind == "transient" {
				want = 1
			}
			if err != nil || len(st.Items) != 0 || len(st.Proposals) != want {
				t.Fatalf("settled candidate must go, retryable must stay: %+v, %v", st, err)
			}
			if want == 1 && len(st.Proposals[0].Candidates) != 1 {
				t.Fatal("retryable candidate lost")
			}
		})
	}
}
