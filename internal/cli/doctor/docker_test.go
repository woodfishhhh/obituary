package doctor

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/docker"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestDockerUnavailableIsInformational(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	row := diagnoseDocker()
	if row.Status != "INFO" || !strings.Contains(row.Detail, "skipped") {
		t.Fatal(row)
	}
}

func TestDockerDoctorPolicy(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	_, state, _ := testenv.FakeDocker(t)
	testenv.WriteDocker(t, state)
	row := diagnoseDocker()
	if row.Status != "PASS" || !strings.Contains(row.Detail, "recycle") || !strings.Contains(row.Detail, "volumes") {
		t.Fatal(row)
	}
	t.Setenv("OBIT_FAKE_DOCKER_MODE", "fail")
	if row := diagnoseDocker(); row.Status != "INFO" {
		t.Fatal(row)
	}
}

func TestDockerDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{exec.ErrNotFound, "Install Docker CLI"},
		{context.DeadlineExceeded, "timed out"},
		{errors.New("permission denied"), "socket permissions"},
		{errors.New("daemon unavailable"), "Start Docker Desktop"},
	} {
		if got := docker.Diagnostic(tc.err); !strings.Contains(got, tc.want) {
			t.Fatal(got, tc.want)
		}
	}
}

func TestDockerDoctorUnavailable(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	testenv.FakeDocker(t)
	t.Setenv("OBIT_FAKE_DOCKER_MODE", "fail")
	c := diagnoseDocker()
	if c.Status != "INFO" || !strings.Contains(c.Detail, "daemon unavailable") || !strings.Contains(c.Detail, "Start Docker Desktop") {
		t.Fatal(c)
	}
}

func TestDockerDoctorLegacyTarget(t *testing.T) {
	st := core.State{Items: []core.Item{{Container: &docker.Identity{ID: "legacy"}}}}
	c := diagnoseDockerTargets(st)
	if c.Status != "WARN" || !strings.Contains(c.Detail, "legacy v14") {
		t.Fatal(c)
	}
	st.Items[0].Container.Target = &docker.Target{Endpoint: "test", DaemonID: "one"}
	if c := diagnoseDockerTargets(st); c.Status != "PASS" {
		t.Fatal(c)
	}
}
