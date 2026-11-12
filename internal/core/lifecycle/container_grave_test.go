package lifecycle_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/docker"
	"github.com/woodfishhhh/obituary/internal/testenv"
)

func TestContainerPurgeReplacementKeepsAmbiguousIntent(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	f, it, row, state, _ := fakeStopFixture(t)
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	row.Running = false
	neighbor := row
	neighbor.ID, neighbor.Name = strings.Repeat("b", 64), "neighbor"
	testenv.WriteDocker(t, state, row, neighbor)
	t.Setenv("OBIT_FAKE_DOCKER_RM_REPLACE", row.ID)
	f.clock = f.clock.Add(8 * 24 * time.Hour)
	for range 2 {
		if _, err := f.svc.PurgeRefs([]string{fmt.Sprint(it.ID)}, false); err != nil {
			t.Fatal(err)
		}
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || st.Stats.Purged != 0 || len(st.Burials) != 1 || len(st.ContainerIntents) != 1 {
		t.Fatal("ambiguous removal committed as purged", st, err)
	}
	events, err := f.svc.Store.History(20)
	count := 0
	for _, e := range events {
		if e.Action == "container-ambiguous" {
			count++
		}
	}
	if err != nil || count != 1 {
		t.Fatal("ambiguous removal audit missing or duplicated", events, err)
	}
}

func TestContainerGraveCommitFailureRecovers(t *testing.T) {
	for _, op := range []string{"start", "rm"} {
		t.Run(op, func(t *testing.T) {
			f, d, it := containerFixture(t)
			f.clock = f.clock.Add(2 * time.Hour)
			if _, err := f.svc.Sweep(false); err != nil {
				t.Fatal(err)
			}
			f.clock = f.clock.Add(8 * 24 * time.Hour)
			tmp := filepath.Join(f.svc.Store.Home, "store.json.tmp")
			d.afterAction = func(string) {
				if err := os.Mkdir(tmp, 0700); err != nil {
					t.Fatal(err)
				}
			}
			action := f.svc.Resurrect
			if op == "rm" {
				action = func(refs []string) ([]core.GraveResult, error) { return f.svc.PurgeRefs(refs, false) }
			}
			rows, err := action([]string{fmt.Sprint(it.ID)})
			if err == nil {
				t.Fatal("grave commit failure not injected", rows)
			}
			st, err := f.svc.Store.ReadSnapshot()
			if err != nil || len(st.ContainerIntents) != 1 || len(st.Burials) != 1 {
				t.Fatal("grave recovery evidence lost", st, err)
			}
			if err := os.Remove(tmp); err != nil {
				t.Fatal(err)
			}
			d.afterAction = nil
			if _, _, err := f.svc.Tick(); err != nil {
				t.Fatal(err)
			}
			st, err = f.svc.Store.ReadSnapshot()
			if err != nil || len(st.Burials) != 0 || len(st.ContainerIntents) != 0 || strings.Join(d.calls, ",") != "stop,"+op {
				t.Fatal("grave commit recovery repeated action", st, d.calls, err)
			}
			if st.Stats.Purged+st.Stats.Resurrected != 1 {
				t.Fatal("grave recovery accounting duplicated", st.Stats)
			}
		})
	}
}

func TestContainerGraveCommitRechecksIntent(t *testing.T) {
	f, d, it := containerFixture(t)
	f.clock = f.clock.Add(2 * time.Hour)
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	f.clock = f.clock.Add(8 * 24 * time.Hour)
	d.afterAction = func(string) {
		if err := f.svc.Store.Update(func(st *core.State) error { st.ContainerIntents[0].Token = "replacement"; return nil }); err != nil {
			t.Fatal(err)
		}
	}
	_, err := f.svc.PurgeRefs([]string{fmt.Sprint(it.ID)}, false)
	st, readErr := f.svc.Store.ReadSnapshot()
	if err == nil || readErr != nil || len(st.Burials) != 1 || st.Stats.Purged != 0 {
		t.Fatal("changed intent committed stale result", st, err, readErr)
	}
}

func TestContainerPurgeRecoversInterruptedCommit(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	f := newFixture(t)
	executable, state, _ := testenv.FakeDocker(t)
	f.svc.Docker = docker.CLI{Executable: executable}
	row := docker.Container{Identity: docker.Identity{ID: strings.Repeat("e", 64), Name: "cache", Created: f.clock.Add(-time.Hour)}, Running: true}
	testenv.WriteDocker(t, state, row)
	it, err := f.svc.AddContainer("cache", core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	f.clock = f.clock.Add(2 * time.Hour)
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	f.clock = f.clock.Add(8 * 24 * time.Hour)
	if err := f.svc.Store.Update(func(st *core.State) error {
		st.Burials[0].PurgeIntent = &core.PurgeIntent{VerifiedAt: f.clock, Basis: "container retention elapsed"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	testenv.WriteDocker(t, state)
	results, err := f.svc.PurgeRefs([]string{fmt.Sprint(it.ID)}, false)
	if err != nil || len(results) != 1 || results[0].Action != core.ActionPurged {
		t.Fatal(results, err)
	}
	st, err := f.svc.Store.Read()
	if err != nil || len(st.Burials) != 0 || st.Stats.Purged != 1 {
		t.Fatal(st, err)
	}
}

func TestContainerLifecycle(t *testing.T) {
	for _, mode := range []string{core.ModeGraveyard, core.ModeRecycle} {
		t.Run(mode, func(t *testing.T) {
			f, d, it := containerFixture(t)
			f.svc.Mode = mode
			f.clock = f.clock.Add(2 * time.Hour)
			rows, err := f.svc.Sweep(false)
			if err != nil || len(rows) != 1 || rows[0].Action != core.ActionBuried {
				t.Fatalf("sweep=%+v %v", rows, err)
			}
			restored, err := f.svc.Resurrect([]string{fmt.Sprint(it.ID)})
			if err != nil || core.GraveFailed(restored) || len(restored) != 1 || restored[0].Action != core.ActionResurrected {
				t.Fatalf("restore=%+v %v", restored, err)
			}
			if strings.Join(d.calls, ",") != "stop,start" {
				t.Fatal(d.calls)
			}
		})
	}
}

func TestContainerPurgeRunningPreserved(t *testing.T) {
	f, d, it := containerFixture(t)
	f.clock = f.clock.Add(2 * time.Hour)
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	d.row.Running = true
	f.clock = f.clock.Add(8 * 24 * time.Hour)
	_, rows, err := f.svc.Tick()
	if err != nil || len(rows) != 1 || rows[0].Action != core.ActionResurrected {
		t.Fatalf("purge=%+v %v", rows, err)
	}
	for _, call := range d.calls {
		if call == "rm" {
			t.Fatal("running container removed")
		}
	}
	st, err := f.svc.Store.Read()
	if err != nil || st.BurialIndex(it.ID) >= 0 {
		t.Fatal(st, err)
	}
}

func TestContainerFakeDockerPurge(t *testing.T) {
	if testing.Short() {
		t.Skip("fake Docker executable")
	}
	f := newFixture(t)
	executable, state, log := testenv.FakeDocker(t)
	f.svc.Docker = docker.CLI{Executable: executable}
	row := docker.Container{Identity: docker.Identity{ID: strings.Repeat("c", 64), Name: "cache", Created: f.clock.Add(-time.Hour)}, Running: true}
	testenv.WriteDocker(t, state, row)
	it, err := f.svc.AddContainer("cache", core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	refs := []string{fmt.Sprint(it.ID)}
	if results, err := f.svc.Extend(refs, time.Hour); err != nil || core.Failed(results) {
		t.Fatal(results, err)
	}
	f.clock = f.clock.Add(3 * time.Hour)
	if rows, err := f.svc.Sweep(false); err != nil || len(rows) != 1 || rows[0].Action != core.ActionBuried {
		t.Fatal(rows, err)
	}
	if rows, err := f.svc.PurgeRefs(refs, false); err != nil || len(rows) != 1 || rows[0].Error == "" {
		t.Fatal("retention bypassed", rows, err)
	}
	f.clock = f.clock.Add(8 * 24 * time.Hour)
	_, rows, err := f.svc.Tick()
	if err != nil || len(rows) != 1 || rows[0].Action != core.ActionPurged {
		t.Fatal(rows, err)
	}
	st, err := f.svc.Store.Read()
	if err != nil || len(st.Burials) != 0 || st.Stats.Purged != 1 {
		t.Fatal(st, err)
	}
	removed := false
	for _, raw := range testenv.DockerCalls(t, log) {
		args := testenv.DockerCommand(raw)
		if args[0] == "rm" {
			removed = true
			if len(args) != 3 || args[1] != "--" || args[2] != row.ID {
				t.Fatalf("unsafe removal: %v", args)
			}
		}
	}
	if !removed {
		t.Fatal("container never removed")
	}
}
