package ledger

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/internal/fileio"
	_ "modernc.org/sqlite"
)

const sqliteDatabaseName = "store.db"

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS ledger (
  kind TEXT NOT NULL,
  key TEXT NOT NULL,
  position INTEGER NOT NULL,
  rev TEXT NOT NULL,
  payload BLOB NOT NULL,
  PRIMARY KEY(kind, key)
) WITHOUT ROWID;
CREATE TABLE IF NOT EXISTS receipts (
  id TEXT PRIMARY KEY,
  result BLOB NOT NULL
);
PRAGMA user_version = 19;
`

type sqliteKey struct {
	kind string
	key  string
}

type sqliteRow struct {
	payload  []byte
	rev      string
	position int
}

type sqliteRows map[sqliteKey]sqliteRow

type sqliteMeta struct {
	Graveyard         *GraveyardReference `json:"graveyard,omitempty"`
	PreviousGraveyard *GraveyardReference `json:"previousGraveyard,omitempty"`
	GraveyardIndex    *graveyardIndex     `json:"graveyardIndex,omitempty"`
	GraveyardGCCursor string              `json:"graveyardGCCursor,omitempty"`
	DataID            string              `json:"dataId,omitempty"`
	Version           int                 `json:"version"`
	NextID            int64               `json:"nextId"`
	Stats             Stats               `json:"stats"`
}

type sqliteFence struct {
	Version           int                 `json:"version"`
	Storage           string              `json:"storage"`
	Database          string              `json:"database"`
	DataID            string              `json:"dataId,omitempty"`
	NextID            int64               `json:"nextId,omitempty"`
	Graveyard         *GraveyardReference `json:"graveyard,omitempty"`
	PreviousGraveyard *GraveyardReference `json:"previousGraveyard,omitempty"`
	GraveyardIndex    *graveyardIndex     `json:"graveyardIndex,omitempty"`
}

var (
	commitSQLite = func(ctx context.Context, tx *sql.Tx) error { return tx.Commit() }

	pendingSQLiteMu sync.Mutex
	pendingSQLite   = map[string]State{}
)

func (s Store) databasePath() string { return filepath.Join(s.Home, sqliteDatabaseName) }

func sqliteFenceBytes(st State) []byte {
	data, _ := json.Marshal(sqliteFence{
		Version: stateVersion, Storage: "sqlite", Database: sqliteDatabaseName,
		DataID: st.DataID, NextID: st.NextID, Graveyard: st.Graveyard,
		PreviousGraveyard: st.PreviousGraveyard, GraveyardIndex: st.GraveyardIndex,
	})
	return append(data, '\n')
}

func sqliteFenceFor(data []byte) (sqliteFence, error) {
	var fence sqliteFence
	if err := json.Unmarshal(data, &fence); err != nil {
		return fence, err
	}
	return fence, nil
}

func (s Store) hasSQLiteFence() (bool, error) {
	data, err := readShared(s.statePath())
	if errors.Is(err, os.ErrNotExist) {
		if _, dbErr := os.Stat(s.databasePath()); dbErr == nil {
			return false, errors.New("SQLite ledger exists without its version fence")
		}
		return false, nil
	}
	if err != nil {
		return false, err
	}
	fence, err := sqliteFenceFor(data)
	if err != nil {
		return false, nil
	}
	if fence.Version > stateVersion {
		return false, newerStoreError{fence.Version}
	}
	if fence.Version != stateVersion {
		return false, nil
	}
	if fence.Storage != "sqlite" || fence.Database != sqliteDatabaseName {
		return false, errors.New("invalid SQLite version fence")
	}
	if _, err := os.Stat(s.databasePath()); err != nil {
		return false, fmt.Errorf("SQLite ledger is missing: %w", err)
	}
	return true, nil
}

func openSQLite(path, mode string) (*sql.DB, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	query := []string{"mode=" + mode, "_pragma=busy_timeout(0)"}
	if mode == "ro" {
		if _, err := os.Stat(path + "-wal"); errors.Is(err, os.ErrNotExist) {
			// A checkpointed database has no sidecar to observe. Immutable read
			// mode keeps diagnostics from creating a new SHM file.
			query = append(query, "immutable=1")
		}
	}
	if mode != "ro" {
		query = append(query, "_pragma=journal_mode(WAL)", "_pragma=synchronous(FULL)")
	}
	dsn := "file:" + filepath.ToSlash(abs) + "?" + strings.Join(query, "&")
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	return db, nil
}

func ensureSQLiteSchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, sqliteSchema); err != nil {
		return err
	}
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version != stateVersion {
		return fmt.Errorf("unsupported SQLite ledger version %d", version)
	}
	var journal string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journal); err != nil {
		return err
	}
	if strings.ToLower(journal) != "wal" {
		return fmt.Errorf("ledger requires SQLite WAL; got %s", journal)
	}
	var synchronous int
	if err := db.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&synchronous); err != nil {
		return err
	}
	if synchronous != 2 {
		return fmt.Errorf("ledger requires SQLite synchronous=FULL; got %d", synchronous)
	}
	return nil
}

func checkSQLiteSchema(ctx context.Context, db *sql.DB) error {
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return err
	}
	if version != stateVersion {
		return fmt.Errorf("unsupported SQLite ledger version %d", version)
	}
	var synchronous int
	if err := db.QueryRowContext(ctx, "PRAGMA synchronous").Scan(&synchronous); err != nil {
		return err
	}
	if synchronous != 2 {
		return fmt.Errorf("ledger requires SQLite synchronous=FULL; got %d", synchronous)
	}
	return nil
}

func sqliteFileVersion(path string) (int, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}
	db, err := openSQLite(path, "ro")
	if err != nil {
		return 0, err
	}
	defer db.Close()
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
}

func sqliteMetaOf(st State) sqliteMeta {
	return sqliteMeta{
		Graveyard: st.Graveyard, PreviousGraveyard: st.PreviousGraveyard,
		GraveyardIndex: st.GraveyardIndex, GraveyardGCCursor: st.GraveyardGCCursor,
		DataID: st.DataID, Version: st.Version, NextID: st.NextID, Stats: st.Stats,
	}
}

func appendSQLiteRows(rows sqliteRows, kind, key string, position int, rev int64, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	k := sqliteKey{kind: kind, key: key}
	if _, exists := rows[k]; exists {
		return fmt.Errorf("duplicate %s row %s", kind, key)
	}
	rows[k] = sqliteRow{payload: data, position: position, rev: strconv.FormatInt(rev, 10)}
	return nil
}

func occurrenceKey(value any, index int) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x:%d", sha256.Sum256(data), index), nil
}

func stateSQLiteRows(st State) (sqliteRows, error) {
	rows := make(sqliteRows)
	if err := appendSQLiteRows(rows, "meta", "state", 0, 0, sqliteMetaOf(st)); err != nil {
		return nil, err
	}
	for _, item := range st.Items {
		if err := appendSQLiteRows(rows, "items", strconv.FormatInt(item.ID, 10), item.IDAsPosition(), item.Rev, item); err != nil {
			return nil, err
		}
	}
	for i, proposal := range st.Proposals {
		if err := appendSQLiteRows(rows, "proposals", strconv.FormatInt(proposal.ID, 10), i, 0, proposal); err != nil {
			return nil, err
		}
	}
	for i, watch := range st.Watches {
		if err := appendSQLiteRows(rows, "watches", strconv.FormatInt(watch.ID, 10), i, watch.Rev, watch); err != nil {
			return nil, err
		}
	}
	for i, receipt := range st.AutoReceipts {
		if err := appendSQLiteRows(rows, "autoReceipts", strconv.FormatInt(receipt.ID, 10), i, 0, receipt); err != nil {
			return nil, err
		}
	}
	for i, rejection := range st.LeftoverRejections {
		key, err := occurrenceKey(rejection, i)
		if err != nil {
			return nil, err
		}
		if err := appendSQLiteRows(rows, "leftoverRejections", key, i, 0, rejection); err != nil {
			return nil, err
		}
	}
	for key, project := range st.Projects {
		if err := appendSQLiteRows(rows, "projects", key, 0, 0, project); err != nil {
			return nil, err
		}
	}
	for key, at := range st.ObservationReceipts {
		if err := appendSQLiteRows(rows, "observationReceipts", key, 0, 0, at); err != nil {
			return nil, err
		}
	}
	for i, intent := range st.ContainerIntents {
		key := intent.Token
		if key == "" {
			key, _ = occurrenceKey(intent, i)
		}
		if err := appendSQLiteRows(rows, "containerIntents", key, i, 0, intent); err != nil {
			return nil, err
		}
	}
	for i, exemption := range st.WatchExemptions {
		key, err := occurrenceKey(exemption, i)
		if err != nil {
			return nil, err
		}
		if err := appendSQLiteRows(rows, "watchExemptions", key, i, 0, exemption); err != nil {
			return nil, err
		}
	}
	for i, exemption := range st.AutoExemptions {
		key, err := occurrenceKey(exemption, i)
		if err != nil {
			return nil, err
		}
		if err := appendSQLiteRows(rows, "autoExemptions", key, i, 0, exemption); err != nil {
			return nil, err
		}
	}
	for i, event := range st.PendingAudit {
		key, err := occurrenceKey(event, i)
		if err != nil {
			return nil, err
		}
		if err := appendSQLiteRows(rows, "pendingAudit", key, i, 0, event); err != nil {
			return nil, err
		}
	}
	return rows, nil
}

func (it Item) IDAsPosition() int { return int(it.ID) }

func decodeSQLiteRows(ctx context.Context, db *sql.DB) (State, sqliteRows, error) {
	var st State
	rows := make(sqliteRows)
	query, err := db.QueryContext(ctx, "SELECT kind,key,position,rev,payload FROM ledger ORDER BY kind,position,key")
	if err != nil {
		return st, nil, err
	}
	defer query.Close()
	seenMeta := false
	for query.Next() {
		var kind, key, rev string
		var position int
		var payload []byte
		if err := query.Scan(&kind, &key, &position, &rev, &payload); err != nil {
			return st, nil, err
		}
		row := sqliteRow{payload: append([]byte(nil), payload...), position: position, rev: rev}
		rows[sqliteKey{kind, key}] = row
		switch kind {
		case "meta":
			if key != "state" {
				return st, nil, fmt.Errorf("unknown ledger metadata %s", key)
			}
			var meta sqliteMeta
			if err := json.Unmarshal(payload, &meta); err != nil {
				return st, nil, err
			}
			st.Graveyard, st.PreviousGraveyard, st.GraveyardIndex = meta.Graveyard, meta.PreviousGraveyard, meta.GraveyardIndex
			st.GraveyardGCCursor, st.DataID = meta.GraveyardGCCursor, meta.DataID
			st.Version, st.NextID, st.Stats = meta.Version, meta.NextID, meta.Stats
			seenMeta = true
		case "items":
			var item Item
			if err := json.Unmarshal(payload, &item); err != nil {
				return st, nil, err
			}
			if strconv.FormatInt(item.ID, 10) != key || strconv.FormatInt(item.Rev, 10) != rev {
				return st, nil, fmt.Errorf("item row %s revision disagrees with payload", key)
			}
			st.Items = append(st.Items, item)
		case "proposals":
			var proposal Proposal
			if err := json.Unmarshal(payload, &proposal); err != nil {
				return st, nil, err
			}
			if strconv.FormatInt(proposal.ID, 10) != key {
				return st, nil, fmt.Errorf("proposal row %s disagrees with payload", key)
			}
			st.Proposals = append(st.Proposals, proposal)
		case "watches":
			var watch WatchPolicy
			if err := json.Unmarshal(payload, &watch); err != nil {
				return st, nil, err
			}
			if strconv.FormatInt(watch.ID, 10) != key || strconv.FormatInt(watch.Rev, 10) != rev {
				return st, nil, fmt.Errorf("watch row %s revision disagrees with payload", key)
			}
			st.Watches = append(st.Watches, watch)
		case "autoReceipts":
			var receipt AutoReceipt
			if err := json.Unmarshal(payload, &receipt); err != nil {
				return st, nil, err
			}
			if strconv.FormatInt(receipt.ID, 10) != key {
				return st, nil, fmt.Errorf("auto receipt row %s disagrees with payload", key)
			}
			st.AutoReceipts = append(st.AutoReceipts, receipt)
		case "leftoverRejections":
			var value LeftoverRejection
			if err := json.Unmarshal(payload, &value); err != nil {
				return st, nil, err
			}
			st.LeftoverRejections = append(st.LeftoverRejections, value)
		case "projects":
			var value ProjectState
			if err := json.Unmarshal(payload, &value); err != nil {
				return st, nil, err
			}
			if st.Projects == nil {
				st.Projects = make(map[string]ProjectState)
			}
			st.Projects[key] = value
		case "observationReceipts":
			var value time.Time
			if err := json.Unmarshal(payload, &value); err != nil {
				return st, nil, err
			}
			if st.ObservationReceipts == nil {
				st.ObservationReceipts = make(map[string]time.Time)
			}
			st.ObservationReceipts[key] = value
		case "containerIntents", "watchExemptions", "autoExemptions", "pendingAudit":
			continue
		default:
			return st, nil, fmt.Errorf("unknown ledger row kind %s", kind)
		}
	}
	if err := query.Err(); err != nil {
		return st, nil, err
	}
	if !seenMeta {
		return st, nil, errors.New("missing ledger metadata state")
	}
	for _, kind := range []string{"containerIntents", "watchExemptions", "autoExemptions", "pendingAudit"} {
		keys := make([]sqliteKey, 0)
		for key := range rows {
			if key.kind == kind {
				keys = append(keys, key)
			}
		}
		sort.Slice(keys, func(i, j int) bool { return rows[keys[i]].position < rows[keys[j]].position })
		for _, key := range keys {
			row := rows[key]
			switch kind {
			case "containerIntents":
				var value ContainerIntent
				if err := json.Unmarshal(row.payload, &value); err != nil {
					return st, nil, err
				}
				st.ContainerIntents = append(st.ContainerIntents, value)
			case "watchExemptions":
				var value WatchExemption
				if err := json.Unmarshal(row.payload, &value); err != nil {
					return st, nil, err
				}
				st.WatchExemptions = append(st.WatchExemptions, value)
			case "autoExemptions":
				var value AutoExemption
				if err := json.Unmarshal(row.payload, &value); err != nil {
					return st, nil, err
				}
				st.AutoExemptions = append(st.AutoExemptions, value)
			case "pendingAudit":
				var value Event
				if err := json.Unmarshal(row.payload, &value); err != nil {
					return st, nil, err
				}
				st.PendingAudit = append(st.PendingAudit, value)
			}
		}
	}
	sort.SliceStable(st.Items, func(i, j int) bool { return st.Items[i].ExpiresAt.Before(st.Items[j].ExpiresAt) })
	sort.SliceStable(st.Proposals, func(i, j int) bool {
		return rows[sqliteKey{"proposals", strconv.FormatInt(st.Proposals[i].ID, 10)}].position < rows[sqliteKey{"proposals", strconv.FormatInt(st.Proposals[j].ID, 10)}].position
	})
	sort.SliceStable(st.Watches, func(i, j int) bool {
		return rows[sqliteKey{"watches", strconv.FormatInt(st.Watches[i].ID, 10)}].position < rows[sqliteKey{"watches", strconv.FormatInt(st.Watches[j].ID, 10)}].position
	})
	sort.SliceStable(st.AutoReceipts, func(i, j int) bool {
		return rows[sqliteKey{"autoReceipts", strconv.FormatInt(st.AutoReceipts[i].ID, 10)}].position < rows[sqliteKey{"autoReceipts", strconv.FormatInt(st.AutoReceipts[j].ID, 10)}].position
	})
	return st, rows, nil
}

func writeSQLiteRows(ctx context.Context, tx *sql.Tx, before, after sqliteRows) error {
	for key, old := range before {
		if _, ok := after[key]; ok {
			continue
		}
		result, err := tx.ExecContext(ctx, "DELETE FROM ledger WHERE kind=? AND key=? AND rev=?", key.kind, key.key, old.rev)
		if err != nil {
			return err
		}
		if n, _ := result.RowsAffected(); n != 1 {
			return fmt.Errorf("ledger revision conflict deleting %s/%s", key.kind, key.key)
		}
	}
	for key, row := range after {
		old, exists := before[key]
		if exists && old.position == row.position && old.rev == row.rev && bytes.Equal(old.payload, row.payload) {
			continue
		}
		if !exists {
			if _, err := tx.ExecContext(ctx, "INSERT INTO ledger(kind,key,position,rev,payload) VALUES(?,?,?,?,?)", key.kind, key.key, row.position, row.rev, row.payload); err != nil {
				return err
			}
			continue
		}
		result, err := tx.ExecContext(ctx, "UPDATE ledger SET position=?,rev=?,payload=? WHERE kind=? AND key=? AND rev=?", row.position, row.rev, row.payload, key.kind, key.key, old.rev)
		if err != nil {
			return err
		}
		if n, _ := result.RowsAffected(); n != 1 {
			return fmt.Errorf("ledger revision conflict updating %s/%s", key.kind, key.key)
		}
	}
	return nil
}

func sqliteRowsReceiptID(rows sqliteRows) string {
	keys := make([]sqliteKey, 0, len(rows))
	for key := range rows {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].kind != keys[j].kind {
			return keys[i].kind < keys[j].kind
		}
		return keys[i].key < keys[j].key
	})
	hash := sha256.New()
	for _, key := range keys {
		row := rows[key]
		_, _ = hash.Write([]byte(key.kind))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(key.key))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(row.rev))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write([]byte(strconv.Itoa(row.position)))
		_, _ = hash.Write([]byte{0})
		_, _ = hash.Write(row.payload)
		_, _ = hash.Write([]byte{0})
	}
	return fmt.Sprintf("%x", hash.Sum(nil))
}

func (s Store) loadSQLiteHot() (State, []Event, error) {
	db, err := openSQLite(s.databasePath(), "ro")
	if err != nil {
		return State{}, nil, fmt.Errorf("open SQLite ledger: %w", err)
	}
	defer db.Close()
	ctx := s.Context()
	if err := checkSQLiteSchema(ctx, db); err != nil {
		return State{}, nil, err
	}
	state, _, err := decodeSQLiteRows(ctx, db)
	if err != nil {
		return State{}, nil, err
	}
	if state.Version != stateVersion {
		return State{}, nil, newerStoreError{state.Version}
	}
	state, events, err := s.normalizeState(state)
	return state, events, err
}

func (s Store) saveSQLite(state State, migrating bool) error {
	ctx := s.Context()
	if version, err := sqliteFileVersion(s.databasePath()); err != nil {
		return fmt.Errorf("inspect SQLite ledger: %w", err)
	} else if version > stateVersion {
		return newerStoreError{version}
	} else if version != 0 && version != stateVersion {
		return fmt.Errorf("unsupported SQLite ledger version %d", version)
	}
	db, err := openSQLite(s.databasePath(), "rwc")
	if err != nil {
		return fmt.Errorf("open SQLite ledger: %w", err)
	}
	defer db.Close()
	if err := ensureSQLiteSchema(ctx, db); err != nil {
		return err
	}
	before := make(sqliteRows)
	if _, err := os.Stat(s.databasePath()); err == nil {
		if current, rows, readErr := decodeSQLiteRows(ctx, db); readErr == nil && current.Version == stateVersion {
			before = rows
		}
	}
	after, err := stateSQLiteRows(state)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	rollback := true
	defer func() {
		if rollback {
			_ = tx.Rollback()
		}
	}()
	if migrating {
		if _, err := tx.ExecContext(ctx, "DELETE FROM ledger"); err != nil {
			return err
		}
		before = make(sqliteRows)
	}
	if err := writeSQLiteRows(ctx, tx, before, after); err != nil {
		return err
	}
	receiptID := sqliteRowsReceiptID(after)
	if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO receipts(id,result) VALUES(?,?)", receiptID, []byte("committed")); err != nil {
		return err
	}
	if migrating {
		if err := writeDurably(s.statePath(), sqliteFenceBytes(state)); err != nil {
			if !fileio.IsCommitUnconfirmed(err) {
				return err
			}
			if err := commitSQLite(ctx, tx); err != nil {
				return fmt.Errorf("commit unconfirmed: %w", err)
			}
			rollback = false
			return err
		}
	} else {
		pendingSQLiteMu.Lock()
		pendingSQLite[s.Home] = state
		pendingSQLiteMu.Unlock()
		fenceErr := writeDurably(s.statePath(), sqliteFenceBytes(state))
		pendingSQLiteMu.Lock()
		delete(pendingSQLite, s.Home)
		pendingSQLiteMu.Unlock()
		if fenceErr != nil && !fileio.IsCommitUnconfirmed(fenceErr) {
			return fenceErr
		}
		if err := commitSQLite(ctx, tx); err != nil {
			return fmt.Errorf("commit unconfirmed: %w", err)
		}
		rollback = false
		if fenceErr != nil {
			return fenceErr
		}
		return nil
	}
	if err := commitSQLite(ctx, tx); err != nil {
		return fmt.Errorf("commit unconfirmed: %w", err)
	}
	rollback = false
	return nil
}

func (s Store) stagedSQLiteState() (State, bool) {
	pendingSQLiteMu.Lock()
	defer pendingSQLiteMu.Unlock()
	state, ok := pendingSQLite[s.Home]
	return state, ok
}

func (s Store) sqliteModTime() time.Time {
	info, err := os.Stat(s.statePath())
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}
