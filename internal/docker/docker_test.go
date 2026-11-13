package docker_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/docker"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func dockerFixture(t *testing.T) (docker.CLI, docker.Container, string, string) {
	t.Helper()
	executable, state, log := testenv.FakeDocker(t)
	row := docker.Container{Identity: docker.Identity{ID: strings.Repeat("a", 64), ShortID: strings.Repeat("a", 12), Name: "cache", Created: time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)}, Running: true}
	testenv.WriteDocker(t, state, row)
	return docker.CLI{Executable: executable}, row, state, log
}

func TestIdentity(t *testing.T) {
	id := docker.Identity{ID: strings.Repeat("a", 64), Created: time.Now()}
	if !id.Valid() || !id.Same(id) {
		t.Fatal(id)
	}
	other := id
	other.Created = other.Created.Add(time.Second)
	if id.Same(other) {
		t.Fatal("creation time ignored")
	}
	other = id
	other.ID = "short"
	if other.Valid() {
		t.Fatal("short ID accepted")
	}
}

func TestDockerCommands(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	cli, row, state, log := dockerFixture(t)
	ctx := t.Context()
	got, err := cli.Inspect(ctx, "cache")
	if err != nil || !got.Same(row.Identity) {
		t.Fatal(got, err)
	}
	rows, err := cli.List(ctx)
	if err != nil || len(rows) != 1 {
		t.Fatal(rows, err)
	}
	if err := cli.Stop(ctx, row.Identity, nil); err != nil {
		t.Fatal(err)
	}
	if err := cli.Start(ctx, row.Identity); err != nil {
		t.Fatal(err)
	}
	running, err := cli.Remove(ctx, row.Identity, nil)
	if err != nil || !running {
		t.Fatal("running container removed", err)
	}
	row.Running = false
	testenv.WriteDocker(t, state, row)
	running, err = cli.Remove(ctx, row.Identity, nil)
	if err != nil || running {
		t.Fatal(running, err)
	}
	calls := testenv.DockerCalls(t, log)
	actions := 0
	for i, raw := range calls {
		args := testenv.DockerCommand(raw)
		if args[0] != "stop" && args[0] != "start" && args[0] != "rm" {
			continue
		}
		actions++
		if len(args) != 3 || args[1] != "--" || args[2] != row.ID {
			t.Fatalf("unsafe Docker arguments: %v", args)
		}
		j := i - 1
		if j >= 0 && testenv.DockerCommand(calls[j])[0] == "ps" {
			j--
		}
		if j < 0 || testenv.DockerCommand(calls[j])[0] != "container" || testenv.DockerCommand(calls[j])[3] != row.ID {
			t.Fatalf("action lacks fresh inspect: %v", calls)
		}
	}
	if actions != 3 {
		t.Fatalf("unexpected actions %v", calls)
	}
}

func TestDockerIdentityRefusal(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	for _, change := range []string{"id", "created"} {
		t.Run(change, func(t *testing.T) {
			cli, row, state, log := dockerFixture(t)
			want := row.Identity
			if change == "id" {
				t.Setenv("OBIT_FAKE_DOCKER_MODE", "wrong-id")
			} else {
				want.Created = want.Created.Add(time.Second)
			}
			testenv.WriteDocker(t, state, row)
			for _, action := range []func(context.Context, docker.Identity) error{func(ctx context.Context, id docker.Identity) error { return cli.Stop(ctx, id, nil) }, cli.Start, func(ctx context.Context, id docker.Identity) error { _, err := cli.Remove(ctx, id, nil); return err }} {
				if err := action(t.Context(), want); err == nil {
					t.Fatal("identity mismatch permitted action")
				}
			}
			for _, raw := range testenv.DockerCalls(t, log) {
				args := testenv.DockerCommand(raw)
				if args[0] == "stop" || args[0] == "start" || args[0] == "rm" {
					t.Fatalf("identity mismatch executed action: %v", args)
				}
			}
		})
	}
}

func TestDockerAutoRemoveRefused(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	cli, row, state, log := dockerFixture(t)
	row.AutoRemove = true
	testenv.WriteDocker(t, state, row)
	if err := cli.Stop(t.Context(), row.Identity, nil); !errors.Is(err, docker.ErrAutoRemove) {
		t.Fatal("auto-remove container stopped", err)
	}
	for _, raw := range testenv.DockerCalls(t, log) {
		args := testenv.DockerCommand(raw)
		if args[0] == "stop" {
			t.Fatal("auto-remove container stopped")
		}
	}
}

func TestDockerStopGracePeriod(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker stop exceeds five seconds")
	}
	cli, row, _, _ := dockerFixture(t)
	t.Setenv("OBIT_FAKE_DOCKER_STOP_DELAY", "6s")
	if err := cli.Stop(t.Context(), row.Identity, nil); err != nil {
		t.Fatal("stop timeout did not cover shutdown grace period", err)
	}
}

func TestDockerEndpointPinned(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	cli, row, state, log := dockerFixture(t)
	got, err := cli.Inspect(t.Context(), row.ID)
	if err != nil {
		t.Fatal(err)
	}
	testenv.WriteDocker(t, state+".other", row)
	t.Setenv("DOCKER_CONTEXT", "other")
	if err := cli.Stop(t.Context(), got.Identity, nil); err != nil {
		t.Fatal("pinned endpoint operation failed after context switch", err)
	}
	t.Setenv("DOCKER_CONTEXT", "")
	original, err := cli.Inspect(t.Context(), row.ID)
	if err != nil || original.Running {
		t.Fatal("endpoint switch stopped another daemon", original, err, testenv.DockerCalls(t, log))
	}
}

func TestDockerFullIDNameCollision(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	cli, row, state, log := dockerFixture(t)
	neighbor := row
	neighbor.ID = strings.Repeat("b", 64)
	neighbor.Name = row.ID
	testenv.WriteDocker(t, state, row, neighbor)
	t.Setenv("OBIT_FAKE_DOCKER_DISAPPEAR_AFTER_INSPECT", row.ID)
	err := cli.Stop(t.Context(), row.Identity, nil)
	if err == nil {
		t.Fatal("full ID fell back to same-name container", testenv.DockerCalls(t, log))
	}
	got, inspectErr := cli.Inspect(t.Context(), neighbor.ID)
	if inspectErr != nil || !got.Running {
		t.Fatal("same-name neighbor was stopped", got, inspectErr, err)
	}
}

func TestDockerLateNameCollisionReported(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	cli, row, _, _ := dockerFixture(t)
	t.Setenv("OBIT_FAKE_DOCKER_COLLIDE_AFTER_CHECK", row.ID)
	err := cli.Stop(t.Context(), row.Identity, nil)
	if !errors.Is(err, docker.ErrAmbiguousAction) {
		t.Fatal("late collision not diagnosed", err)
	}
}

func TestDockerStopSendBoundary(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	for _, mode := range []string{"collision", "callback", "start", "sent"} {
		t.Run(mode, func(t *testing.T) {
			cli, row, state, log := dockerFixture(t)
			if mode == "collision" {
				neighbor := row
				neighbor.ID, neighbor.Name = strings.Repeat("b", 64), row.ID
				testenv.WriteDocker(t, state, row, neighbor)
			}
			ctx := t.Context()
			var started <-chan time.Time
			if mode == "sent" {
				ready := filepath.Join(t.TempDir(), "stop.ready")
				t.Setenv("OBIT_FAKE_DOCKER_ENTERED", ready)
				t.Setenv("OBIT_FAKE_DOCKER_STOP_DELAY", "2s")
				ctx, started = testenv.ReadyDeadline(t, ready, 500*time.Millisecond)
			}
			called := false
			err := cli.Stop(ctx, row.Identity, func(got docker.Container) error {
				called = true
				if !got.Same(row.Identity) {
					t.Fatal("callback identity changed")
				}
				for _, raw := range testenv.DockerCalls(t, log) {
					if testenv.DockerCommand(raw)[0] == "stop" {
						t.Fatal("stop preceded durable callback")
					}
				}
				if mode == "callback" {
					return errors.New("ledger unavailable")
				}
				if mode == "start" {
					return os.Remove(cli.Executable)
				}
				return nil
			})
			if started != nil {
				testenv.ReadyElapsed(t, started)
			}
			if err == nil || errors.Is(err, docker.ErrNotSent) != (mode != "sent") || called != (mode != "collision") {
				t.Fatal("stop send boundary classification", mode, called, err)
			}
		})
	}
}

func TestDockerStopReadbackTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable and five-second readback deadline")
	}
	for _, command := range []string{"context", "info", "container"} {
		t.Run(command, func(t *testing.T) {
			cli, row, state, _ := dockerFixture(t)
			row.StopTimeout = new(-1)
			testenv.WriteDocker(t, state, row)
			t.Setenv("OBIT_FAKE_DOCKER_AFTER_STOP_HANG", command)
			ctx, cancel := context.WithTimeout(t.Context(), 9*time.Second)
			defer cancel()
			start := time.Now()
			err := cli.Stop(ctx, row.Identity, nil)
			elapsed := time.Since(start)
			if !errors.Is(err, context.DeadlineExceeded) || elapsed > 8*time.Second {
				t.Fatalf("readback inherited stop timeout: elapsed=%s err=%v", elapsed, err)
			}
		})
	}
}

func TestDockerRemoveLateReplacementReported(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	cli, row, state, _ := dockerFixture(t)
	row.Running = false
	neighbor := row
	neighbor.ID, neighbor.Name = strings.Repeat("b", 64), "neighbor"
	testenv.WriteDocker(t, state, row, neighbor)
	t.Setenv("OBIT_FAKE_DOCKER_RM_REPLACE", row.ID)
	_, err := cli.Remove(t.Context(), row.Identity, nil)
	if !errors.Is(err, docker.ErrAmbiguousAction) {
		t.Fatal("removed replacement was reported as ordinary success", err)
	}
	if _, err := cli.Inspect(t.Context(), neighbor.ID); !errors.Is(err, docker.ErrNotFound) {
		t.Fatal("replacement deletion window not reproduced", err)
	}
}

// Session cleanup lists every container after each agent turn: one inspect
// process covers them all instead of one per container.
func TestDockerListInspectsInOneCall(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	cli, first, state, log := dockerFixture(t)
	rows := []docker.Container{first}
	for _, c := range "bc" {
		row := first
		row.ID, row.ShortID, row.Name = strings.Repeat(string(c), 64), strings.Repeat(string(c), 12), "cache-"+string(c)
		rows = append(rows, row)
	}
	testenv.WriteDocker(t, state, rows...)
	got, err := cli.List(t.Context())
	if err != nil || len(got) != len(rows) {
		t.Fatal(got, err)
	}
	for i := range rows {
		if !got[i].Same(rows[i].Identity) || got[i].Name != rows[i].Name {
			t.Fatalf("row %d: %+v", i, got[i])
		}
	}
	inspects := 0
	for _, raw := range testenv.DockerCalls(t, log) {
		if args := testenv.DockerCommand(raw); len(args) > 1 && args[0] == "container" && args[1] == "inspect" {
			inspects++
		}
	}
	if inspects != 1 {
		t.Fatalf("%d inspect processes for %d containers", inspects, len(rows))
	}
	// Every listed ID must come back exactly once.
	t.Setenv("OBIT_FAKE_DOCKER_MODE", "wrong-id")
	if got, err := cli.List(t.Context()); !errors.Is(err, docker.ErrIdentity) {
		t.Fatalf("mismatched inspection accepted: %+v %v", got, err)
	}
}

func TestDockerUnavailableIncludesDiagnostic(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	cli, _, _, _ := dockerFixture(t)
	t.Setenv("OBIT_FAKE_DOCKER_MODE", "fail")
	err := cli.Available(t.Context())
	if err == nil || !strings.Contains(err.Error(), "daemon unavailable") {
		t.Fatal("daemon stderr missing", err)
	}
}

func TestDockerUnavailable(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable and timeout")
	}
	cli, _, _, _ := dockerFixture(t)
	for _, mode := range []string{"fail", "sleep"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("OBIT_FAKE_DOCKER_MODE", mode)
			cli.Timeout = 50 * time.Millisecond
			_, err := cli.List(t.Context())
			if err == nil {
				t.Fatal("unavailable daemon accepted")
			}
			if mode == "sleep" && !errors.Is(err, context.DeadlineExceeded) {
				t.Fatal(err)
			}
		})
	}
	t.Setenv("PATH", t.TempDir())
	if _, err := (docker.CLI{}).List(t.Context()); err == nil {
		t.Fatal("missing Docker accepted")
	}
}
