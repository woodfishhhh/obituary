package auto

import (
	"fmt"
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
	elsewhere := docker.Container{Labels: map[string]string{"com.docker.compose.project.working_dir": t.TempDir()}, Binds: []string{root + "-sibling"}}
	for _, row := range []docker.Container{{}, elsewhere} {
		if reason, ok := projectContainer(row, root); ok {
			t.Fatal(row, reason)
		}
	}
}

func TestContainerSessionProposals(t *testing.T) {
	if testing.Short() {
		t.Skip("real Git repository and fake Docker executable")
	}
	r := newRepo(t)
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
			row.Labels = map[string]string{"com.docker.compose.project.working_dir": t.TempDir()}
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
