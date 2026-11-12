//go:build windows

package core_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/testenv"
	"github.com/woodfishhhh/obituary/internal/watch"
	"golang.org/x/sys/windows"
)

func TestStoreContainerIntentV14Compatibility(t *testing.T) {
	f, _, it := containerFixture(t)
	path := filepath.Join(f.svc.Store.Home, "store.json")
	data, err := os.ReadFile(path)
	if err != nil || bytes.Contains(data, []byte("containerIntents")) {
		t.Fatal("empty optional intent serialized", err)
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || st.Version != 17 || len(st.ContainerIntents) != 0 {
		t.Fatal(st, err)
	}
	saveContainerIntent(t, f, stopIntent(f, it))
	st, err = f.svc.Store.ReadSnapshot()
	if err != nil || st.Version != 17 || len(st.ContainerIntents) != 1 || st.ContainerIntents[0].Item.Rev != it.Rev {
		t.Fatal("v14 intent did not round trip", st, err)
	}
	if err := f.svc.Store.Update(func(st *core.State) error { st.ContainerIntents = nil; return nil }); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil || bytes.Contains(data, []byte("containerIntents")) {
		t.Fatal("retired intent serialized", err)
	}
}

func TestStoreContainerMigration(t *testing.T) {
	f, _, it := containerFixture(t)
	path := filepath.Join(f.svc.Store.Home, "store.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var st core.State
	if err := json.Unmarshal(data, &st); err != nil {
		t.Fatal(err)
	}
	st.Version = 13
	legacy := core.Item{ID: 39, Rev: 7, Path: f.file("legacy.txt"), CreatedAt: f.clock, ExpiresAt: f.clock.Add(time.Hour)}
	identity, identifyErr := f.svc.FS.Identify(legacy.Path)
	if identifyErr != nil {
		t.Fatal(identifyErr)
	}
	legacy.Identity = &identity
	st.Items = []core.Item{legacy}
	st.NextID = 40
	data, _ = json.Marshal(st)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	read, err := f.svc.Store.Inspect()
	if err != nil || read.Version != 13 || read.NextID != 40 {
		t.Fatal(read, err)
	}
	it, err = f.svc.AddContainer("cache", core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	if err != nil || it.ID != 40 {
		t.Fatal(it, err)
	}
	read, err = f.svc.Store.Read()
	if err != nil || read.Version != 17 || read.Index(it.ID) < 0 || read.Items[read.Index(it.ID)].Container.ID != it.Container.ID {
		t.Fatal("v13 migration lost container", read, err)
	}
	beforeFile, _ := json.Marshal(legacy)
	afterFile, _ := json.Marshal(read.Items[read.Index(legacy.ID)])
	if !bytes.Equal(beforeFile, afterFile) {
		t.Fatal("container migration changed file record")
	}
	read.Version = 18
	data, _ = json.Marshal(read)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Store.Read(); err == nil || !strings.Contains(err.Error(), "newer Obituary") || strings.Contains(err.Error(), "corrupt") {
		t.Fatal("newer ledger accepted", err)
	}
	after, _ := os.ReadFile(path)
	if !bytes.Equal(after, data) {
		t.Fatal("future ledger overwritten")
	}
}

func TestStoreUnchangedUpdateDoesNotCommit(t *testing.T) {
	f := newFixture(t)
	f.add(f.file("keep.txt"), time.Hour)
	path := filepath.Join(f.svc.Store.Home, "store.json")
	commits := 0
	restore := core.SetReplacementSync(func(file *os.File) error {
		if strings.HasSuffix(file.Name(), filepath.Base(path)+".tmp") {
			commits++
		}
		return nil
	})
	defer restore()
	if err := f.svc.Store.Update(func(*core.State) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if commits != 0 {
		t.Fatalf("unchanged state replaced ledger %d times", commits)
	}
}

func TestLegacyWatchApprovalRequiresNewDecision(t *testing.T) {
	if testing.Short() {
		t.Skip("legacy policy expiry uses a real Git repository")
	}
	for _, version := range []int{9, 10, 11} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			f := newFixture(t)
			testenv.InitRepo(t, f.dir)
			f.svc.Owner = auto.Owner
			path := f.file("downloads/old.txt")
			p, err := f.svc.SetWatch(filepath.Dir(path), "2s", core.WatchFirstSeen)
			if err != nil {
				t.Fatal(err)
			}
			approveWatch(t, f.svc)
			if err := f.svc.RegisterWatchChild(p, path); err != nil {
				t.Fatal(err)
			}
			storePath := filepath.Join(f.svc.Store.Home, "store.json")
			data, err := os.ReadFile(storePath)
			if err != nil {
				t.Fatal(err)
			}
			var legacy map[string]any
			decoder := json.NewDecoder(bytes.NewReader(data))
			decoder.UseNumber()
			if err := decoder.Decode(&legacy); err != nil {
				t.Fatal(err)
			}
			legacy["version"] = version
			for _, key := range []string{"graveyard", "previousGraveyard", "graveyardIndex"} {
				delete(legacy, key)
			}
			for _, p := range legacy["watches"].([]any) {
				delete(p.(map[string]any), "approvedRev")
			}
			data, err = json.Marshal(legacy)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(storePath, data, 0600); err != nil {
				t.Fatal(err)
			}
			migrated, err := f.svc.Store.Read()
			if err != nil || migrated.Watches[0].Consent != "pending" || len(migrated.Proposals) != 1 {
				t.Fatalf("legacy approval survived without evidence: %+v %v", migrated.Watches, err)
			}
			p, err = f.svc.SetWatch(p.Path, p.Lifespan, p.Age)
			st, readErr := f.svc.Store.Read()
			if err != nil || readErr != nil || p.Consent != "pending" || len(st.Proposals) != 1 {
				t.Fatalf("legacy approval survived without evidence: %+v %+v %v %v", p, st.Proposals, err, readErr)
			}
			proposal := st.Proposals[0].ID
			st, err = f.svc.Store.Read()
			if err != nil || len(st.Proposals) != 1 || st.Proposals[0].ID != proposal {
				t.Fatalf("migration is not idempotent: %+v %v", st, err)
			}
			out := f.sweepAt(20 * time.Minute)
			if len(out) != 1 || out[0].Action != core.ActionRetry || !exists(path) || len(f.items()) != 1 {
				t.Fatalf("pending migration moved or cancelled old child: %+v", out)
			}
			approveWatch(t, f.svc)
			p, err = f.svc.SetWatch(p.Path, p.Lifespan, p.Age)
			if err != nil || p.Consent != core.ConsentApproved {
				t.Fatalf("explicit approval was lost: %+v %v", p, err)
			}
			if err := f.svc.RegisterWatchChild(p, f.file("downloads/new.txt")); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStoreUpgradesLegacyBurials(t *testing.T) {
	if testing.Short() {
		t.Skip("too slow for testing.Short")
	}

	for _, version := range []int{5, 6, 7} {
		t.Run(fmt.Sprint(version), func(t *testing.T) {
			f := newFixture(t)
			b := buryOne(t, f, "payload.txt")
			st, err := f.svc.Store.ReadSnapshot()
			if err != nil {
				t.Fatal(err)
			}
			st.Version = version
			st.Burials[0].Operation = nil // legacy versions did not record command batches
			if version == 5 {
				st.Burials[0].Parents = nil
			}
			data, err := json.Marshal(st)
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(f.svc.Store.Home, "store.json")
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			rows, err := f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
			if err != nil || core.GraveFailed(rows) || read(t, b.Path) != "payload.txt" {
				t.Fatalf("legacy burial failed to migrate: %+v %v", rows, err)
			}
			st, err = f.svc.Store.ReadSnapshot()
			if err != nil || st.Version != 17 || len(st.Burials) != 0 || len(st.RestoreCleanups) != 0 {
				t.Fatalf("legacy store did not reach v17: %+v %v", st, err)
			}
		})
	}
}

func TestStoreVersionTenRetainsWatchKeepDecisions(t *testing.T) {
	f := newFixture(t)
	path := f.file("kept.log")
	id, err := f.svc.FS.Identify(path)
	if err != nil {
		t.Fatal(err)
	}
	old := core.State{Version: 10, NextID: 1, WatchExemptions: []core.WatchExemption{{WatchID: 42, Identity: id}}}
	data, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(f.svc.Store.Home, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.svc.Store.Home, "store.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := f.svc.Store.Update(func(*core.State) error { return nil }); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.Read()
	if err != nil || st.Version != 17 || len(st.WatchExemptions) != 1 || st.WatchExemptions[0].Identity != id || len(st.AutoExemptions) != 0 {
		t.Fatalf("v10 lost keep decisions on upgrade: %+v %v", st, err)
	}
}

func TestStoreVersionTwelveRetainsWatchBurialAndHookReceipts(t *testing.T) {
	if testing.Short() {
		t.Skip("migration and watch expiry use a real Git repository")
	}
	f, path, _ := watchFixture(t, "tree/payload.zip", core.WatchFirstSeen)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	legacy, err := f.svc.Store.ReadSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	legacy.Version = 12
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.svc.Store.Home, "store.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	queued := f.file("queued.log")
	requests, err := f.svc.CaptureObservations([]string{queued}, "migration")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.Add([]string{queued}, core.AddOptions{ExpiresAt: f.clock.Add(24 * time.Hour), Observed: &requests[0]})
	if err != nil || core.Failed(rows) {
		t.Fatalf("observed registration: %+v %v", rows, err)
	}
	f.sweepAt(2 * time.Hour)
	proposal := expiredWatchProposal(t, f)
	// Rewriting the migrated ledger must preserve both independent v13 features.
	if err := f.svc.Store.Update(func(st *core.State) error { st.NextID++; return nil }); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.ReadSnapshot()
	if err != nil || st.Version != 17 || len(st.Proposals) != 1 || st.Proposals[0].WatchBurial == nil || st.Proposals[0].WatchBurial.ItemID != legacy.Items[0].ID || st.Proposals[0].WatchBurial.Identity != *legacy.Items[0].Identity || core.ProposalFingerprint(st.Proposals[0]) != core.ProposalFingerprint(proposal) || !st.ObservationCompleted(&requests[0]) || !st.ObservationReceipts[requests[0].Token].Equal(requests[0].ObservedAt) || !exists(path) {
		t.Fatalf("v12 migration lost watch burial review or hook receipt: %+v %v", st, err)
	}
	if _, err := f.svc.Add([]string{queued}, core.AddOptions{ExpiresAt: f.clock.Add(48 * time.Hour), Observed: &requests[0]}); err != nil {
		t.Fatal(err)
	}
	st, err = f.svc.Store.ReadSnapshot()
	queuedIndex := st.Index(rows[0].Item.ID)
	if err != nil || len(st.Items) != 2 || queuedIndex < 0 || st.Items[queuedIndex].Rev != 1 || len(st.Proposals) != 1 || core.ProposalFingerprint(st.Proposals[0]) != core.ProposalFingerprint(proposal) {
		t.Fatalf("replayed hook changed the migrated ledger: %+v %v", st, err)
	}
}

func TestVersionTwoStoreRetainsSessionRejectionAfterUpgrade(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(f.svc.Store.Home, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.svc.Store.Home, "store.json"), []byte(`{"version":2,"nextId":1,"items":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	path := f.file("leftover.txt")
	p, _, _, err := f.svc.Propose(core.ProposalLeftovers, f.dir, "s", []core.Candidate{{Path: path, Lifespan: "3d"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Reject([]string{fmt.Sprint(p.ID)}); err != nil {
		t.Fatal(err)
	}
	st, err := (core.Store{Home: f.svc.Store.Home}).Read()
	if err != nil || st.Version != 17 || !st.LeftoverRejected(strings.ToUpper(f.dir), "s", strings.ToUpper(path)) {
		t.Fatalf("upgraded rejection not durable: %+v, %v", st, err)
	}
}

func TestStoreNormalizesUNCVolumes(t *testing.T) {
	f := newFixture(t)
	if err := f.svc.Store.Update(func(st *core.State) error {
		st.Items = []core.Item{{ID: 1, Identity: &core.Identity{Volume: `\\SERVER\Share\`, Serial: 1, FileID: "file"}}}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.Read()
	if err != nil || st.Items[0].Identity.Volume != `\\server\share\` || st.FindIdentity(core.Identity{Volume: `\\Server\SHARE\`, Serial: 1, FileID: "file"}) != 0 {
		t.Fatalf("UNC identity depends on spelling: %+v, %v", st.Items, err)
	}
}

func TestLegacyIdentityVolumesAreNormalized(t *testing.T) {
	f := newFixture(t)
	path := f.file("legacy.log")
	f.add(path, 24*time.Hour)
	b := buryOne(t, f, "buried.log")
	if err := f.svc.Store.Update(func(st *core.State) error {
		for i := range st.Items {
			st.Items[i].Identity.Volume = strings.ToLower(st.Items[i].Identity.Volume)
		}
		for i := range st.Burials {
			st.Burials[i].Identity.Volume = strings.ToLower(st.Burials[i].Identity.Volume)
			st.Burials[i].GraveID.Volume = strings.ToLower(st.Burials[i].GraveID.Volume)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if st.Items[0].Identity.Volume != strings.ToUpper(st.Items[0].Identity.Volume) {
		t.Errorf("legacy item not normalized: %+v", st.Items[0].Identity)
	}
	for _, b := range st.Burials {
		for _, id := range []*core.Identity{b.Identity, b.GraveID} {
			if id.Volume != strings.ToUpper(id.Volume) {
				t.Errorf("legacy identity not normalized: %+v", id)
			}
		}
	}
	got, err := f.svc.Resurrect([]string{fmt.Sprint(b.ID)})
	if err != nil || core.GraveFailed(got) {
		t.Fatalf("legacy burial not restorable: %+v, %v", got, err)
	}
}

func TestStoreMergesNormalizedDuplicateIdentities(t *testing.T) {
	for _, version := range []int{2, 3} {
		for _, manualFirst := range []bool{false, true} {
			t.Run(fmt.Sprintf("v%d/manualFirst=%v", version, manualFirst), func(t *testing.T) {
				f := newFixture(t)
				path := f.file("legacy.log")
				first := f.add(path, time.Hour)
				first.Source, first.Project, first.Note = "rule", f.dir, "keep lower ID metadata"
				second := first
				id := *first.Identity
				id.Volume = strings.ToLower(id.Volume)
				second.ID, second.Identity, second.ExpiresAt, second.Source = 7, &id, f.clock.Add(30*24*time.Hour), "cli"
				second.Note = "discard higher ID metadata"
				if manualFirst {
					first.Source, second.Source = "mktemp", "session"
				}
				data, err := json.Marshal(core.State{Version: version, Items: []core.Item{second, first}})
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(f.svc.Store.Home, "store.json"), data, 0o600); err != nil {
					t.Fatal(err)
				}
				st, err := f.svc.Store.Read()
				wantSource := "cli"
				if manualFirst {
					wantSource = "mktemp"
				}
				if err != nil || len(st.Items) != 1 || st.Items[0].ID != first.ID || !st.Items[0].ExpiresAt.Equal(second.ExpiresAt) || st.Items[0].Source != wantSource || st.Items[0].Note != first.Note || st.NextID <= second.ID {
					t.Fatalf("duplicate identity was not conservatively merged: %+v, %v", st, err)
				}
				events, err := f.svc.Store.History(20)
				if err != nil || len(events) < 2 || events[len(events)-1].Action != "merged" {
					t.Fatalf("missing merge audit: %+v, %v", events, err)
				}
				if _, err := f.svc.Store.Read(); err != nil {
					t.Fatal(err)
				}
				again, _ := f.svc.Store.History(20)
				if len(again) != len(events) {
					t.Fatal("read repeated the migration audit")
				}
				if out := f.sweepAt(2 * time.Hour); len(out) != 0 || !exists(path) {
					t.Fatalf("merged expiry was shortened: %+v", out)
				}
			})
		}
	}
}

func TestV4IntentFieldsAreIgnored(t *testing.T) {
	f := newFixture(t)
	path := f.file("keep.txt")
	item := f.add(path, time.Hour)
	storePath := filepath.Join(f.svc.Store.Home, "store.json")
	data, err := os.ReadFile(storePath)
	if err != nil {
		t.Fatal(err)
	}
	var st map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&st); err != nil {
		t.Fatal(err)
	}
	st["version"] = 4
	for _, key := range []string{"graveyard", "previousGraveyard", "graveyardIndex"} {
		delete(st, key)
	}
	old := st["items"].([]any)[0].(map[string]any)
	old["buryingFrom"], old["buryingGrave"], old["buryingReason"] = "ignored", "ignored", "bury"
	data, _ = json.Marshal(st)
	if err := os.WriteFile(storePath, data, 0o600); err != nil {
		t.Fatal(err)
	}
	rows, err := f.svc.Extend([]string{fmt.Sprint(item.ID)}, time.Hour)
	if err != nil || core.Failed(rows) || !exists(path) {
		t.Fatalf("v4 intent still affects ordinary commands: %+v, %v", rows, err)
	}
	data, err = os.ReadFile(storePath)
	var upgraded struct {
		Version int `json:"version"`
	}
	if err != nil || json.Unmarshal(data, &upgraded) != nil || strings.Contains(string(data), "burying") || upgraded.Version != 17 {
		t.Fatalf("upgraded ledger retained v4 intent: %s, %v", data, err)
	}
}

func TestMigrationAuditFailureReturnsCommittedAdd(t *testing.T) {
	f := newFixture(t)
	path := f.file("old.txt")
	first := f.add(path, time.Hour)
	second := first
	second.ID = 7
	data, err := json.Marshal(core.State{Version: 4, Items: []core.Item{first, second}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.svc.Store.Home, "store.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	audit := filepath.Join(f.svc.Store.Home, "audit.jsonl")
	if err := os.Rename(audit, audit+".saved"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(audit, 0o700); err != nil {
		t.Fatal(err)
	}
	newPath := f.file("new.txt")
	got, err := f.svc.Add([]string{newPath}, core.AddOptions{ExpiresAt: f.clock.Add(time.Hour)})
	var warning *core.AuditError
	if !errors.As(err, &warning) || len(got) != 1 || got[0].Item == nil || got[0].Item.Path != newPath || core.Failed(got) {
		t.Fatalf("committed add was lost behind migration warning: %+v, %v", got, err)
	}
	st, err := f.svc.Store.Inspect()
	if err != nil || st.Index(got[0].Item.ID) < 0 {
		t.Fatalf("result not committed: %+v, %v", st, err)
	}
}

func TestStoreDeadlineUsesSharedReadAndBoundsWrite(t *testing.T) {
	f := newFixture(t)
	f.add(f.file("keep.txt"), time.Hour)
	release, err := core.LockFile(filepath.Join(f.svc.Store.Home, "store.lock"), false)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	store := f.svc.Store.WithDeadline(time.Now().Add(100 * time.Millisecond))
	if st, err := store.Read(); err != nil || len(st.Items) != 1 {
		t.Fatalf("bounded read did not share the lock: %+v, %v", st, err)
	}
	called := false
	err = store.Update(func(*core.State) error { called = true; return nil })
	if !errors.Is(err, context.DeadlineExceeded) || called {
		t.Fatalf("bounded write ran under another reader: called=%v, %v", called, err)
	}
}

func TestCorruptStoreWithDuplicateIDsIsRejected(t *testing.T) {
	f := newFixture(t)
	if err := os.MkdirAll(f.svc.Store.Home, 0o700); err != nil {
		t.Fatal(err)
	}
	data := `{"version":1,"items":[{"id":1,"path":"C:\a"},{"id":1,"path":"C:\b"}]}`
	if err := os.WriteFile(filepath.Join(f.svc.Store.Home, "store.json"), []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := f.svc.Store.Read(); err == nil {
		t.Fatal("duplicate ids must be rejected")
	}
}

func TestDiagnosticReadersAllowConcurrentCommits(t *testing.T) {
	if testing.Short() {
		t.Skip("stresses concurrent readers across many ledger replacements")
	}
	f := newFixture(t)
	if err := f.svc.Store.Update(func(st *core.State) error {
		for i := range 1024 {
			st.Items = append(st.Items, core.Item{ID: int64(i + 1), Path: filepath.Join(f.dir, fmt.Sprint(i)), Note: strings.Repeat("x", 4096)})
		}
		st.NextID = 1025
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	ready := make(chan struct{}, 8)
	errs := make(chan error, 8)
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			ready <- struct{}{}
			for {
				select {
				case <-stop:
					return
				default:
				}
				var err error
				if i%2 == 0 {
					_, err = f.svc.Store.Inspect()
				} else {
					_, err = f.svc.PendingGraves()
				}
				if err != nil {
					errs <- err
					return
				}
			}
		})
	}
	for range 8 {
		<-ready
	}
	failed := 0
	for range 50 {
		if err := f.svc.Store.Update(func(st *core.State) error { st.NextID++; return nil }); err != nil {
			t.Logf("commit failed: %v", err)
			failed++
		}
	}
	close(stop)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("diagnostic reader failed: %v", err)
	}
	if failed != 0 {
		t.Fatalf("diagnostic reader blocked ledger replacement: %d commits failed", failed)
	}
}

func TestStoreReplaceWithDeleteSharedReader(t *testing.T) {
	f := newFixture(t)
	f.add(f.file("one"), time.Hour)
	p, _ := windows.UTF16PtrFromString(filepath.Join(f.svc.Store.Home, "store.json"))
	h, err := windows.CreateFile(p, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	if err := f.svc.Store.Update(func(st *core.State) error { st.NextID++; return nil }); err != nil {
		t.Fatalf("shared reader blocked replacement: %v", err)
	}
}

func TestStoreMigratesWatchTimingConservatively(t *testing.T) {
	f, path, _ := watchFixture(t, "legacy.txt", core.WatchFirstSeen)
	if err := watch.Reconcile(f.svc); err != nil {
		t.Fatal(err)
	}
	st, err := f.svc.Store.Read()
	if err != nil {
		t.Fatal(err)
	}
	st.Version, st.Items[0].Watch = 9, nil
	data, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.svc.Store.Home, "store.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	approveWatch(t, f.svc)
	out := f.sweepAt(2 * time.Hour)
	if !exists(path) || len(f.burials()) != 0 {
		t.Fatalf("legacy watch bypassed fresh stability window: %+v", out)
	}
	st, err = f.svc.Store.Read()
	if err != nil || st.Version != 17 || st.Items[0].Watch == nil {
		t.Fatalf("watch migration: %+v %v", st, err)
	}
	out = f.sweepAt(core.WatchStableWindow + time.Minute)
	if len(out) != 1 || out[0].Action != core.ActionBuried {
		t.Fatalf("migrated stable watch did not expire: %+v", out)
	}
}
