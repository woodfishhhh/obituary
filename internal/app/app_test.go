//go:build windows

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

func TestTickWithdrawsDeletedProject(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository")
	}
	root := t.TempDir()
	project := filepath.Join(root, "worktree")
	testenv.InitRepo(t, project)
	path := filepath.Join(project, "candidate.log")
	if err := os.WriteFile(path, []byte("pending"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := &core.Service{Store: core.Store{Home: filepath.Join(root, "home")}, FS: winfs.FS{}}
	p, _, _, err := svc.Propose(core.ProposalLeftovers, project, "s", []core.Candidate{{Path: path, Lifespan: "1d"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(project); err != nil {
		t.Fatal(err)
	}
	out, purged, err := Tick(svc)
	if err != nil || len(out) != 0 || len(purged) != 0 {
		t.Fatalf("deleted project tick: %+v %+v %v", out, purged, err)
	}
	st, err := svc.Store.Read()
	if err != nil || len(st.Proposals) != 0 || len(st.Items) != 0 || len(st.Burials) != 0 || len(st.Projects) != 0 || len(st.LeftoverRejections) != 0 {
		t.Fatalf("withdrawal became a decision or file operation: %+v %v", st, err)
	}
	events, err := svc.Store.History(20)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range events {
		if e.Action == "proposal-withdrawn" && e.ID == p.ID && e.Path == p.Project && strings.Contains(e.Detail, "project deleted") {
			return
		}
	}
	t.Fatalf("missing deleted-project audit: %+v", events)
}

func TestTickAuditsUnrelatedIdentityFailure(t *testing.T) {
	root := t.TempDir()
	svc := &core.Service{Store: core.Store{Home: filepath.Join(root, "data")}, FS: winfs.FS{}}
	path := filepath.Join(root, "keep.txt")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	rows, err := svc.Add([]string{path}, core.AddOptions{ExpiresAt: time.Now().Add(24 * time.Hour)})
	if err != nil || core.Failed(rows) {
		t.Fatalf("add: %+v %v", rows, err)
	}
	if err := os.Link(path, path+".alias"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Tick(svc); err != nil {
		t.Fatal(err)
	}
	history, err := svc.Store.History(10)
	if err != nil || len(history) != 2 || history[1].Action != "identity-error" || history[1].ID != rows[0].Item.ID || !strings.Contains(history[1].Detail, "hard links") {
		t.Fatalf("background tick swallowed identity failure: %+v %v", history, err)
	}
	st, err := svc.Store.Read()
	if err != nil || len(st.Items) != 1 || st.Items[0].Rev != rows[0].Item.Rev {
		t.Fatalf("background diagnosis changed a lifespan: %+v %v", st.Items, err)
	}
}

func TestTickReconcilesWatchBeforeSweeping(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "downloads")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	svc := &core.Service{Store: core.Store{Home: filepath.Join(root, "data")}, FS: winfs.FS{},
		Owner: auto.Owner, OwnerBatch: auto.OwnerBatch, PrepareOwners: auto.PrepareOwners}
	if _, err := svc.SetWatch(dir, "1h", core.WatchFirstSeen); err != nil {
		t.Fatal(err)
	}
	state, err := svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	approved, err := (&auto.Engine{Svc: svc}).Approve([]string{fmt.Sprint(state.Proposals[0].ID)})
	if err != nil || approved[0].Error != "" {
		t.Fatalf("approve: %+v %v", approved, err)
	}
	path := filepath.Join(dir, "offline.txt")
	if err := os.WriteFile(path, []byte("arrived while stopped"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Tick(svc); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Store.Read()
	if err != nil || len(st.Items) != 1 {
		t.Fatalf("tick missed offline arrival: %+v %v", st, err)
	}
	expiry := st.Items[0].ExpiresAt
	svc.Now = func() time.Time { return expiry.Add(time.Second) }
	outcomes, _, err := Tick(svc)
	if err != nil || len(outcomes) != 1 || outcomes[0].Action != core.ActionBuried {
		t.Fatalf("tick did not bury expired child: %+v %v", outcomes, err)
	}
}

func TestTickUsesEditedWatchStableWindow(t *testing.T) {
	if testing.Short() {
		t.Skip("folder policy evidence uses a real Git repository")
	}
	root := t.TempDir()
	project := filepath.Join(root, "project")
	if err := os.Mkdir(project, 0700); err != nil {
		t.Fatal(err)
	}
	testenv.InitRepo(t, project)
	t.Setenv("OBIT_CONFIG_DIR", root)
	cfg := filepath.Join(root, "config.toml")
	if err := os.WriteFile(cfg, []byte("[watch]\nstable_window='25m'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(project, "downloads")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	svc := &core.Service{Store: core.Store{Home: filepath.Join(root, "home")}, FS: winfs.FS{},
		Owner: auto.Owner, OwnerBatch: auto.OwnerBatch, PrepareOwners: auto.PrepareOwners}
	if _, err := svc.SetWatch(dir, "1s", core.WatchFirstSeen); err != nil {
		t.Fatal(err)
	}
	pending, err := svc.Store.Read()
	if err != nil || len(pending.Proposals) != 1 {
		t.Fatalf("policy proposal: %+v %v", pending.Proposals, err)
	}
	if err := svc.ApproveWatch(pending.Proposals[0]); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(path, []byte("payload"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Tick(svc); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Store.Read()
	if err != nil || len(st.Items) != 1 {
		t.Fatalf("register: %+v %v", st.Items, err)
	}
	now := st.Items[0].CreatedAt.Add(11 * time.Minute)
	svc.Now = func() time.Time { return now }
	out, _, err := Tick(svc)
	if err != nil || len(out) != 1 || out[0].Action != core.ActionRetry {
		t.Fatalf("tick ignored configured stable window: %+v %v", out, err)
	}
	if err := os.WriteFile(cfg, []byte("[watch]\nstable_window='10m'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	out, _, err = Tick(svc)
	if err != nil || len(out) != 1 || out[0].Action != core.ActionBuried || svc.WatchStableWindow != 0 {
		t.Fatalf("tick missed edited watch window or mutated shared service: %+v %v", out, err)
	}
}

func TestTickUsesEditedRetentionWithoutMutatingSharedService(t *testing.T) {
	root := t.TempDir()
	cfgDir := filepath.Join(root, "config")
	t.Setenv("OBIT_CONFIG_DIR", cfgDir)
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(cfgDir, "config.toml")
	if err := os.WriteFile(configPath, []byte("[graveyard]\nretention_days = 3\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	svc := &core.Service{Store: core.Store{Home: filepath.Join(root, "home")}, FS: winfs.FS{}, Retention: 99 * 24 * time.Hour}
	path := filepath.Join(root, "file")
	if err := os.WriteFile(path, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(time.Hour)
	if _, err := svc.Add([]string{path}, core.AddOptions{ExpiresAt: expires}); err != nil {
		t.Fatal(err)
	}
	svc.Now = func() time.Time { return expires.Add(time.Second) }
	out, _, err := Tick(svc)
	if err != nil || len(out) != 1 || out[0].Action != core.ActionBuried {
		t.Fatalf("tick: %+v %v", out, err)
	}
	st, err := svc.Store.Inspect()
	if err != nil || len(st.Burials) != 1 || st.Burials[0].PurgeAt.Sub(st.Burials[0].BuriedAt) != 3*24*time.Hour {
		t.Fatalf("retention: %+v %v", st.Burials, err)
	}
	if svc.Retention != 99*24*time.Hour {
		t.Fatal("mutated shared service")
	}
	if err := os.WriteFile(configPath, []byte("[graveyard]\nmode = 'invalid'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Tick(svc); err == nil {
		t.Fatal("malformed config ignored by resident")
	}
}

func TestTickPrunesAndWithdrawsStaleProposals(t *testing.T) {
	if testing.Short() {
		t.Skip("requires a real Git repository or child process")
	}
	// Tick rereads config.toml; "ask" keeps the leftovers a proposal to prune.
	configDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("[proposals]\nmode = \"ask\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OBIT_CONFIG_DIR", configDir)
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		if out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
	}
	git("init", "-q")
	svc := &core.Service{Store: core.Store{Home: t.TempDir()}, FS: winfs.FS{}, ProposalMode: core.ProposalsAsk}
	engine := Engine(svc)
	if err := engine.Snapshot(root, "s"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"dist", "coverage", "node_modules"} {
		testenv.Leftover(t, root, name)
	}
	if _, err := engine.Leftovers(root, "s"); err != nil {
		t.Fatal(err)
	}
	st, err := svc.Store.Read()
	if err != nil || len(st.Proposals) != 1 || len(st.Proposals[0].Candidates) != 3 {
		t.Fatalf("leftovers fixture: %+v %v", st.Proposals, err)
	}
	id := st.Proposals[0].ID
	git("add", "-f", "--", "dist/index.js")
	git("commit", "-qm", "track proposal candidate")
	if err := os.RemoveAll(filepath.Join(root, "coverage")); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Tick(svc); err != nil {
		t.Fatal(err)
	}
	st, err = svc.Store.Read()
	if err != nil || len(st.Proposals) != 1 || st.Proposals[0].ID != id || len(st.Proposals[0].Candidates) != 1 || filepath.Base(st.Proposals[0].Candidates[0].Path) != "node_modules" {
		t.Fatalf("background tick retained stale proposal candidates: %+v %v", st.Proposals, err)
	}
	git("add", "-f", "--", "node_modules/index.js")
	git("commit", "-qm", "track last candidate")
	if _, _, err := Tick(svc); err != nil {
		t.Fatal(err)
	}
	st, err = svc.Store.Read()
	if err != nil || len(st.Proposals) != 0 || len(st.LeftoverRejections) != 0 || len(st.Projects) != 0 || len(st.Items) != 0 {
		t.Fatalf("automatic withdrawal became a user decision: %+v %v", st, err)
	}
	events, err := svc.Store.History(20)
	pruned, withdrawn := 0, 0
	for _, event := range events {
		if event.ID != id {
			continue
		}
		if event.Action == "proposal-pruned" {
			pruned++
		}
		if event.Action == "proposal-withdrawn" {
			withdrawn++
		}
	}
	if err != nil || pruned != 3 || withdrawn != 1 {
		t.Fatalf("proposal pruning/withdrawal audit missing: %+v %v", events, err)
	}
}
