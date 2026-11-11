package docker_test

import (
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/cli/clitest"
	dockercmd "github.com/woodfishhhh/obituary/internal/cli/docker"
	listcmd "github.com/woodfishhhh/obituary/internal/cli/list"
	"github.com/woodfishhhh/obituary/internal/docker"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestDockerHelp(t *testing.T) {
	out, _, err := clitest.Execute(t, dockercmd.NewCmd(), "--help")
	for _, text := range []string{"retention_days", "recycle", "volumes", "add", "list"} {
		if err != nil || !strings.Contains(out, text) {
			t.Fatalf("missing %s: %s %v", text, out, err)
		}
	}
}

func TestDockerAddAndList(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	clitest.IsolateRunner(t)
	clitest.CommandDir(t)
	_, state, log := testenv.FakeDocker(t)
	row := docker.Container{Identity: docker.Identity{ID: strings.Repeat("a", 64), Name: "cache", Created: time.Now().Add(-time.Hour)}, Running: true}
	testenv.WriteDocker(t, state, row)
	out, _, err := clitest.Execute(t, dockercmd.NewCmd(), "add", "cache", "2d", "--json")
	if err != nil || !strings.Contains(out, `"kind": "container"`) || !strings.Contains(out, "docker:cache") {
		t.Fatal(out, err)
	}
	before := len(testenv.DockerCalls(t, log))
	for _, cmd := range []bool{false, true} {
		command := dockercmd.NewCmd()
		args := []string{"list", "--json"}
		if cmd {
			command = listcmd.NewCmd()
			args = []string{"--json"}
		}
		out, _, err = clitest.Execute(t, command, args...)
		if err != nil || !strings.Contains(out, "aaaaaaaaaaaa") || !strings.Contains(out, `"container"`) {
			t.Fatal(out, err)
		}
	}
	if len(testenv.DockerCalls(t, log)) != before {
		t.Fatal("listing invoked Docker")
	}
}
