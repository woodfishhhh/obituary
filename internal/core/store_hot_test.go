package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

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
	for _, part := range []string{"burials", "cleanups", "clear-burials", "clear-cleanups", "loaded", "version", "reference", "index"} {
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
		st.queueAudit(Event{Action: "hot"})
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
			st.queueAudit(Event{Action: "pending"})
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	err := store.UpdateHot(func(st *State) error {
		if st.pendingAuditCount() != maxPendingAudit {
			return fmt.Errorf("hot audit reservation lost: %d", st.pendingAuditCount())
		}
		return st.requireAuditSpace()
	})
	if !errors.Is(err, errAuditBacklog) {
		t.Fatal("hot audit space undercounted purge intent", err)
	}
}
