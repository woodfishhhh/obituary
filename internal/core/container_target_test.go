package core_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/docker"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestContainerLegacyTargetMigration(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	for _, missing := range []bool{false, true} {
		t.Run(fmt.Sprint(missing), func(t *testing.T) {
			f := newFixture(t)
			exe, state, _ := testenv.FakeDocker(t)
			f.svc.Docker = docker.CLI{Executable: exe}
			row := docker.Container{Identity: docker.Identity{ID: strings.Repeat("a", 64), Created: f.clock, Name: "legacy"}, Running: true}
			testenv.WriteDocker(t, state, row)
			_, err := f.svc.AddContainer("legacy", core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
			if err != nil {
				t.Fatal(err)
			}
			if err := f.svc.Store.Update(func(st *core.State) error { st.Items[0].Container.Target = nil; return nil }); err != nil {
				t.Fatal(err)
			}
			if missing {
				testenv.WriteDocker(t, state)
			}
			f.clock = f.clock.Add(2 * time.Hour)
			rows, err := f.svc.Sweep(false)
			st, readErr := f.svc.Store.ReadSnapshot()
			if err != nil || readErr != nil || st.Version != 16 {
				t.Fatal(rows, st, err, readErr)
			}
			if missing {
				if len(st.Items) != 1 || len(st.Burials) != 0 || st.Items[0].Container.Target != nil {
					t.Fatal("unbound absence retired or migrated record", st)
				}
			} else if len(st.Burials) != 1 || st.Burials[0].Container.Target == nil {
				t.Fatal("legacy target was not durably bound", st)
			}
		})
	}
}

func TestContainerAmbiguousActionAudited(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	f := newFixture(t)
	exe, state, _ := testenv.FakeDocker(t)
	f.svc.Docker = docker.CLI{Executable: exe}
	row := docker.Container{Identity: docker.Identity{ID: strings.Repeat("e", 64), Created: f.clock, Name: "cache"}, Running: true}
	testenv.WriteDocker(t, state, row)
	if _, err := f.svc.AddContainer("cache", core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OBIT_FAKE_DOCKER_COLLIDE_AFTER_CHECK", row.ID)
	f.clock = f.clock.Add(2 * time.Hour)
	rows, err := f.svc.Sweep(false)
	st, readErr := f.svc.Store.ReadSnapshot()
	if err != nil || readErr != nil || len(st.Items) != 1 || len(st.ContainerIntents) != 1 || !strings.Contains(rows[0].Detail, "same-name") {
		t.Fatal(rows, st, err, readErr)
	}
	events, err := f.svc.Store.History(20)
	count := 0
	for _, e := range events {
		if e.Action == "container-ambiguous" {
			count++
		}
	}
	if err != nil || count != 1 {
		t.Fatal("ambiguous action audit missing", events, err)
	}
}

func TestContainerDaemonChangePreservesGrave(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	f := newFixture(t)
	exe, state, _ := testenv.FakeDocker(t)
	f.svc.Docker = docker.CLI{Executable: exe}
	row := docker.Container{Identity: docker.Identity{ID: strings.Repeat("b", 64), Created: f.clock, Name: "cache"}, Running: true}
	testenv.WriteDocker(t, state, row)
	it, err := f.svc.AddContainer("cache", core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	f.clock = f.clock.Add(2 * time.Hour)
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	b := f.burials()[0]
	saveContainerIntent(t, f, core.ContainerIntent{Token: "rm", Operation: "rm", At: f.clock, Burial: &b})
	testenv.WriteDocker(t, state)
	t.Setenv("OBIT_FAKE_DOCKER_DAEMON", "replacement")
	rows, err := f.svc.PurgeRefs([]string{fmt.Sprint(it.ID)}, false)
	st, readErr := f.svc.Store.ReadSnapshot()
	if err != nil || readErr != nil || len(st.Burials) != 1 || len(st.ContainerIntents) != 1 || st.Stats.Purged != 0 || !strings.Contains(rows[0].Error, "daemon") {
		t.Fatal("different daemon retired grave", rows, st, err, readErr)
	}
}
