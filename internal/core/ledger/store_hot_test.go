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
	"testing"
)

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
