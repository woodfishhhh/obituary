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

func TestContainerOwnerUnattributed(t *testing.T) {
	root, reason := containerOwner(docker.Container{}, t.TempDir())
	if root != "" || reason != "会话期间新建" {
		t.Fatal(root, reason)
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
	for i, name := range []string{"compose", "bind", "unattributed", "old"} {
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
		rows = append(rows, row)
	}
	testenv.WriteDocker(t, state, rows...)
	if _, err := r.engine.Finish(r.root, "session"); err != nil {
		t.Fatal(err)
	}
	st, err := r.engine.Svc.Store.Read()
	if err != nil || len(st.Proposals) != 2 || len(st.Items) != 0 {
		t.Fatalf("proposals=%+v err=%v", st, err)
	}
	for _, p := range st.Proposals {
		for _, c := range p.Candidates {
			if c.Container.Name == "unattributed" && (p.Project != "" || c.Reason != "会话期间新建") {
				t.Fatal(p)
			}
			if c.Container.Name != "unattributed" && p.Project != r.root {
				t.Fatal(p)
			}
		}
		if p.Project == "" {
			results, err := r.engine.Svc.Reject([]string{fmt.Sprint(p.ID)})
			if err != nil || results[0].Error != "" {
				t.Fatal(results, err)
			}
		} else {
			results, err := r.engine.Approve([]string{fmt.Sprint(p.ID)})
			if err != nil || len(results) != 1 || core.Failed(results[0].Results) {
				t.Fatal(results, err)
			}
		}
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
