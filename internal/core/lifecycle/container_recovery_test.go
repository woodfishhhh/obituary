package lifecycle_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	core "github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

func TestContainerRetryUsesHotTransaction(t *testing.T) {
	for _, pending := range []bool{false, true} {
		t.Run(fmt.Sprint(pending), func(t *testing.T) {
			f, _, it := containerFixture(t)
			if pending {
				saveContainerIntent(t, f, stopIntent(f, it))
			}
			before, restore := unreadableSnapshot(t, f.svc.Store)
			out := core.Outcome{Item: it, Path: it.Path, Action: core.ActionRetry, Detail: "retry reason"}
			if err := core.CommitContainerRetry(f.svc, out); err != nil {
				t.Fatal("container retry read cold evidence", err)
			}
			restore()
			after, err := f.svc.Store.ReadSnapshot()
			if err != nil || *after.Graveyard != *before.Graveyard || len(after.ContainerIntents) != len(before.ContainerIntents) {
				t.Fatal("container retry changed cold evidence or intent", after, err)
			}
			wantRev := it.Rev + 1
			if pending {
				wantRev = it.Rev
			}
			if len(after.Items) != 1 || after.Items[0].Rev != wantRev {
				t.Fatal("container retry changed pending revision or lost retry", after.Items)
			}
		})
	}
}

func stopIntent(f *fixture, it core.Item) core.ContainerIntent {
	running := true
	return core.ContainerIntent{Token: "interrupted-stop", Operation: "stop", Phase: "sent", WasRunning: &running, At: f.clock, Item: &it}
}

func saveContainerIntent(t *testing.T, f *fixture, p core.ContainerIntent) {
	t.Helper()
	if err := f.svc.Store.Update(func(st *core.State) error { st.ContainerIntents = append(st.ContainerIntents, p); return nil }); err != nil {
		t.Fatal(err)
	}
}

func TestContainerRemovalSnapshotSurvivesRecovery(t *testing.T) {
	f, d, _ := containerFixture(t)
	f.clock = f.clock.Add(2 * time.Hour)
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	b := f.burials()[0]
	p := core.ContainerIntent{Token: "interrupted-rm", Operation: "rm", At: f.clock, Burial: &b, RemovalIDs: []string{b.Container.ID, strings.Repeat("b", 64)}}
	saveContainerIntent(t, f, p)
	d.missing = true
	for range 2 {
		if _, _, err := f.svc.Tick(); err != nil {
			t.Fatal(err)
		}
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || st.Stats.Purged != 0 || len(st.Burials) != 1 || len(st.ContainerIntents) != 1 || st.ContainerIntents[0].Diagnostic == "" {
		t.Fatal("recovery ignored collateral disappearance", st, err)
	}
}

func TestContainerUnknownStopPreservesChangedDecision(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(fmt.Sprint(cancel), func(t *testing.T) {
			f, d, it := containerFixture(t)
			f.clock = f.clock.Add(2 * time.Hour)
			saveContainerIntent(t, f, stopIntent(f, it))
			refs := []string{fmt.Sprint(it.ID)}
			if cancel {
				if _, err := f.svc.Cancel(refs); err != nil {
					t.Fatal(err)
				}
			} else if _, err := f.svc.Extend(refs, time.Hour); err != nil {
				t.Fatal(err)
			}
			for range 2 {
				if _, err := f.svc.Sweep(false); err != nil {
					t.Fatal(err)
				}
				f.clock = f.clock.Add(time.Minute)
			}
			st, err := f.svc.Store.ReadSnapshot()
			if err != nil || len(st.ContainerIntents) != 0 || len(d.calls) != 0 || !d.row.Running {
				t.Fatal("changed lifespan retained ineffective stop", st, d.calls, err)
			}
		})
	}
}

func TestContainerInfiniteStopRemainsUnknown(t *testing.T) {
	f, d, it := containerFixture(t)
	f.clock = f.clock.Add(2 * time.Hour)
	p := stopIntent(f, it)
	p.StopTimeout = new(-1)
	saveContainerIntent(t, f, p)
	for range 2 {
		if _, err := f.svc.Sweep(false); err != nil {
			t.Fatal(err)
		}
		f.clock = f.clock.Add(48 * time.Hour)
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || len(st.ContainerIntents) != 1 || len(d.calls) != 0 {
		t.Fatal("infinite stop blindly retried", st, d.calls, err)
	}
}

func TestContainerStopRecoveryUsesActualState(t *testing.T) {
	for _, running := range []bool{false, true} {
		t.Run(fmt.Sprint(running), func(t *testing.T) {
			f, d, it := containerFixture(t)
			f.clock = f.clock.Add(2 * time.Hour)
			saveContainerIntent(t, f, stopIntent(f, it))
			d.row.Running = running
			rows, err := f.svc.Sweep(false)
			st, readErr := f.svc.Store.ReadSnapshot()
			if err != nil || readErr != nil || len(d.calls) != 0 || (running && len(st.ContainerIntents) != 1) || (!running && len(st.ContainerIntents) != 0) {
				t.Fatalf("recovery repeated stop: %+v calls=%v err=%v/%v", rows, d.calls, err, readErr)
			}
			if running && (len(st.Burials) != 0 || len(st.Items) != 1 || st.Items[0].ExpiresAt != it.ExpiresAt) {
				t.Fatal("running container buried or lifespan changed", st)
			}
			if !running && (len(st.Burials) != 1 || st.Stats.Buried != 1) {
				t.Fatal("stopped container not recovered", st)
			}
		})
	}
}

func TestContainerCanceledStopRecoveryRestarts(t *testing.T) {
	f, d, it := containerFixture(t)
	f.clock = f.clock.Add(2 * time.Hour)
	saveContainerIntent(t, f, stopIntent(f, it))
	if rows, err := f.svc.Cancel([]string{fmt.Sprint(it.ID)}); err != nil || core.Failed(rows) {
		t.Fatal(rows, err)
	}
	d.row.Running = false
	if _, _, err := f.svc.Tick(); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || !d.row.Running || len(st.Items) != 0 || len(st.Burials) != 0 || len(st.ContainerIntents) != 0 {
		t.Fatal("canceled stop not compensated", st, err)
	}
	events, err := f.svc.Store.History(20)
	found := false
	for _, e := range events {
		if e.Action == "container-resumed" {
			found = true
		}
	}
	if err != nil || !found {
		t.Fatal("compensation audit missing", events, err)
	}
}

func TestContainerGraveRecoveryDoesNotRepeatAction(t *testing.T) {
	for _, op := range []string{"start", "rm"} {
		for _, applied := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/%v", op, applied), func(t *testing.T) {
				f, d, _ := containerFixture(t)
				f.clock = f.clock.Add(2 * time.Hour)
				if _, err := f.svc.Sweep(false); err != nil {
					t.Fatal(err)
				}
				b := f.burials()[0]
				saveContainerIntent(t, f, core.ContainerIntent{Token: "interrupted", Operation: op, At: f.clock, Burial: &b})
				d.calls = nil
				d.row.Running, d.missing = op == "start" && applied, op == "rm" && applied
				_, rows, err := f.svc.Tick()
				st, readErr := f.svc.Store.ReadSnapshot()
				if err != nil || readErr != nil || len(d.calls) != 0 {
					t.Fatalf("recovery repeated destructive action: rows=%+v calls=%v err=%v/%v", rows, d.calls, err, readErr)
				}
				if len(st.ContainerIntents) != 0 || applied && len(st.Burials) != 0 || !applied && len(st.Burials) != 1 {
					t.Fatal("incorrect recovered grave", st)
				}
			})
		}
	}
}

func TestContainerExternalRemovalSettles(t *testing.T) {
	for _, phase := range []string{"registered", "stop", "resume", "start", "grave"} {
		t.Run(phase, func(t *testing.T) {
			f, d, it := containerFixture(t)
			f.clock = f.clock.Add(2 * time.Hour)
			switch phase {
			case "stop", "resume":
				p := stopIntent(f, it)
				p.Operation = phase
				saveContainerIntent(t, f, p)
			case "start", "grave":
				if _, err := f.svc.Sweep(false); err != nil {
					t.Fatal(err)
				}
				if phase == "start" {
					b := f.burials()[0]
					saveContainerIntent(t, f, core.ContainerIntent{Token: "start", Operation: "start", At: f.clock, Burial: &b})
				}
			}
			d.missing = true
			f.clock = f.clock.Add(8 * 24 * time.Hour)
			if _, _, err := f.svc.Tick(); err != nil {
				t.Fatal(err)
			}
			st, err := f.svc.Store.ReadSnapshot()
			if err != nil || len(st.Items)+len(st.Burials)+len(st.ContainerIntents) != 0 {
				t.Fatalf("external removal stranded record: items=%d graves=%d intents=%d err=%v", len(st.Items), len(st.Burials), len(st.ContainerIntents), err)
			}
			if st.Stats.Purged != 0 || st.Stats.Resurrected != 0 {
				t.Fatal("external deletion counted as our action", st.Stats)
			}
			events, err := f.svc.Store.History(20)
			count := 0
			for _, e := range events {
				if e.Action == core.ActionLost {
					count++
				}
			}
			if err != nil || count != 1 {
				t.Fatal("external deletion audit missing or duplicated", events, err)
			}
		})
	}
}

func TestContainerStopCommitFailureRecovers(t *testing.T) {
	f, d, _ := containerFixture(t)
	f.clock = f.clock.Add(2 * time.Hour)
	tmp := filepath.Join(f.svc.Store.Home, "store.json.tmp")
	d.afterStop = func() {
		if err := os.Mkdir(tmp, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.svc.Sweep(false); err == nil {
		t.Fatal("stop commit failure not injected")
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || len(st.ContainerIntents) != 1 || len(st.Burials) != 0 {
		t.Fatal("stop lost recovery evidence", st, err)
	}
	if err := os.Remove(tmp); err != nil {
		t.Fatal(err)
	}
	d.afterStop = nil
	if _, err := f.svc.Sweep(false); err != nil {
		t.Fatal(err)
	}
	st, err = f.svc.Store.ReadSnapshot()
	if err != nil || st.Stats.Buried != 1 || strings.Join(d.calls, ",") != "stop" {
		t.Fatal("stop recovery repeated work", st, d.calls, err)
	}
}
