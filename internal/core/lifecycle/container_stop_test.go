package lifecycle_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/docker"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func fakeStopFixture(t *testing.T) (*fixture, core.Item, docker.Container, string, string) {
	t.Helper()
	f := newFixture(t)
	exe, state, log := testenv.FakeDocker(t)
	f.svc.Docker = docker.CLI{Executable: exe}
	row := docker.Container{Identity: docker.Identity{ID: strings.Repeat("a", 64), Name: "cache", Created: f.clock}, Running: true}
	testenv.WriteDocker(t, state, row)
	it, err := f.svc.AddContainer("cache", core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	f.clock = f.clock.Add(2 * time.Hour)
	return f, it, row, state, log
}

type unsentContainerStop struct{ *containerStub }

func (d unsentContainerStop) Stop(_ context.Context, _ docker.Identity, beforeSend func(docker.Container) error) error {
	if err := beforeSend(d.row); err != nil {
		return err
	}
	return docker.ErrNotSent
}

func TestContainerUnsentStopClearsIntent(t *testing.T) {
	f, d, _ := containerFixture(t)
	f.svc.Docker = unsentContainerStop{d}
	f.clock = f.clock.Add(2 * time.Hour)
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || len(st.ContainerIntents) != 0 || len(st.Items) != 1 || len(st.Burials) != 0 || !d.row.Running {
		t.Fatal("unsent subprocess left sent intent", st, err)
	}
	f.svc.Docker = d
	f.clock = f.clock.Add(time.Minute)
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	if len(f.burials()) != 1 {
		t.Fatal("unsent subprocess could not retry")
	}
}

func TestContainerStopPreflightRetries(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	f, _, row, state, log := fakeStopFixture(t)
	neighbor := row
	neighbor.ID, neighbor.Name = strings.Repeat("b", 64), row.ID
	testenv.WriteDocker(t, state, row, neighbor)
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || len(st.ContainerIntents) != 0 && st.ContainerIntents[0].Phase != "prepared" {
		t.Errorf("unsent stop retained sent intent: %+v %v", st.ContainerIntents, err)
	}
	for _, raw := range testenv.DockerCalls(t, log) {
		if testenv.DockerCommand(raw)[0] == "stop" {
			t.Fatal("preflight issued stop")
		}
	}
	testenv.WriteDocker(t, state, row)
	f.clock = f.clock.Add(time.Minute)
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	st, err = f.svc.Store.ReadSnapshot()
	if err != nil || len(st.Burials) != 1 || len(st.ContainerIntents) != 0 {
		t.Fatal("preflight refusal permanently blocked stop", st, err)
	}
}

func TestContainerStopReadbackTimeoutKeepsIntent(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable and five-second readback timeout")
	}
	f, _, row, state, _ := fakeStopFixture(t)
	row.StopTimeout = new(-1)
	testenv.WriteDocker(t, state, row)
	t.Setenv("OBIT_FAKE_DOCKER_AFTER_STOP_HANG", "info")
	start := time.Now()
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(start); elapsed > 8*time.Second {
		t.Fatalf("readback held graveyard lock too long: %s", elapsed)
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || len(st.ContainerIntents) != 1 || st.ContainerIntents[0].Phase != "sent" || len(st.Burials) != 0 {
		t.Fatal("readback failure lost pending stop", st, err)
	}
	t.Setenv("OBIT_FAKE_DOCKER_AFTER_STOP_HANG", "")
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	if len(f.burials()) != 1 {
		t.Fatal("readback recovery did not commit stopped container")
	}
}

func TestContainerSentStopEventuallyRetries(t *testing.T) {
	f, d, it := containerFixture(t)
	f.clock = f.clock.Add(2 * time.Hour)
	p := stopIntent(f, it)
	p.StopTimeout = new(90)
	saveContainerIntent(t, f, p)
	for _, elapsed := range []time.Duration{0, 94 * time.Second, time.Second} {
		f.clock = f.clock.Add(elapsed)
		if _, err := f.svc.Sweep(false); err != nil {
			t.Fatal(err)
		}
		if len(d.calls) != 0 {
			t.Fatal("unknown stop retried before observation window elapsed", d.calls)
		}
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || len(st.ContainerIntents) != 0 {
		t.Fatal("running sent stop never became retryable", st, err)
	}
	f.clock = f.clock.Add(time.Minute)
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	if len(f.burials()) != 1 || strings.Join(d.calls, ",") != "stop" {
		t.Fatal("expired unknown stop failed to retry", d.calls)
	}
	events, err := f.svc.Store.History(20)
	found := false
	for _, e := range events {
		found = found || e.Action == "container-stop-retry"
	}
	if err != nil || !found {
		t.Fatal("stop retry audit missing", events, err)
	}
}

func TestContainerSentCrashRetriesWithFakeDocker(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	f, it, _, _, log := fakeStopFixture(t)
	saveContainerIntent(t, f, stopIntent(f, it))
	for range 2 {
		if _, err := f.svc.Sweep(false); err != nil {
			t.Fatal(err)
		}
		f.clock = f.clock.Add(time.Minute)
	}
	for _, raw := range testenv.DockerCalls(t, log) {
		if testenv.DockerCommand(raw)[0] == "stop" {
			t.Fatal("unknown stop resent during observation")
		}
	}
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	if len(f.burials()) != 1 {
		t.Fatal("crash before stop permanently blocked container")
	}
}

func TestContainerSlowStopAllowsHookWrite(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker stop waits two seconds")
	}
	f := newFixture(t)
	exe, state, _ := testenv.FakeDocker(t)
	f.svc.Docker = docker.CLI{Executable: exe}
	row := docker.Container{Identity: docker.Identity{ID: strings.Repeat("c", 64), Created: f.clock.Add(-time.Hour), Name: "cache"}, Running: true}
	testenv.WriteDocker(t, state, row)
	if _, err := f.svc.AddContainer("cache", core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	entered := filepath.Join(t.TempDir(), "entered")
	t.Setenv("OBIT_FAKE_DOCKER_STOP_DELAY", "2s")
	t.Setenv("OBIT_FAKE_DOCKER_ENTERED", entered)
	f.clock = f.clock.Add(2 * time.Hour)
	done := make(chan error, 1)
	go func() { _, err := f.svc.Sweep(false); done <- err }()
	t.Cleanup(func() {
		if done != nil {
			<-done
		}
	})
	deadline := time.Now().Add(10 * time.Second)
	for !exists(entered) {
		if time.Now().After(deadline) {
			t.Fatal("fake stop did not begin")
		}
		time.Sleep(10 * time.Millisecond)
	}
	start := time.Now()
	err := f.svc.Store.WithDeadline(start.Add(time.Second)).Update(func(st *core.State) error {
		st.Projects = map[string]core.ProjectState{"hook": {Consent: core.ConsentApproved}}
		return nil
	})
	if err != nil {
		t.Errorf("hook write blocked during stop: %v", err)
	}
	select {
	case err := <-done:
		done = nil
		t.Fatalf("stop ended before hook write: %v", err)
	default:
	}
	t.Logf("hook write during 2s stop took %s", time.Since(start))
	if err := <-done; err != nil {
		t.Error(err)
	}
	done = nil
}

func TestContainerStopRunningAfterCommandRemainsLive(t *testing.T) {
	f, d, _ := containerFixture(t)
	d.afterStop = func() { d.row.Running = true }
	f.clock = f.clock.Add(2 * time.Hour)
	rows, err := f.svc.Sweep(false)
	st, readErr := f.svc.Store.ReadSnapshot()
	if err != nil || readErr != nil || len(st.Burials) != 0 || len(st.Items) != 1 || len(st.ContainerIntents) != 0 {
		t.Fatal("running container buried after stop", rows, st, err, readErr)
	}
}

type deadlineContainerStop struct {
	docker.CLI
	ctx context.Context
}

func (d deadlineContainerStop) Stop(_ context.Context, id docker.Identity, beforeSend func(docker.Container) error) error {
	return d.CLI.Stop(d.ctx, id, beforeSend)
}

func TestContainerStopTimeoutKeepsIntent(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable and delayed engine stop")
	}
	f := newFixture(t)
	exe, state, _ := testenv.FakeDocker(t)
	cli := docker.CLI{Executable: exe}
	f.svc.Docker = cli
	row := docker.Container{Identity: docker.Identity{ID: strings.Repeat("d", 64), Name: "delayed", Created: f.clock}, Running: true}
	testenv.WriteDocker(t, state, row)
	it, err := f.svc.AddContainer("delayed", core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("OBIT_FAKE_DOCKER_STOP_DELAY", "2s")
	t.Setenv("OBIT_FAKE_DOCKER_BACKEND_STOP", "1")
	ready := filepath.Join(t.TempDir(), "stop.ready")
	t.Setenv("OBIT_FAKE_DOCKER_ENTERED", ready)
	ctx, started := testenv.ReadyDeadline(t, ready, 500*time.Millisecond)
	f.svc.Docker = deadlineContainerStop{CLI: cli, ctx: ctx}
	f.clock = f.clock.Add(2 * time.Hour)
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	testenv.ReadyElapsed(t, started)
	f.svc.Docker = cli
	if _, err := f.svc.Cancel([]string{fmt.Sprint(it.ID)}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || len(st.ContainerIntents) != 1 {
		t.Fatalf("in-flight backend stop lost its intent: intents=%d err=%v", len(st.ContainerIntents), err)
	}
	time.Sleep(2100 * time.Millisecond)
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	got, err := cli.Inspect(t.Context(), row.ID)
	st, readErr := f.svc.Store.ReadSnapshot()
	if err != nil || readErr != nil || !got.Running || len(st.ContainerIntents) != 0 {
		t.Fatal("late stop was not compensated", got, st, err, readErr)
	}
}

func TestContainerStopHonorsChangedLifespan(t *testing.T) {
	for _, running := range []bool{false, true} {
		for _, cancel := range []bool{false, true} {
			t.Run(fmt.Sprintf("%v/%v", cancel, running), func(t *testing.T) {
				f, d, it := containerFixture(t)
				d.row.Running = running
				d.afterStop = func() {
					svc := *f.svc
					svc.Store = svc.Store.WithDeadline(time.Now().Add(100 * time.Millisecond))
					refs := []string{fmt.Sprint(it.ID)}
					var results []core.Result
					var err error
					if cancel {
						results, err = svc.Cancel(refs)
					} else {
						results, err = svc.Extend(refs, time.Hour)
					}
					if err != nil || core.Failed(results) {
						t.Errorf("concurrent lifespan write blocked: %v/%v", results, err)
					}
				}
				f.clock = f.clock.Add(2 * time.Hour)
				rows, err := f.svc.Sweep(false)
				st, readErr := f.svc.Store.Read()
				if err != nil || readErr != nil || d.row.Running != running || len(st.Burials) != 0 || st.Stats.Buried != 0 {
					t.Fatalf("changed lifespan left container stopped or buried: rows=%+v state=%+v calls=%v err=%v/%v", rows, st, d.calls, err, readErr)
				}
			})
		}
	}

}
