package auto

import (
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

func TestProjectContainerNeedsProjectEvidence(t *testing.T) {
	root := t.TempDir()
	for _, row := range []docker.Container{{}, {Binds: []string{root}}, {Binds: []string{"relative"}}, {Labels: map[string]string{"com.docker.compose.project.working_dir": "."}}} {
		if owner, reason, err := projectContainer(row); owner != "" || err != nil {
			t.Fatal(row, owner, reason, err)
		}
	}
}

func TestProjectContainerUsesComposeDirectory(t *testing.T) {
	root := t.TempDir()
	row := docker.Container{Labels: map[string]string{"com.docker.compose.project.working_dir": root}, Binds: []string{t.TempDir()}}
	owner, reason, err := projectContainer(row)
	if err != nil || owner != root || reason != "compose working_dir" {
		t.Fatal(owner, reason, err)
	}
}

func TestProjectContainerFindsUniqueBindProject(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git repositories")
	}
	r, other := newRepo(t), newRepo(t)
	for _, tc := range []struct {
		name  string
		binds []string
		want  string
	}{
		{"nested bind", []string{filepath.Join(r.root, "data")}, r.root},
		{"same project twice", []string{filepath.Join(r.root, "data"), r.root}, r.root},
		{"no Git", []string{t.TempDir()}, ""},
		{"multiple projects", []string{r.root, other.root}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, _, err := projectContainer(docker.Container{Binds: tc.binds})
			if err != nil || got != tc.want {
				t.Fatal(got, err)
			}
		})
	}
}

func TestContainerSessionProposals(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git repository and fake Docker executable")
	}
	r := newRepo(t)
	r.ask() // exercises the proposal itself, as "ask" mode keeps it
	executable, state, _ := testenv.FakeDocker(t)
	now := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	r.engine.Svc.Now = func() time.Time { return now }
	r.engine.Svc.Docker = docker.CLI{Executable: executable}
	if err := r.engine.Snapshot(r.root, "session"); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Hour)
	var rows []docker.Container
	for i, name := range []string{"compose", "bind", "unattributed", "old", "elsewhere"} {
		row := docker.Container{Identity: docker.Identity{ID: strings.Repeat(fmt.Sprint(i+1), 64), Name: name, Created: now.Add(-time.Minute)}, Running: true}
		if i == 0 {
			row.Labels = map[string]string{"com.docker.compose.project.working_dir": r.root}
		}
		if i == 1 {
			row.Binds = []string{filepath.Join(r.root, "data")}
		}
		if i == 3 {
			row.Created = now.Add(-2 * time.Hour)
		}
		if i == 4 {
			row.Binds = []string{t.TempDir()}
		}
		rows = append(rows, row)
	}
	testenv.WriteDocker(t, state, rows...)
	if _, err := r.engine.Finish(r.root, "session"); err != nil {
		t.Fatal(err)
	}
	st, err := r.engine.Svc.Store.Read()
	// Only containers tied to the project are proposed, so approval is always
	// the recommended answer; unattributed ones may be the user's own.
	if err != nil || len(st.Proposals) != 1 || len(st.Items) != 0 || st.Proposals[0].Project != r.root || len(st.Proposals[0].Candidates) != 2 {
		t.Fatalf("proposals=%+v err=%v", st, err)
	}
	for _, c := range st.Proposals[0].Candidates {
		if c.Container.Name != "compose" && c.Container.Name != "bind" {
			t.Fatal(c)
		}
	}
	results, err := r.engine.Approve([]string{fmt.Sprint(st.Proposals[0].ID)})
	if err != nil || len(results) != 1 || core.Failed(results[0].Results) {
		t.Fatal(results, err)
	}
	if _, err := r.engine.Finish(r.root, "session"); err != nil {
		t.Fatal(err)
	}
	st, err = r.engine.Svc.Store.Read()
	if err != nil || len(st.Items) != 2 || len(st.Proposals) != 0 {
		t.Fatalf("decisions replayed: %+v %v", st, err)
	}
}

func TestContainerSessionsDoNotDuplicateOrOwnProjects(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git repository and fake Docker executable")
	}
	r := newRepo(t)
	r.ask()
	executable, state, _ := testenv.FakeDocker(t)
	r.engine.Svc.Docker = docker.CLI{Executable: executable}
	now := time.Now()
	r.engine.Svc.Now = func() time.Time { return now }
	sessionRoot := t.TempDir()
	for _, session := range []string{"first", "second"} {
		if err := r.engine.SnapshotSession(sessionRoot, session); err != nil {
			t.Fatal(err)
		}
	}
	now = now.Add(time.Hour)
	row := docker.Container{Identity: docker.Identity{ID: strings.Repeat("a", 64), Created: now.Add(-time.Minute), Name: "cache"}, Binds: []string{r.root}}
	testenv.WriteDocker(t, state, row)
	for _, session := range []string{"first", "second"} {
		if _, err := r.engine.ContainerLeftovers(sessionRoot, session); err != nil {
			t.Fatal(err)
		}
	}
	st, err := r.engine.Svc.Store.Read()
	if err != nil || len(st.Proposals) != 1 || len(st.Proposals[0].Candidates) != 1 || st.Proposals[0].Project != r.root {
		t.Fatalf("state=%+v err=%v", st, err)
	}
}

func TestApproveContainerUnavailableKeepsProposalAndLocalizes(t *testing.T) {
	r := newEngineFixture(t)
	r.ask()
	id := docker.Identity{ID: strings.Repeat("b", 64), Name: "cache", Created: time.Now()}
	p, _, _, err := r.engine.Svc.Propose(core.ProposalContainer, r.root, "session", []core.Candidate{{Kind: core.KindContainer, Container: &id, Path: id.Display(), Lifespan: "1d"}})
	if err != nil {
		t.Fatal(err)
	}
	configDir := t.TempDir()
	t.Setenv("OBIT_CONFIG_DIR", configDir)
	for _, tc := range []struct{ language, want string }{{"zh-CN", "需要先启动 Docker"}, {"en", "Start Docker before"}} {
		if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("[ui]\nlanguage='"+tc.language+"'\n"), 0600); err != nil {
			t.Fatal(err)
		}
		rows, err := r.engine.Approve([]string{fmt.Sprint(p.ID)})
		if err == nil || !strings.Contains(err.Error(), tc.want) || len(rows) != 1 || !strings.Contains(rows[0].Results[0].Error, tc.want) {
			t.Fatal(rows, err)
		}
	}
	st, err := r.engine.Svc.Store.Read()
	if err != nil || len(st.Proposals) != 1 || len(st.Items) != 0 {
		t.Fatal(st, err)
	}
}

func TestContainerDiscoveryUnavailable(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git repository and fake Docker executable")
	}
	r := newRepo(t)
	executable, _, _ := testenv.FakeDocker(t)
	r.engine.Svc.Docker = docker.CLI{Executable: executable, Timeout: 50 * time.Millisecond}
	if err := r.engine.Snapshot(r.root, "session"); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"fail", "sleep"} {
		t.Setenv("OBIT_FAKE_DOCKER_MODE", mode)
		if rows, err := r.engine.ContainerLeftovers(r.root, "session"); err != nil || len(rows) != 0 {
			t.Fatal(rows, err)
		}
	}
}
