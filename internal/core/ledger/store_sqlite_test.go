package ledger

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSQLiteLedgerUsesWALAndFullDurability(t *testing.T) {
	store := (Store{Home: t.TempDir()}).WithPersistentSQLite()
	if err := store.Update(func(st *State) error {
		st.Items = []Item{{ID: 1, Rev: 1, Path: "one"}, {ID: 2, Rev: 1, Path: "two"}}
		st.NextID = 3
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(store.Home, sqliteDatabaseName))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var journal string
	var synchronous int
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journal); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow("PRAGMA synchronous").Scan(&synchronous); err != nil {
		t.Fatal(err)
	}
	if journal != "wal" || synchronous != 2 {
		t.Fatalf("unsafe SQLite durability: journal=%s synchronous=%d", journal, synchronous)
	}
	if _, err := os.Stat(filepath.Join(store.Home, "store.db")); err != nil {
		t.Fatal(err)
	}
}

func TestSQLiteLedgerSupportsLongUnicodeHome(t *testing.T) {
	home := t.TempDir()
	for range 3 {
		home = filepath.Join(home, strings.Repeat("长目录", 20))
	}
	if err := os.MkdirAll(home, 0o700); err != nil {
		t.Fatal(err)
	}
	store := Store{Home: home}
	if err := store.Update(func(st *State) error {
		st.Items = []Item{{ID: 1, Rev: 1, Path: "one"}}
		st.NextID = 2
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(store.statePath()); err != nil {
		t.Fatalf("long home did not retain JSON fallback: %v", err)
	}
	if _, err := os.Stat(store.databasePath()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("long home unexpectedly created SQLite database: %v", err)
	}
}

func TestSQLiteLedgerUpdatesOnlyChangedRowsAndReplaysReceipt(t *testing.T) {
	store := Store{Home: t.TempDir()}
	if err := store.Update(func(st *State) error {
		st.Items = []Item{{ID: 1, Rev: 1, Path: "one"}, {ID: 2, Rev: 1, Path: "two"}}
		st.NextID = 3
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(store.Home, sqliteDatabaseName))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE changed(kind TEXT, key TEXT);
CREATE TRIGGER changed_rows AFTER UPDATE ON ledger BEGIN INSERT INTO changed VALUES(new.kind, new.key); END;`); err != nil {
		t.Fatal(err)
	}
	var beforeReceipts int
	if err := db.QueryRow("SELECT count(*) FROM receipts").Scan(&beforeReceipts); err != nil {
		t.Fatal(err)
	}
	if err := store.UpdateHot(func(st *State) error {
		st.Items[0].Note = "changed"
		st.Items[0].Rev++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var changed []string
	rows, err := db.Query("SELECT kind || '/' || key FROM changed ORDER BY kind, key")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			t.Fatal(err)
		}
		changed = append(changed, key)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if !reflect.DeepEqual(changed, []string{"items/1"}) {
		t.Fatalf("sparse update rewrote unrelated rows: %v", changed)
	}
	var afterReceipts int
	if err := db.QueryRow("SELECT count(*) FROM receipts").Scan(&afterReceipts); err != nil {
		t.Fatal(err)
	}
	if afterReceipts != beforeReceipts+1 {
		t.Fatalf("commit receipt count=%d, want %d", afterReceipts, beforeReceipts+1)
	}
	if err := store.UpdateHot(func(*State) error { return nil }); err != nil {
		t.Fatal(err)
	}
	var replayReceipts int
	if err := db.QueryRow("SELECT count(*) FROM receipts").Scan(&replayReceipts); err != nil {
		t.Fatal(err)
	}
	if replayReceipts != afterReceipts {
		t.Fatalf("replayed identical commit created a duplicate receipt: %d", replayReceipts)
	}
}

func TestSQLiteHotSessionDecodesOnce(t *testing.T) {
	store := (Store{Home: t.TempDir()}).WithPersistentSQLite()
	items := make([]Item, 10000)
	for i := range items {
		items[i] = Item{ID: int64(i + 1), Rev: 1, Path: fmt.Sprintf("item-%d", i)}
	}
	if err := store.Update(func(st *State) error {
		st.Items, st.NextID = items, int64(len(items)+1)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	clearSQLiteSessionsForTest(store.Home)
	start := sqliteSessionDecodeCount()
	if err := store.UpdateHot(func(st *State) error {
		st.Items[0].Rev++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	first := sqliteSessionDecodeCount()
	if err := store.UpdateHot(func(st *State) error {
		st.Items[0].Rev++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	second := sqliteSessionDecodeCount()
	if first != start+1 || second != first {
		t.Fatalf("SQLite hot session decodes=%d then %d, want one decode", first-start, second-first)
	}
}

func TestSQLiteLedgerMigratesV18AndRefusesV20(t *testing.T) {
	store := snapshotTestStore(t)
	want, err := store.ReadSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	coldPath := filepath.Join(store.Home, want.Graveyard.File)
	cold, err := os.ReadFile(coldPath)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot graveyardSnapshot
	if err := json.Unmarshal(cold, &snapshot); err != nil {
		t.Fatal(err)
	}
	snapshot.Version = 18
	cold, err = json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(coldPath, cold, 0600); err != nil {
		t.Fatal(err)
	}
	want.Version = 18
	want.Graveyard.Hash = fmt.Sprintf("%x", sha256.Sum256(cold))
	hot, err := marshalHot(want)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(store.statePath(), hot, 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{sqliteDatabaseName, sqliteDatabaseName + "-wal", sqliteDatabaseName + "-shm"} {
		_ = os.Remove(filepath.Join(store.Home, name))
	}
	if err := store.Update(func(*State) error { return nil }); err != nil {
		t.Fatal(err)
	}
	got, err := store.ReadSnapshot()
	if err != nil || got.Version != stateVersion || !reflect.DeepEqual(got.Burials, want.Burials) {
		t.Fatalf("v18 migration lost cold evidence: version=%d err=%v", got.Version, err)
	}
	future := []byte(`{"version":20,"storage":"sqlite","database":"store.db"}`)
	if err := os.WriteFile(store.statePath(), future, 0600); err != nil {
		t.Fatal(err)
	}
	_, err = store.Inspect()
	var newer newerStoreError
	if !errors.As(err, &newer) || newer.version != 20 {
		t.Fatalf("v20 fence was not rejected as future: %v", err)
	}
}

func BenchmarkSQLiteSparseUpdate10K(b *testing.B) {
	store := (Store{Home: b.TempDir()}).WithPersistentSQLite()
	items := make([]Item, 10000)
	for i := range items {
		items[i] = Item{ID: int64(i + 1), Rev: 1, Path: fmt.Sprintf("item-%d", i)}
	}
	if err := store.Update(func(st *State) error { st.Items, st.NextID = items, int64(len(items)+1); return nil }); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := int64(i%len(items) + 1)
		if err := store.UpdateHot(func(st *State) error {
			idx := st.Index(id)
			st.Items[idx].Rev++
			st.Items[idx].Note = "changed"
			return nil
		}); err != nil {
			b.Fatal(err)
		}
	}
}
