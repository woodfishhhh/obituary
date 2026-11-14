package auto

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestPOSIXObservationChecksKnownTrackedPaths(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX volume-wide lookup is unavailable")
	}
	if testing.Short() {
		t.Skip("automatic observations query a real Git repository")
	}
	r := newRepo(t)
	approveAutomatic(t, r)
	first, second := r.write("first.log", "first"), r.write("second.log", "second")
	rows, _, err := r.engine.Observe([]string{first, second}, "native")
	st, readErr := r.engine.Svc.Store.ReadSnapshot()
	if err != nil || readErr != nil || len(rows) != 2 || rows[0].Action != Registered || rows[1].Action != Registered || len(st.Items) != 2 {
		t.Fatalf("known tracked file blocked another observation: %+v %v %v", rows, err, readErr)
	}
	if err := os.Rename(first, first+".moved"); err != nil {
		t.Fatal(err)
	}
	third := r.write("third.log", "third")
	rows, _, _ = r.engine.Observe([]string{third}, "native")
	after, err := r.engine.Svc.Store.ReadSnapshot()
	if err != nil || len(rows) != 1 || rows[0].Action != Retry || rows[0].Err == nil || len(after.Items) != 2 {
		t.Fatalf("unknown location was treated as disappearance: %+v %v", rows, err)
	}
}

func TestObserveUnreadableLedgerReturnsEveryPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "store.json"), []byte("{broken"), 0o600); err != nil {
		t.Fatal(err)
	}
	e := &Engine{Svc: &core.Service{Store: core.Store{Home: dir}, ProposalMode: core.ProposalsTiered}}
	ds, notices, err := e.observe([]string{"first", "second"}, Options{Session: "s"}, locations{})
	if err == nil || len(ds) != 2 || ds[0].Path != "first" || ds[1].Path != "second" || ds[0].Err == nil || ds[1].Err == nil || len(notices) != 0 {
		t.Fatalf("unreadable ledger lost per-path failures: %+v %+v %v", ds, notices, err)
	}
}

func TestObserveMergesBatchNotices(t *testing.T) {
	if testing.Short() {
		t.Skip("automatic registration uses a real Git repository")
	}
	r := newRepo(t)
	paths := []string{r.write("first.log", "keep"), r.write("second.log", "keep")}
	ds, notices, err := r.engine.Observe(paths, "s")
	if err != nil || len(ds) != 2 || ds[0].Action != Proposed || ds[1].Action != Proposed || len(notices) != 1 || !notices[0].Created || len(notices[0].Proposal.Candidates) != 2 {
		t.Fatalf("merged proposal lost batch candidates or creation notice: %+v %+v %v", ds, notices, err)
	}
	for i, path := range paths {
		if ds[i].Path != path || notices[0].Proposal.Candidates[i].Path != path {
			t.Fatalf("observation order changed: %+v %+v", ds, notices)
		}
	}
}

func TestObserveCapturedSkipsCompletedRequest(t *testing.T) {
	if testing.Short() {
		t.Skip("automatic registration uses a real Git repository")
	}
	for _, approved := range []bool{false, true} {
		t.Run(fmt.Sprint(approved), func(t *testing.T) {
			r := newRepo(t)
			want := Proposed
			if approved {
				if err := r.engine.Svc.Store.Update(func(st *core.State) error {
					st.Projects = map[string]core.ProjectState{core.ProjectKey(r.root): {Consent: core.ConsentApproved}}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				want = Registered
			}
			path := r.write("debug.log", "artifact")
			rows, err := r.engine.Svc.CaptureObservations([]string{path}, "s")
			if err != nil {
				t.Fatal(err)
			}
			ds, _, err := r.engine.ObserveCaptured(rows)
			if err != nil || len(ds) != 1 || ds[0].Action != want {
				t.Fatal(ds, err)
			}
			ds, notices, err := r.engine.ObserveCaptured(rows)
			if err != nil || len(ds) != 1 || ds[0].Path != path || ds[0].Action != Skipped || len(notices) != 0 {
				t.Fatalf("completed request was handled again: %+v %+v %v", ds, notices, err)
			}
		})
	}
}

func TestInvalidProjectRulesPauseAutomaticRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("automatic registration uses a real Git repository")
	}
	for _, content := range []string{"[broken 2h\n", "*.log 2h\n"} {
		t.Run(content, func(t *testing.T) {
			r := newRepo(t)
			rulesPath := r.write(".obitrules", "*.log 1d\n")
			if _, err := r.engine.Trust.Trust(rulesPath, ""); err != nil {
				t.Fatal(err)
			}
			approveAutomatic(t, r)
			if err := r.engine.Snapshot(r.root, "paused"); err != nil {
				t.Fatal(err)
			}
			r.write(".obitrules", content)
			path := r.write("malformed-auto.log", "keep")
			_, _, _ = r.engine.Observe([]string{path}, "paused")
			r.write("notes.txt", "also keep")
			_, _ = r.engine.Leftovers(r.root, "paused")
			st, err := r.engine.Svc.Store.Read()
			if err != nil || len(st.Items) != 0 || len(st.Proposals) != 0 {
				t.Fatalf("invalid or changed rules did not pause automatic registration: %+v %v", st, err)
			}
			r.write(".obitrules", "*.log 2h\n")
			if _, err := r.engine.Trust.Trust(rulesPath, ""); err != nil {
				t.Fatal(err)
			}
			ds, _, err := r.engine.Observe([]string{path}, "paused")
			if err != nil || len(ds) != 1 || ds[0].Action != Registered {
				t.Fatalf("trusted repair did not resume registration: %+v %v", ds, err)
			}
		})
	}
}

func approveAutomatic(t *testing.T, r *repo) {
	t.Helper()
	if err := r.engine.Svc.Store.Update(func(st *core.State) error {
		st.Projects = map[string]core.ProjectState{core.ProjectKey(r.root): {Consent: core.ConsentApproved}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRemovedTrustedRulesPauseObservationAndExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip("automatic rules use a real Git repository")
	}
	r := newRepo(t)
	rulesPath := r.write(".obitrules", "*.log off\nother.txt 1s\n")
	if _, err := r.engine.Trust.Trust(rulesPath, ""); err != nil {
		t.Fatal(err)
	}
	approveAutomatic(t, r)
	path := r.write("protected.log", "keep")
	due := r.write("other.txt", "expire only with rules available")
	if _, _, err := r.engine.Observe([]string{path, due}, "s"); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(rulesPath); err != nil {
		t.Fatal(err)
	}
	_, _, _ = r.engine.Observe([]string{path}, "s")
	r.engine.Svc.Now = func() time.Time { return time.Now().Add(time.Minute) }
	out, err := r.engine.Svc.Sweep(false)
	st, readErr := r.engine.Svc.Store.Read()
	if err != nil || readErr != nil || len(st.Items) != 1 || len(out) != 1 || out[0].Action != core.ActionRetry {
		t.Fatalf("missing trusted protection permitted automatic work: %+v %+v %v %v", st.Items, out, err, readErr)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(due); err != nil {
		t.Fatal(err)
	}
	if was, err := r.engine.Trust.Untrust(rulesPath); err != nil || !was {
		t.Fatalf("untrust: %v %v", was, err)
	}
	ds, _, err := r.engine.Observe([]string{path}, "s")
	if err != nil || len(ds) != 1 || ds[0].Action != Registered {
		t.Fatalf("explicit untrust did not resume: %+v %v", ds, err)
	}
}

func TestKeptObjectsStayOutOfSessionRegistration(t *testing.T) {
	if testing.Short() {
		t.Skip("session diff and file identities use a real Git repository")
	}
	for _, action := range []string{"spare", "resurrect"} {
		t.Run(action, func(t *testing.T) {
			r := newRepo(t)
			approveAutomatic(t, r)
			if err := r.engine.Snapshot(r.root, "keep"); err != nil {
				t.Fatal(err)
			}
			path := r.write("restored.log", "keep")
			rows, err := r.engine.Svc.Add([]string{path}, core.AddOptions{ExpiresAt: time.Now().Add(time.Hour)})
			if err != nil || core.Failed(rows) {
				t.Fatalf("add: %+v %v", rows, err)
			}
			keepObject(t, r, path, action)
			moved := filepath.Join(r.root, "renamed.log")
			if err := os.Rename(path, moved); err != nil {
				t.Fatal(err)
			}
			// A new engine models the next hook process reading persisted consent.
			r.engine = &Engine{Svc: r.engine.Svc, Trust: r.engine.Trust}
			if _, err := r.engine.Leftovers(r.root, "keep"); err != nil {
				t.Fatal(err)
			}
			st, err := r.engine.Svc.Store.Read()
			if err != nil || len(st.Items) != 0 || len(st.Proposals) != 0 {
				t.Fatalf("kept object was automatically registered again: %+v %v", st, err)
			}
			r.write("renamed.log", "new write after the keep decision")
			if _, err := r.engine.Leftovers(r.root, "keep"); err != nil {
				t.Fatal(err)
			}
			st, err = r.engine.Svc.Store.Read()
			if err != nil || len(st.Items) != 1 {
				t.Fatalf("changed object did not resume automatic registration: %+v %v", st, err)
			}
		})
	}
}

func keepObject(t *testing.T, r *repo, path, action string) {
	t.Helper()
	if action == "spare" {
		rows, err := r.engine.Svc.Cancel([]string{path})
		if err != nil || core.Failed(rows) {
			t.Fatalf("spare: %+v %v", rows, err)
		}
		return
	}
	buried, err := r.engine.Svc.Bury([]string{path}, false, "")
	if err != nil || len(buried) != 1 || buried[0].Action != core.ActionBuried {
		t.Fatalf("bury: %+v %v", buried, err)
	}
	restored, err := r.engine.Svc.Resurrect([]string{path})
	if err != nil || len(restored) != 1 || restored[0].Action != core.ActionResurrected {
		t.Fatalf("resurrect: %+v %v", restored, err)
	}
}

func TestKeptUnmatchedObjectDoesNotBecomeLeftoverProposal(t *testing.T) {
	if testing.Short() {
		t.Skip("session diff uses a real Git repository")
	}
	r := newRepo(t)
	if err := r.engine.Snapshot(r.root, "keep"); err != nil {
		t.Fatal(err)
	}
	path := r.write("notes.txt", "keep")
	rows, err := r.engine.Svc.Add([]string{path}, core.AddOptions{ExpiresAt: time.Now().Add(time.Hour)})
	if err != nil || core.Failed(rows) {
		t.Fatalf("add: %+v %v", rows, err)
	}
	keepObject(t, r, path, "spare")
	if _, err := r.engine.Leftovers(r.root, "keep"); err != nil {
		t.Fatal(err)
	}
	st, err := r.engine.Svc.Store.Read()
	if err != nil || len(st.Items) != 0 || len(st.Proposals) != 0 {
		t.Fatalf("kept unmatched object was proposed again: %+v %v", st, err)
	}
}

func TestObserveRejectsNamedStreams(t *testing.T) {
	testenv.WindowsOnly(t, "NTFS named streams share the host file identity")
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	for _, extended := range []bool{false, true} {
		t.Run(fmt.Sprint(extended), func(t *testing.T) {
			t.Parallel()
			r := newRepo(t)
			path := filepath.Join(r.root, "debug.log:review")
			if extended {
				path = `\\?\` + path
			}
			if err := os.WriteFile(path, []byte("stream"), 0600); err != nil {
				t.Fatal(err)
			}
			ds, notices, _ := r.engine.Observe([]string{path}, "session")
			st, err := r.engine.Svc.Store.ReadSnapshot()
			if err != nil || len(ds) != 1 || !strings.Contains(ds[0].Reason, "whole files or directories") || len(notices) != 0 || len(st.Items) != 0 || len(st.Proposals) != 0 {
				t.Fatalf("stream observation selected whole file: %+v %+v %v", ds, notices, err)
			}
		})
	}
}

func TestLeftoversDoNotMistakeExpandedDirectoriesForNewFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	r.write("README.md", "x")
	r.commit("README.md")
	r.write("scratch/old.log", "from before the session")
	if err := r.engine.Snapshot(r.root, "s1"); err != nil {
		t.Fatal(err)
	}
	r.write("scratch/tracked.txt", "x")
	r.commit("scratch/tracked.txt") // Git now lists scratch/old.log on its own
	if _, err := r.engine.Leftovers(r.root, "s1"); err != nil {
		t.Fatal(err)
	}
	st, _ := r.engine.Svc.Store.Read()
	if len(st.Items) != 0 || len(st.Proposals) != 0 {
		t.Fatalf("scratch/old.log predates the session, got items=%+v proposals=%+v", st.Items, st.Proposals)
	}
}

// Dependencies, build output and caches are in use however Git treats them,
// and uncommitted files are work: a session's leftovers are never proposed.
func TestLeftoversProposeNothing(t *testing.T) {
	if testing.Short() {
		t.Skip("session leftovers use a real Git repository")
	}
	t.Parallel()
	r := newRepo(t)
	r.ask() // exercises the proposal itself, as "ask" mode keeps it
	r.write(".gitignore", "node_modules/\ndist/\n.env\n*.o\n")
	r.write("src/main.go", "package main")
	r.commit(".gitignore", "src/main.go")
	if err := r.engine.Snapshot(r.root, "s"); err != nil {
		t.Fatal(err)
	}
	r.write("src/feature.go", "package main // uncommitted work")
	r.write("newpkg/a.go", "package newpkg")
	r.write(".env", "TOKEN=secret")
	r.write("build/x.o", "Git lists build/ as ignored although only x.o is")
	r.write("node_modules/left-pad/index.js", "x")
	r.write("dist/app.js", "x")
	r.write("newpkg/node_modules/m/index.js", "x")
	r.write("node_modules/.cache/build.log", "a rule would match this log")
	if _, err := r.engine.Leftovers(r.root, "s"); err != nil {
		t.Fatal(err)
	}
	st, err := r.engine.Svc.Store.Read()
	if err != nil || len(st.Proposals) != 0 || len(st.Items) != 0 {
		t.Fatalf("leftovers were handled: %+v %v", st, err)
	}
}

func TestLeftoversDeduplicateSessions(t *testing.T) {
	if testing.Short() {
		t.Skip("session leftovers use a real Git repository")
	}
	for _, decision := range []string{"approve", "reject"} {
		t.Run(decision, func(t *testing.T) {
			r := newRepo(t)
			r.ask() // exercises the proposal itself, as "ask" mode keeps it
			for _, session := range []string{"first", "second"} {
				if err := r.engine.Snapshot(r.root, session); err != nil {
					t.Fatal(err)
				}
			}
			r.write("test-results/index.js", "generated")
			path := filepath.Join(r.root, "test-results")
			for _, session := range []string{"first", "second"} {
				if _, err := r.engine.Leftovers(r.root, session); err != nil {
					t.Fatal(err)
				}
			}
			st, err := r.engine.Svc.Store.Read()
			if err != nil || len(st.Proposals) != 1 || len(st.Proposals[0].Candidates) != 1 || st.Proposals[0].Candidates[0].Path != path {
				t.Fatalf("same leftover has multiple pending proposals: %+v %v", st.Proposals, err)
			}
			if len(st.Items) != 0 || len(st.Burials) != 0 {
				t.Fatalf("unapproved leftover changed registration: %+v", st)
			}
			refs := []string{fmt.Sprint(st.Proposals[0].ID)}
			var out []core.ProposalResult
			if decision == "approve" {
				out, err = r.engine.Approve(refs)
			} else {
				out, err = r.engine.Svc.Reject(refs)
			}
			if err != nil || len(out) != 1 || out[0].Error != "" || core.Failed(out[0].Results) {
				t.Fatalf("decision failed: %+v %v", out, err)
			}
			for _, session := range []string{"first", "second"} {
				if _, err := r.engine.Leftovers(r.root, session); err != nil {
					t.Fatal(err)
				}
			}
			st, err = r.engine.Svc.Store.Read()
			wantItems := 0
			if decision == "approve" {
				wantItems = 1
			}
			if err != nil || len(st.Proposals) != 0 || len(st.Items) != wantItems || len(st.Burials) != 0 {
				t.Fatalf("decided leftover was proposed again: %+v %v", st, err)
			}
			if data, err := os.ReadFile(filepath.Join(path, "index.js")); err != nil || string(data) != "generated" {
				t.Fatalf("leftover content changed: %q %v", data, err)
			}
		})
	}
}

func TestObserveReturnsEachPathOutcome(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	t.Parallel()
	r := newRepo(t)
	bad, good, source := r.write("bad.log", "retry"), r.write("good.log", "register"), r.write("notes.txt", "keep")
	if err := r.engine.Svc.Store.Update(func(st *core.State) error {
		st.Projects = map[string]core.ProjectState{core.ProjectKey(r.root): {Consent: core.ConsentApproved}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	r.engine.Svc.FS = &unreadableObservationFS{FS: r.engine.Svc.FS, path: bad}
	ds, notices, err := r.engine.Observe([]string{bad, good, source}, "s")
	if err == nil || len(ds) != 3 || len(notices) != 0 || ds[0].Action != Retry || ds[0].Err == nil || ds[1].Action != Registered || ds[2].Action != Skipped || ds[2].Err != nil {
		t.Fatalf("missing per-path outcomes: %+v, %+v, %v", ds, notices, err)
	}
}

func TestShortPathSessionSharesSnapshotAndProposals(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	r := newRepo(t)
	r.ask() // exercises the proposal itself, as "ask" mode keeps it
	root, err := filepath.EvalSymlinks(r.root)
	if err != nil {
		t.Fatal(err)
	}
	r.root = root
	short := shortPath(t, root)
	for _, roots := range [][2]string{{short, root}, {root, short}} {
		session := roots[0]
		if err := r.engine.Snapshot(roots[0], session); err != nil {
			t.Fatal(err)
		}
		r.write("test-results/report.html", "x")
		path := filepath.Join(r.root, "test-results")
		notices, err := r.engine.Leftovers(roots[1], session)
		if err != nil || len(notices) != 1 || notices[0].Proposal.Project != root || len(notices[0].Proposal.Candidates) != 1 || notices[0].Proposal.Candidates[0].Path != path {
			t.Fatalf("aliases split snapshot or proposal: %+v %v", notices, err)
		}
		if err := os.RemoveAll(path); err != nil {
			t.Fatal(err)
		}
		if err := r.engine.RefreshProposals(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestObserveVetsRegistrationAndProposals(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	for _, route := range []string{"approved", "large", "onboard", "declined"} {
		for _, gitState := range []string{"untracked", "tracked", "tracked-child", "broken-index"} {
			t.Run(route+"/"+gitState, func(t *testing.T) {
				r := newRepo(t)
				path := r.write("debug.log", "keep")
				consent := core.ConsentApproved
				switch route {
				case "onboard":
					consent = ""
				case "declined":
					consent = core.ConsentDeclined
				}
				if err := r.engine.Svc.Store.Update(func(st *core.State) error {
					st.Projects = map[string]core.ProjectState{core.ProjectKey(r.root): {Consent: consent}}
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				if route == "large" {
					previous := LargeBytes
					LargeBytes = 0
					t.Cleanup(func() { LargeBytes = previous })
				}
				switch gitState {
				case "tracked":
					r.git("add", "--", "debug.log")
				case "tracked-child":
					path = r.write("test-results/result.txt", "keep")
					r.git("add", "--", "test-results/result.txt")
				case "broken-index":
					r.write(".git/index", "not a Git index")
				}
				ds, notices, err := r.engine.Observe([]string{path}, "s")
				st, readErr := r.engine.Svc.Store.ReadSnapshot()
				if readErr != nil || len(ds) != 1 {
					t.Fatalf("observation result: %+v, %v", ds, readErr)
				}
				if gitState != "untracked" {
					want := Skipped
					if gitState == "broken-index" {
						want = Retry
					}
					if ds[0].Action != want || (err != nil) != (want == Retry) || len(st.Items) != 0 || len(st.Proposals) != 0 || len(notices) != 0 {
						t.Fatalf("Git vetting bypassed: %+v, %+v, %v", ds, st, err)
					}
				} else {
					want, items, proposals := Proposed, 0, 1
					switch route {
					case "approved":
						want, items, proposals = Registered, 1, 0
					case "declined":
						want, proposals = Skipped, 0
					}
					if err != nil || ds[0].Action != want || len(st.Items) != items || len(st.Proposals) != proposals || len(notices) != proposals {
						t.Fatalf("consent or size gate changed: %+v, %+v, %v", ds, st, err)
					}
				}
				if data, err := os.ReadFile(path); err != nil || string(data) != "keep" {
					t.Fatalf("observation changed content: %q, %v", data, err)
				}
			})
		}
	}
}

func TestObserveApprovedQueriesLiveGitOnce(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	r := newRepo(t)
	path := r.write("debug.log", "keep")
	seed := func() {
		t.Helper()
		if err := r.engine.Svc.Store.Update(func(st *core.State) error {
			*st = core.State{NextID: 1, Projects: map[string]core.ProjectState{core.ProjectKey(r.root): {Consent: core.ConsentApproved}}}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	seed()
	trace := filepath.Join(t.TempDir(), "git-trace.jsonl")
	t.Setenv("GIT_TRACE2_EVENT", trace)
	ds, _, err := r.engine.Observe([]string{path}, "first")
	if err != nil || len(ds) != 1 || ds[0].Action != Registered {
		t.Fatalf("initial untracked observation: %+v, %v", ds, err)
	}
	data, err := os.ReadFile(trace)
	if err != nil {
		t.Fatal(err)
	}
	queries := 0
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var event struct {
			Event string   `json:"event"`
			Argv  []string `json:"argv"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		if event.Event == "start" {
			for _, arg := range event.Argv {
				if arg == "ls-files" {
					queries++
				}
			}
		}
	}
	if queries != 1 {
		t.Fatalf("approved observation queried Git %d times, want one live query", queries)
	}
	// Reusing the same engine must never reuse the previous untracked answer.
	seed()
	r.git("add", "--", "debug.log")
	ds, notices, err := r.engine.Observe([]string{path}, "second")
	st, readErr := r.engine.Svc.Store.ReadSnapshot()
	if err != nil || readErr != nil || len(ds) != 1 || ds[0].Action != Skipped || !strings.Contains(ds[0].Reason, "Git tracks") || len(st.Items) != 0 || len(st.Proposals) != 0 || len(notices) != 0 {
		t.Fatalf("observation reused stale Git answer: %+v, %+v, %v, %v", ds, st, err, readErr)
	}
}

func TestPOSIXObserveUsesGitAndRequiresConsent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX integration")
	}
	if testing.Short() {
		t.Skip("creates a real Git repository")
	}
	r := newRepo(t)
	tracked := r.write("tracked.log", "keep")
	free := r.write("debug.log", "scratch")
	r.git("add", "--", "tracked.log")
	decisions, _, err := r.engine.Observe([]string{tracked, free}, "posix-session")
	if err != nil || len(decisions) != 2 {
		t.Fatal(decisions, err)
	}
	state, err := r.engine.Svc.Store.ReadSnapshot()
	if err != nil || len(state.Items) != 0 || len(state.Proposals) != 1 {
		t.Fatal(state, err)
	}
	if len(state.Proposals[0].Candidates) != 1 || state.Proposals[0].Candidates[0].Path != free {
		t.Fatalf("tracked file proposed: %+v", state.Proposals)
	}
}
