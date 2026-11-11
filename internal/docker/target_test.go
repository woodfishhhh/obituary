package docker_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/docker"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestDockerTargetChangesRefused(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	for _, change := range []string{"daemon", "endpoint"} {
		t.Run(change, func(t *testing.T) {
			cli, row, state, log := dockerFixture(t)
			got, err := cli.Inspect(t.Context(), row.ID)
			if err != nil {
				t.Fatal(err)
			}
			testenv.WriteDocker(t, state)
			if change == "daemon" {
				t.Setenv("OBIT_FAKE_DOCKER_DAEMON", "replacement")
			} else {
				// The same named context now routes to a different socket.
				got.Target.Context = "saved"
				t.Setenv("OBIT_FAKE_DOCKER_CONTEXT_HOST", "tcp://replacement:2375")
			}
			_, err = cli.Inspect(docker.WithTarget(t.Context(), got.Target), got.ID)
			if !errors.Is(err, docker.ErrTarget) || errors.Is(err, docker.ErrNotFound) {
				t.Fatal("target change became absence", err)
			}
			if err := cli.Stop(t.Context(), got.Identity, nil); !errors.Is(err, docker.ErrTarget) {
				t.Fatal(err)
			}
			for _, raw := range testenv.DockerCalls(t, log) {
				if testenv.DockerCommand(raw)[0] == "stop" {
					t.Fatal("changed daemon stopped", raw)
				}
			}
		})
	}
}

func TestDockerHostAndCurrentContextPinned(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	for _, selection := range []string{"host", "current"} {
		t.Run(selection, func(t *testing.T) {
			cli, row, state, log := dockerFixture(t)
			if selection == "host" {
				t.Setenv("DOCKER_HOST", "npipe:////./pipe/fake-docker")
			}
			got, err := cli.Inspect(t.Context(), row.ID)
			if err != nil {
				t.Fatal(err)
			}
			testenv.WriteDocker(t, state+".other", row)
			t.Setenv("DOCKER_HOST", "tcp://other:2375")
			t.Setenv("OBIT_FAKE_DOCKER_CURRENT", "other")
			if err := cli.Stop(t.Context(), got.Identity, nil); err != nil {
				t.Fatal(err)
			}
			for _, raw := range testenv.DockerCalls(t, log) {
				if testenv.DockerCommand(raw)[0] == "stop" && (raw[0] != "--host" && raw[0] != "--context") {
					t.Fatal("target not explicit", raw)
				}
			}
		})
	}
}

func TestIdentityTargetComparison(t *testing.T) {
	id := docker.Identity{ID: strings.Repeat("a", 64), Target: &docker.Target{Endpoint: "a", DaemonID: "one"}}
	other := id
	other.Target = &docker.Target{Endpoint: "b", DaemonID: "one"}
	if id.Same(other) {
		t.Fatal("endpoint ignored")
	}
	other.Target = &docker.Target{Endpoint: "a", DaemonID: "two"}
	if id.Same(other) {
		t.Fatal("daemon ignored")
	}
}
