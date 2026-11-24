package ledger

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/woodfishhhh/obituary/internal/docker"
	"testing"
	"time"
)

func TestPresentationReadsCommittedStateDuringWriter(t *testing.T) {
	s := snapshotTestStore(t)
	release, err := LockFile(filepath.Join(s.Home, "store.lock"), true)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	s = s.WithDeadline(time.Now().Add(150 * time.Millisecond))
	if _, err := s.ReadActive(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.ReadSummary(); err != nil {
		t.Fatal(err)
	}
}

func TestReadActiveAndSummaryAllowAnEmptyLedger(t *testing.T) {
	store := Store{Home: t.TempDir()}
	active, err := store.ReadActive()
	if err != nil || len(active.Items) != 0 {
		t.Fatalf("empty active read: %+v %v", active, err)
	}
	state, summary, err := store.ReadSummary()
	if err != nil || len(state.Items) != 0 || summary != (GraveyardSummary{}) {
		t.Fatalf("empty summary read: %+v %+v %v", state, summary, err)
	}
}

func TestReadViewsRejectColdSnapshotIndexDamage(t *testing.T) {
	store := snapshotTestStore(t)
	st, err := store.ReadSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(store.Home, st.Graveyard.File)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]json.RawMessage
	if err := json.Unmarshal(fields["burials"], &rows); err != nil {
		t.Fatal(err)
	}
	rows[0]["parentRef"] = json.RawMessage("-1")
	fields["burials"], err = json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	st.Graveyard.Hash = fmt.Sprintf("%x", sha256.Sum256(data))
	hot, err := marshalHot(st)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.statePath(), hot, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadActive(); err == nil {
		t.Fatal("active read accepted a damaged cold reference")
	}
	if _, _, err := store.ReadSummary(); err == nil {
		t.Fatal("summary read accepted a damaged cold reference")
	}
}

func TestReadViewsRejectCrossEntityIDConflict(t *testing.T) {
	store := snapshotTestStore(t)
	if err := store.UpdateGraveyard(func(st *State) error {
		st.Items[0].ID = st.Burials[0].ID
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadActive(); err == nil {
		t.Fatal("active read accepted an active/graveyard ID conflict")
	}
	if _, _, err := store.ReadSummary(); err == nil {
		t.Fatal("summary read accepted an active/graveyard ID conflict")
	}
}

func TestReadActiveRetriesInterruptedMigration(t *testing.T) {
	s, _, old := legacyLedgerVersion(t, 16, false)
	old.Items = []Item{{ID: 90, Identity: benchmarkIdentity(1000)}, {ID: 91, Identity: benchmarkIdentity(1000)}}
	raw, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.statePath(), raw, 0600); err != nil {
		t.Fatal(err)
	}
	blocked := s.statePath() + ".tmp"
	if err := os.Mkdir(blocked, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ReadActive(); err == nil {
		t.Fatal("presentation skipped interrupted migration")
	}
	got, err := os.ReadFile(s.statePath())
	if err != nil || !bytes.Equal(raw, got) {
		t.Fatal("failed migration changed original", err)
	}
	if err := os.Rename(blocked, blocked+".held"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		st, err := s.ReadActive()
		if err != nil || len(st.Items) != 1 || st.Items[0].ID != 90 {
			t.Fatal("migration retry lost the merged item", err)
		}
	}
	assertMigratedLedger(t, s, raw, old)
	if err := s.FlushAudit(); err != nil {
		t.Fatal(err)
	}
	events, err := s.History(10)
	if err != nil {
		t.Fatal(err)
	}
	counts := map[string]int{}
	for _, ev := range events {
		counts[ev.Action]++
	}
	if counts["store-migrated"] != 1 || counts["merged"] != 1 {
		t.Fatalf("duplicate or missing migration audit: %v", counts)
	}
}

func TestHotLegacyNormalizationPreservesGraves(t *testing.T) {
	store, _, state := legacyLedgerVersion(t, 16, false)
	state.Burials[0].Identity.Volume = `\\?\c:\`
	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.statePath(), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateHot(func(*State) error { return nil }); err != nil {
		t.Fatal("legacy normalization mistaken for hot graveyard mutation", err)
	}
	st, err := store.ReadSnapshot()
	if err != nil || len(st.Burials) != len(state.Burials) || st.Version != stateVersion {
		t.Fatal("hot migration lost graveyard", err)
	}
}

func TestHotLedgerRejectsDowngradedReference(t *testing.T) {
	store := snapshotTestStore(t)
	data, err := os.ReadFile(store.statePath())
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	fields["version"] = json.RawMessage("16")
	data, _ = json.Marshal(fields)
	if err := os.WriteFile(store.statePath(), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadSnapshot(); err == nil {
		t.Fatal("legacy version discarded referenced graveyard")
	}
}

func TestHotTransactionRejectsGraveyardChanges(t *testing.T) {
	for _, part := range []string{"burials", "cleanups", "clear-burials", "clear-cleanups", "loaded", "version", "reference", "index", "gc-cursor"} {
		t.Run(part, func(t *testing.T) {
			store := Store{Home: t.TempDir()}
			if err := store.save(benchmarkLedger()); err != nil {
				t.Fatal(err)
			}
			before, _ := os.ReadFile(store.statePath())
			err := store.UpdateHot(func(st *State) error {
				switch part {
				case "burials":
					st.Burials = append(st.Burials, Burial{ID: st.NextID})
				case "cleanups":
					st.RestoreCleanups = append(st.RestoreCleanups, RestoreCleanup{Burial: Burial{ID: st.NextID}})
				case "clear-burials":
					st.Burials = nil
				case "clear-cleanups":
					st.RestoreCleanups = nil
				case "loaded":
					if err := store.loadGraveyard(st); err != nil {
						return err
					}
					st.Burials[0].Size++
				case "version":
					st.Version--
				case "reference":
					st.Graveyard.Hash = strings.Repeat("0", 64)
				case "index":
					st.GraveyardIndex.Keys = nil
				case "gc-cursor":
					st.GraveyardGCCursor = "changed"
				}
				st.NextID++
				return nil
			})
			if err == nil {
				t.Fatal("hot transaction accepted graveyard mutation")
			}
			after, _ := os.ReadFile(store.statePath())
			if !bytes.Equal(before, after) {
				t.Fatal("rejected hot transaction changed committed state")
			}
		})
	}
}

func TestHotUpdateDoesNotReadOrWriteGraveyard(t *testing.T) {
	store := snapshotTestStore(t)
	before, err := store.ReadSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	oldRead := readGraveyardSnapshot
	readGraveyardSnapshot = func(string) ([]byte, error) { return nil, errors.New("cold IO forbidden") }
	t.Cleanup(func() { readGraveyardSnapshot = oldRead })
	if err := store.UpdateHot(func(st *State) error {
		st.Items[0].Rev++
		QueueAudit(st, Event{Action: "hot"})
		return nil
	}); err != nil {
		t.Fatal("hot update read graveyard", err)
	}
	if err := store.FlushAudit(); err != nil {
		t.Fatal("outbox read graveyard", err)
	}
	store.RefreshGraveNoticeIndex()
	readGraveyardSnapshot = oldRead
	after, err := store.ReadSnapshot()
	if err != nil || !sameGraveyardReference(before.Graveyard, after.Graveyard) || !reflect.DeepEqual(before.Burials, after.Burials) || after.Items[0].Rev != before.Items[0].Rev+1 {
		t.Fatal("hot update changed snapshot or lost item", err)
	}
}

func TestHotAuditReservationMatchesJoinedState(t *testing.T) {
	store := snapshotTestStore(t)
	if err := store.UpdateGraveyard(func(st *State) error {
		st.Burials[0].PurgeIntent = &PurgeIntent{}
		for range maxPendingAudit - 1 {
			QueueAudit(st, Event{Action: "pending"})
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	err := store.UpdateHot(func(st *State) error {
		if PendingAuditCount(st) != maxPendingAudit {
			return fmt.Errorf("hot audit reservation lost: %d", PendingAuditCount(st))
		}
		return RequireAuditSpace(st)
	})
	if !errors.Is(err, ErrAuditBacklog) {
		t.Fatal("hot audit space undercounted purge intent", err)
	}
}

func TestPresentationReadsDockerVolumeEntriesAndFallsBackForVolumeBurials(t *testing.T) {
	store := Store{Home: t.TempDir()}
	volume := &docker.Volume{
		Name: "cache", Driver: "local", Scope: "local", CreatedAt: time.Now(),
		Target: docker.Target{Context: "test", Endpoint: "unix:///docker.sock", DaemonID: "daemon", OSType: "linux"},
	}
	if err := store.UpdateHot(func(st *State) error {
		st.Items = append(st.Items, Item{Kind: KindDockerVolume, Volume: volume, ID: st.NextID, Rev: 1, Path: volume.Name, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)})
		st.NextID++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	active, err := store.ReadActive()
	if err != nil || len(active.Items) != 1 || active.Items[0].Volume == nil {
		t.Fatalf("active presentation lost Docker volume: %+v %v", active, err)
	}
	if err := store.UpdateGraveyard(func(st *State) error {
		st.Items = st.Items[:0]
		st.Burials = append(st.Burials, Burial{Kind: KindDockerVolume, Volume: volume, ID: 2, Path: volume.Name, Grave: "volume.tar", Size: 8, BuriedAt: time.Now(), PurgeAt: time.Now().Add(time.Hour), Reason: "bury"})
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	full, summary, err := store.ReadSummary()
	if err != nil || len(full.Burials) != 1 || summary.Count != 1 || full.Burials[0].Volume == nil {
		t.Fatalf("volume burial did not fall back to a complete read: %+v %+v %v", full, summary, err)
	}
}

// A burial made by the command line carries longNames and references into the
// snapshot's tables; the summary must hold it, or every real ledger is read twice
// (the summary projection fails, then the whole cold snapshot is decoded).
func TestReadActiveHoldsBurialsThatReferenceSnapshotTables(t *testing.T) {
	s := snapshotTestStore(t)
	reads, old := 0, readGraveyardSnapshot
	readGraveyardSnapshot = func(path string) ([]byte, error) { reads++; return old(path) }
	t.Cleanup(func() { readGraveyardSnapshot = old })
	if _, summary, err := s.ReadSummary(); err != nil || summary.Count != 1 || reads != 1 {
		t.Fatalf("summary %+v after %d snapshot reads: %v", summary, reads, err)
	}
}

func TestSummaryRefsMustLandOnTableRows(t *testing.T) {
	one, bad := 1, 5
	row := json.RawMessage(`{}`)
	for name, tc := range map[string]struct {
		burial graveyardSummaryBurial
		snap   graveyardSummarySnapshot
		ok     bool
	}{
		"in range":      {graveyardSummaryBurial{ParentRef: &one}, graveyardSummarySnapshot{ParentChains: []json.RawMessage{row, row}}, true},
		"no references": {graveyardSummaryBurial{}, graveyardSummarySnapshot{}, true},
		"out of range":  {graveyardSummaryBurial{OperationRef: &bad}, graveyardSummarySnapshot{Operations: []json.RawMessage{row}}, false},
		"negative":      {graveyardSummaryBurial{OriginRef: new(int)}, graveyardSummarySnapshot{Origins: nil}, false},
		"null row":      {graveyardSummaryBurial{ParentRef: &one}, graveyardSummarySnapshot{ParentChains: []json.RawMessage{row, json.RawMessage("null")}}, false},
	} {
		t.Run(name, func(t *testing.T) {
			tc.snap.Burials = []graveyardSummaryBurial{tc.burial}
			if err := validateSummaryRefs(tc.snap); (err == nil) != tc.ok {
				t.Fatalf("err %v, want ok=%v", err, tc.ok)
			}
		})
	}
}
