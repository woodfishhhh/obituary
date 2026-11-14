package auto

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/docker"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func (r *repo) state() core.State {
	r.t.Helper()
	st, err := r.engine.Svc.Store.Read()
	if err != nil {
		r.t.Fatal(err)
	}
	return st
}

func (r *repo) observe(paths ...string) map[string]Decision {
	r.t.Helper()
	decisions, _, err := r.engine.Observe(paths, "s")
	if err != nil {
		r.t.Fatal(err)
	}
	out := map[string]Decision{}
	for _, d := range decisions {
		out[d.Path] = d
	}
	return out
}

func (r *repo) snapshot(session string) {
	r.t.Helper()
	if err := r.engine.Snapshot(r.root, session); err != nil {
		r.t.Fatal(err)
	}
}

// receipts maps each registered path to the class of its receipt.
func receipts(st core.State) map[string]string {
	out := map[string]string{}
	for _, h := range st.AutoHandled() {
		out[filepath.Base(h.Item.Path)] = h.Class
	}
	return out
}

func candidateNames(st core.State) []string {
	var names []string
	for _, p := range st.Proposals {
		for _, c := range p.Candidates {
			names = append(names, p.Kind+":"+filepath.Base(c.Path))
		}
	}
	return names
}

func TestTieredRegistersWhatTheAgentCreated(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git repository")
	}
	t.Parallel()
	r := newRepo(t)
	r.snapshot("s")
	log := r.write("debug.log", "x")
	shot := r.write("test-results/a.png", "x")
	got := r.observe(log, shot)
	st := r.state()
	if len(st.Proposals) != 0 || len(st.Items) != 2 {
		t.Fatalf("created files were not registered directly: %+v %+v", got, st)
	}
	if want := (map[string]string{"debug.log": core.AutoCreated, "test-results": core.AutoCreated}); fmt.Sprint(receipts(st)) != fmt.Sprint(want) {
		t.Fatalf("receipts = %v, want %v", receipts(st), want)
	}
	if consent := st.Projects[core.ProjectKey(r.root)].Consent; consent != "" {
		t.Fatalf("direct registration recorded consent %q", consent)
	}
	for _, it := range st.Items {
		if it.Source != "rule" || !it.Automatic() {
			t.Fatalf("created item lost its automatic source: %+v", it)
		}
	}
}

// What the hook saw written but existed when the session started is the
// user's: tiering must keep proposing it, however a rule matches it.
func TestTieredProposesWhatPredatesTheSession(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git repository")
	}
	t.Parallel()
	r := newRepo(t)
	old := r.write("old.log", "user's")
	r.write("logs/keep.txt", "user's folder")
	r.write("test-results/report.html", "user's results")
	r.snapshot("s")
	r.write("old.log", "appended by the agent")
	inLogs := r.write("logs/new.log", "x")
	inResults := r.write("test-results/new.png", "x")
	r.observe(old, inLogs, inResults)
	st := r.state()
	if len(st.Items) != 0 || len(st.Proposals) != 1 || st.Proposals[0].Kind != core.ProposalOnboard || len(st.Proposals[0].Candidates) != 3 {
		t.Fatalf("pre-existing units must stay proposals: %+v", st)
	}
}

func TestTieredNeedsTheSessionsOwnSnapshot(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git repository")
	}
	t.Parallel()
	for _, tc := range []string{"none", "other session", "legacy"} {
		t.Run(tc, func(t *testing.T) {
			r := newRepo(t)
			switch tc {
			case "other session":
				r.snapshot("other")
			case "legacy":
				// Snapshots written before tiering lack the Git marker.
				r.snapshot("s")
				path := r.engine.snapshotPath(r.root, "s")
				data, err := json.Marshal(snapshot{Project: r.root, Session: "s", At: time.Now().Add(-time.Hour)})
				if err != nil || os.WriteFile(path, data, 0o600) != nil {
					t.Fatal(err)
				}
			}
			r.observe(r.write("debug.log", "x"))
			if st := r.state(); len(st.Items) != 0 || len(st.Proposals) != 1 {
				t.Fatalf("unproven creation must be proposed: %+v", st)
			}
		})
	}
}

// A folder moved or renamed into a rule's name keeps its creation time; a
// path snapshot alone would take it for new. It stays a proposal.
func TestTieredRenamedFolderIsNotCreated(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git repository; waits out the creation-time slack")
	}
	t.Parallel()
	r := newRepo(t)
	r.write("drafts/thesis.md", "the user's work")
	time.Sleep(creationSlack + 500*time.Millisecond)
	r.snapshot("s")
	if err := os.Rename(filepath.Join(r.root, "drafts"), filepath.Join(r.root, "test-results")); err != nil {
		t.Fatal(err)
	}
	r.observe(r.write("test-results/new.png", "x"))
	st := r.state()
	if len(st.Items) != 0 || len(st.Proposals) != 1 {
		t.Fatalf("renamed user folder was registered without asking: %+v", st)
	}
	if _, ok := birthTime(filepath.Join(r.root, "test-results", "thesis.md")); !ok {
		t.Skip("file system reports no creation times") // nothing counts as created here anyway
	}
}

// New paths found only by comparing the project with the snapshot are not
// proven to be the agent's writes: tiered mode proposes them, auto does not.
func TestSessionDifferenceIsNotEvidenceOfCreation(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git repository")
	}
	t.Parallel()
	for mode, want := range map[string]string{core.ProposalsTiered: "proposal", core.ProposalsAuto: core.AutoAll} {
		t.Run(mode, func(t *testing.T) {
			r := newRepo(t)
			r.engine.Svc.ProposalMode = mode
			r.snapshot("s")
			r.write("x.log", "written by some process")
			if _, err := r.engine.Leftovers(r.root, "s"); err != nil {
				t.Fatal(err)
			}
			st := r.state()
			got := "proposal"
			if len(st.Items) == 1 {
				got = receipts(st)["x.log"]
			}
			if got != want || len(st.Items)+len(candidateNames(st)) != 1 {
				t.Fatalf("mode %s: got %s %+v, want %s", mode, got, st, want)
			}
		})
	}
}

func TestProposalModesForAgentFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git repository")
	}
	t.Parallel()
	for _, tc := range []struct {
		mode      string
		old, made string // "" means proposed
	}{
		{core.ProposalsAsk, "", ""},
		{core.ProposalsTiered, "", core.AutoCreated},
		{core.ProposalsAuto, core.AutoAll, core.AutoCreated},
	} {
		t.Run(tc.mode, func(t *testing.T) {
			r := newRepo(t)
			r.engine.Svc.ProposalMode = tc.mode
			old := r.write("old.log", "x")
			r.snapshot("s")
			r.observe(old, r.write("new.log", "x"))
			st := r.state()
			got := receipts(st)
			if got["old.log"] != tc.old || got["new.log"] != tc.made {
				t.Fatalf("receipts %v, proposals %v", got, candidateNames(st))
			}
			if st.Projects[core.ProjectKey(r.root)].Consent != "" {
				t.Fatal("a mode must never record the user's consent")
			}
		})
	}
}

// No mode registers a Git-tracked file, whoever wrote it.
func TestTrackedFilesAreNeverRegisteredInAnyMode(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git repository")
	}
	t.Parallel()
	for _, mode := range []string{core.ProposalsAsk, core.ProposalsTiered, core.ProposalsAuto} {
		t.Run(mode, func(t *testing.T) {
			r := newRepo(t)
			r.engine.Svc.ProposalMode = mode
			r.snapshot("s")
			log := r.write("app.log", "tracked")
			r.commit("app.log")
			r.write("app.log", "changed by the agent")
			d := r.observe(log)
			st := r.state()
			if len(st.Items) != 0 || len(st.Proposals) != 0 || !strings.Contains(fmt.Sprint(d), "Git tracks it") {
				t.Fatalf("tracked file handled in %s mode: %v %+v", mode, d, st)
			}
		})
	}
}

// A project the user declined stays declined in every mode: no direct
// registration, and leftovers wait for the user as before.
func TestDeclinedProjectIsNeverRegisteredWithoutAsking(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git repository")
	}
	t.Parallel()
	r := newRepo(t)
	r.ask()
	r.write(".gitignore", "dist/\n")
	r.commit(".gitignore")
	old := r.write("old.log", "x")
	r.observe(old)
	st := r.state()
	if len(st.Proposals) != 1 {
		t.Fatalf("want an onboard proposal: %+v", st)
	}
	if _, err := r.engine.Svc.Reject([]string{fmt.Sprint(st.Proposals[0].ID)}); err != nil {
		t.Fatal(err)
	}
	r.engine.Svc.ProposalMode = core.ProposalsAuto
	r.snapshot("s")
	r.observe(r.write("new.log", "x"))
	r.write("dist/app.js", "x")
	if _, err := r.engine.Leftovers(r.root, "s"); err != nil {
		t.Fatal(err)
	}
	if err := r.engine.RefreshProposals(); err != nil {
		t.Fatal(err)
	}
	st = r.state()
	if len(st.Items) != 0 || fmt.Sprint(candidateNames(st)) != "[leftovers:dist]" {
		t.Fatalf("declined project registered without asking: items %+v proposals %v", st.Items, candidateNames(st))
	}
}

// Proposals pending from before tiering, or from "ask" mode, are sorted again
// by the background tick: what the mode lets in is registered with a receipt,
// the rest stays for the user, and no consent is recorded on their behalf.
func TestPendingProposalsAreSortedAgainByMode(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git repository")
	}
	t.Parallel()
	r := newRepo(t)
	r.ask()
	r.write(".gitignore", "dist/\n")
	r.commit(".gitignore")
	dist := r.write("dist/app.js", "x")
	feature := r.write("src/feature.go", "package main // uncommitted work")
	// Leftovers proposals made before 1.1.1 could list uncommitted files.
	if _, _, _, err := r.engine.Svc.Propose(core.ProposalLeftovers, r.root, "old", []core.Candidate{
		{Path: filepath.Dir(dist), Lifespan: LeftoverLifespan}, {Path: feature, Lifespan: LeftoverLifespan},
	}); err != nil {
		t.Fatal(err)
	}
	r.observe(r.write("old.log", "x")) // an onboard proposal
	for _, step := range []struct {
		mode    string
		handled string
		pending string
	}{
		{core.ProposalsAsk, "map[]", "[leftovers:dist leftovers:feature.go onboard:old.log]"},
		{core.ProposalsTiered, "map[dist:regenerable]", "[leftovers:feature.go onboard:old.log]"},
		{core.ProposalsAuto, "map[dist:regenerable old.log:auto]", "[leftovers:feature.go]"},
	} {
		r.engine.Svc.ProposalMode = step.mode
		if err := r.engine.RefreshProposals(); err != nil {
			t.Fatal(err)
		}
		st := r.state()
		if fmt.Sprint(receipts(st)) != step.handled || fmt.Sprint(candidateNames(st)) != step.pending {
			t.Fatalf("%s: handled %v pending %v", step.mode, receipts(st), candidateNames(st))
		}
		if st.Projects[core.ProjectKey(r.root)].Consent != "" {
			t.Fatalf("%s: settling proposals recorded consent", step.mode)
		}
	}
}

func TestSessionContainersAreRegisteredAndSparingSticks(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git repository and fake Docker executable")
	}
	r := newRepo(t)
	executable, state, _ := testenv.FakeDocker(t)
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	r.engine.Svc.Now = func() time.Time { return now }
	r.engine.Svc.Docker = docker.CLI{Executable: executable}
	r.snapshot("session")
	now = now.Add(time.Hour)
	testenv.WriteDocker(t, state, docker.Container{Identity: docker.Identity{ID: strings.Repeat("1", 64), Name: "web", Created: now.Add(-time.Minute)},
		Labels: map[string]string{"com.docker.compose.project.working_dir": r.root}, Running: true})
	if _, err := r.engine.Finish(r.root, "session"); err != nil {
		t.Fatal(err)
	}
	st := r.state()
	if len(st.Items) != 1 || len(st.Proposals) != 0 || receipts(st)[filepath.Base(st.Items[0].Path)] != core.AutoContainer {
		t.Fatalf("session container not registered directly: %+v", st)
	}
	if _, err := r.engine.Svc.Cancel([]string{fmt.Sprint(st.Items[0].ID)}); err != nil {
		t.Fatal(err)
	}
	if _, err := r.engine.Finish(r.root, "session"); err != nil {
		t.Fatal(err)
	}
	if st := r.state(); len(st.Items) != 0 || len(st.Proposals) != 0 || len(st.AutoReceipts) != 0 {
		t.Fatalf("spared container came back: %+v", st)
	}
}

// The mode decision itself needs no Git: a fast check of every branch.
func TestDirectClassFollowsTheMode(t *testing.T) {
	r := newEngineFixture(t)
	unit := r.write("dist/app.js", "x")
	regen := map[string]bool{strings.ToLower(filepath.Join(r.root, "dist")): true}
	for _, tc := range []struct {
		mode string
		opts Options
		want string
	}{
		{core.ProposalsAsk, Options{Written: true, regenerable: regen}, ""},
		{core.ProposalsTiered, Options{Written: true}, ""}, // no session snapshot: not proven created
		{core.ProposalsTiered, Options{regenerable: regen}, core.AutoRegenerable},
		{core.ProposalsAuto, Options{}, core.AutoAll},
		{"", Options{}, ""}, // unset means tiered
	} {
		r.engine.Svc.ProposalMode = tc.mode
		if got := r.engine.directClass(r.root, unit, tc.opts); got != tc.want {
			t.Errorf("mode %q %+v: got %q, want %q", tc.mode, tc.opts, got, tc.want)
		}
	}
	if within(map[string]bool{}, unit) || !within(regen, unit) || within(regen, r.root) {
		t.Fatal("within must match the entry and what lies below it only")
	}
}
