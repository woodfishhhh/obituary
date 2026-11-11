package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func legacyLedger(t *testing.T, indented bool) (Store, []byte, State) {
	t.Helper()
	return legacyLedgerVersion(t, 15, indented)
}

func legacyLedgerVersion(t *testing.T, version int, indented bool) (Store, []byte, State) {
	t.Helper()
	store := Store{Home: t.TempDir()}
	st := State{Version: version, NextID: 20}
	for i, op := range operationEvidence() {
		b := Burial{ID: int64(i + 1), Parents: parentEvidence(), Operation: op,
			Identity: benchmarkIdentity(i + 1), GraveID: benchmarkIdentity(i + 100), Project: `C:\project`, Session: "session"}
		st.Burials = append(st.Burials, b)
		b.ID += 10
		st.RestoreCleanups = append(st.RestoreCleanups, RestoreCleanup{Burial: b})
	}
	data, err := json.Marshal(st)
	if indented {
		data, err = json.MarshalIndent(st, "", "  ")
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.statePath(), data, 0600); err != nil {
		t.Fatal(err)
	}
	return store, data, st
}

func TestLedgerMigrationPreservesParentEvidence(t *testing.T) {
	for _, indented := range []bool{false, true} {
		t.Run(map[bool]string{false: "compact", true: "indented"}[indented], func(t *testing.T) {
			store, original, want := legacyLedger(t, indented)
			for _, read := range []func() (State, error){store.Inspect, store.ReadSnapshot, store.WithDeadline(time.Now().Add(time.Second)).Read} {
				got, err := read()
				if err != nil || !reflect.DeepEqual(got.Burials, want.Burials) || !reflect.DeepEqual(got.RestoreCleanups, want.RestoreCleanups) {
					t.Fatalf("legacy read changed evidence: %+v %v", got, err)
				}
			}
			if backups, _ := filepath.Glob(filepath.Join(store.Home, "*.bak")); len(backups) != 0 {
				t.Fatal("snapshot created migration evidence")
			}
			if err := store.Update(func(*State) error { return nil }); err != nil {
				t.Fatal(err)
			}
			assertMigratedLedger(t, store, original, want)
			if err := store.FlushAudit(); err != nil {
				t.Fatal(err)
			}
			if err := store.Update(func(*State) error { return nil }); err != nil {
				t.Fatal(err)
			}
			events, err := store.History(10)
			if err != nil || len(events) != 1 || events[0].Action != "store-migrated" {
				t.Fatalf("migration event not delivered exactly once on normal retry: %+v %v", events, err)
			}
		})
	}
}

func assertMigratedLedger(t *testing.T, store Store, original []byte, want State) {
	t.Helper()
	got, err := store.Inspect()
	if err != nil || got.Version != stateVersion || !reflect.DeepEqual(got.Burials, want.Burials) || !reflect.DeepEqual(got.RestoreCleanups, want.RestoreCleanups) {
		t.Fatalf("migration changed burial evidence: %+v %v", got, err)
	}
	backups, _ := filepath.Glob(filepath.Join(store.Home, "*.bak"))
	if len(backups) != 1 {
		t.Fatalf("migration backups: %v", backups)
	}
	data, err := os.ReadFile(backups[0])
	if err != nil || !bytes.Equal(data, original) {
		t.Fatal("original bytes unavailable for rollback", err)
	}
	data, err = os.ReadFile(store.statePath())
	if err != nil || bytes.Contains(bytes.TrimSpace(data), []byte("\n")) || len(got.PendingAudit) != 1 || got.PendingAudit[0].Action != "store-migrated" {
		t.Fatalf("compact ledger or transactional migration event missing: %v", err)
	}
}

func TestLedgerMigrationPreservesOperationsAcrossVersions(t *testing.T) {
	for version := 1; version <= 15; version++ {
		t.Run(fmt.Sprintf("v%d", version), func(t *testing.T) {
			store, original, want := legacyLedgerVersion(t, version, version%2 == 0)
			if err := store.Update(func(*State) error { return nil }); err != nil {
				t.Fatal(err)
			}
			assertMigratedLedger(t, store, original, want)
			got, err := store.Inspect()
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(got.PendingAudit[0].Detail, fmt.Sprintf("v%d to v16", version)) {
				t.Fatalf("migration audit has wrong versions: %+v", got.PendingAudit)
			}
		})
	}
}

func TestLedgerMigrationRecoversInterruptedCommits(t *testing.T) {
	RequireFsync(t)
	for _, stage := range []string{"backup", "ledger"} {
		t.Run(stage, func(t *testing.T) {
			store, original, want := legacyLedger(t, true)
			oldSync := syncReplacement
			syncReplacement = func(f *os.File) error {
				if stage == "backup" && strings.HasSuffix(f.Name(), ".bak.tmp") || stage == "ledger" && filepath.Base(f.Name()) == "store.json.tmp" {
					return errors.New("injected interrupted commit")
				}
				return syncFile(f)
			}
			t.Cleanup(func() { syncReplacement = oldSync })
			if err := store.Update(func(*State) error { return nil }); err == nil {
				t.Fatal("interrupted migration reported success")
			}
			if stage == "backup" {
				data, _ := os.ReadFile(store.statePath())
				if !bytes.Equal(data, original) {
					t.Fatal("ledger changed before backup confirmation")
				}
			}
			syncReplacement = oldSync
			if err := store.Update(func(*State) error { return nil }); err != nil {
				t.Fatal(err)
			}
			assertMigratedLedger(t, store, original, want)
		})
	}
}

func TestLedgerMigrationResumesBeforeReplacement(t *testing.T) {
	store, original, want := legacyLedger(t, true)
	blocked := store.statePath() + ".tmp"
	if err := os.Mkdir(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(func(*State) error { return nil }); err == nil {
		t.Fatal("blocked replacement reported success")
	}
	data, _ := os.ReadFile(store.statePath())
	if !bytes.Equal(data, original) {
		t.Fatal("failed replacement changed legacy ledger")
	}
	if err := os.Rename(blocked, blocked+".held"); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(func(*State) error { return nil }); err != nil {
		t.Fatal(err)
	}
	assertMigratedLedger(t, store, original, want)
}

func TestLedgerMigrationAuditRetriesAndBackupRollsBack(t *testing.T) {
	store, original, want := legacyLedger(t, true)
	if err := store.Update(func(*State) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(store.auditPath(), 0700); err != nil {
		t.Fatal(err)
	}
	if err := store.FlushAudit(); err == nil {
		t.Fatal("blocked audit reported success")
	}
	assertMigratedLedger(t, store, original, want)
	if err := os.Rename(store.auditPath(), store.auditPath()+".held"); err != nil {
		t.Fatal(err)
	}
	if err := store.FlushAudit(); err != nil {
		t.Fatal(err)
	}
	backups, _ := filepath.Glob(filepath.Join(store.Home, "*.bak"))
	data, err := os.ReadFile(backups[0])
	if err != nil || os.WriteFile(store.statePath(), data, 0600) != nil {
		t.Fatal("restore original backup", err)
	}
	got, err := store.Inspect()
	if err != nil || got.Version != want.Version || !reflect.DeepEqual(got.Burials, want.Burials) {
		t.Fatal("backup did not restore legacy evidence", got, err)
	}
	if err := store.Update(func(*State) error { return nil }); err != nil {
		t.Fatal(err)
	}
	assertMigratedLedger(t, store, original, want)
}

func TestLedgerMigrationBackupFailureKeepsOriginal(t *testing.T) {
	store, original, _ := legacyLedger(t, false)
	oldSync := syncReplacement
	syncReplacement = func(*os.File) error { return errors.New("backup unavailable") }
	t.Cleanup(func() { syncReplacement = oldSync })
	if err := store.Update(func(st *State) error { st.Burials = nil; return nil }); err == nil {
		t.Fatal("mutation committed without rollback evidence")
	}
	data, err := os.ReadFile(store.statePath())
	if err != nil || !bytes.Equal(data, original) {
		t.Fatal("backup failure changed ledger", err)
	}
}
