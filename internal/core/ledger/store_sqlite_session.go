package ledger

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"reflect"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/woodfishhhh/obituary/internal/core/internal/fileio"
)

var errSQLiteSessionFallback = errors.New("SQLite hot session requires the legacy transaction path")

type sqliteSession struct {
	mu      sync.Mutex
	db      *sql.DB
	state   State
	rows    sqliteRows
	modTime int64
}

var sqliteSessions = struct {
	sync.Mutex
	byHome map[string]*sqliteSession
}{byHome: make(map[string]*sqliteSession)}

var sqliteSessionLoads atomic.Int64

// sqliteSessionDecodeCount and clearSQLiteSessionsForTest keep the regression
// test focused on the production property without exposing test-only state.
func sqliteSessionDecodeCount() int64 { return sqliteSessionLoads.Load() }

func clearSQLiteSessionsForTest(home string) {
	sqliteSessions.Lock()
	delete(sqliteSessions.byHome, home)
	sqliteSessions.Unlock()
}

type sqliteSessionBaseline struct {
	items []Item
	hot   []byte
}

type sqliteRowChange struct {
	before *sqliteRow
	after  *sqliteRow
}

type sqliteRowDelta struct {
	changes map[sqliteKey]sqliteRowChange
}

func (d sqliteRowDelta) empty() bool { return len(d.changes) == 0 }

func (s Store) sqliteHotSession() (*sqliteSession, error) {
	if !s.sqliteSupported() {
		return nil, errSQLiteSessionFallback
	}
	sqliteSessions.Lock()
	defer sqliteSessions.Unlock()
	if session := sqliteSessions.byHome[s.Home]; session != nil {
		return session, nil
	}
	db, err := openSQLite(s.databasePath(), "rwc")
	if err != nil {
		return nil, fmtSQLiteSessionError("open", err)
	}
	st, rows, err := s.loadSQLiteSessionState(db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	info, err := os.Stat(s.statePath())
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	session := &sqliteSession{db: db, state: st, rows: rows, modTime: info.ModTime().UnixNano()}
	sqliteSessions.byHome[s.Home] = session
	return session, nil
}

func fmtSQLiteSessionError(op string, err error) error {
	return errors.New("SQLite hot session " + op + ": " + err.Error())
}

func (s Store) loadSQLiteSessionState(db *sql.DB) (State, sqliteRows, error) {
	ctx := s.Context()
	if err := checkSQLiteSchema(ctx, db); err != nil {
		return State{}, nil, err
	}
	state, rows, err := decodeSQLiteRows(ctx, db)
	if err != nil {
		return State{}, nil, err
	}
	sqliteSessionLoads.Add(1)
	state, events, err := s.normalizeState(state)
	if err != nil {
		return State{}, nil, err
	}
	if len(events) != 0 {
		return State{}, nil, errSQLiteSessionFallback
	}
	if state.Version != stateVersion {
		return State{}, nil, newerStoreError{state.Version}
	}
	return state, rows, nil
}

func (session *sqliteSession) refresh(s Store) error {
	info, err := os.Stat(s.statePath())
	if err != nil {
		return err
	}
	modTime := info.ModTime().UnixNano()
	if modTime == session.modTime {
		return nil
	}
	state, rows, err := s.loadSQLiteSessionState(session.db)
	if err != nil {
		return err
	}
	session.state, session.rows, session.modTime = state, rows, modTime
	return nil
}

func (session *sqliteSession) baseline() (sqliteSessionBaseline, error) {
	items := make([]Item, len(session.state.Items))
	for i, item := range session.state.Items {
		items[i] = cloneSQLiteItem(item)
	}
	hot := session.state
	hot.Items = nil
	data, err := marshalHot(hot)
	if err != nil {
		return sqliteSessionBaseline{}, err
	}
	return sqliteSessionBaseline{items: items, hot: data}, nil
}

func cloneSQLiteItem(item Item) Item {
	if item.Identity != nil {
		identity := *item.Identity
		item.Identity = &identity
	}
	if item.Container != nil {
		container := *item.Container
		if container.Target != nil {
			target := *container.Target
			container.Target = &target
		}
		item.Container = &container
	}
	if item.Volume != nil {
		volume := *item.Volume
		volume.Labels = maps.Clone(volume.Labels)
		volume.Options = maps.Clone(volume.Options)
		item.Volume = &volume
	}
	if item.Watch != nil {
		watch := *item.Watch
		item.Watch = &watch
	}
	return item
}

func (session *sqliteSession) delta(base sqliteSessionBaseline) (sqliteRowDelta, error) {
	hot := session.state
	hot.Items = nil
	currentHot, err := marshalHot(hot)
	if err != nil {
		return sqliteRowDelta{}, err
	}
	if !bytes.Equal(base.hot, currentHot) {
		current, err := stateSQLiteRows(session.state)
		if err != nil {
			return sqliteRowDelta{}, err
		}
		return sqliteRowsDelta(session.rows, current), nil
	}
	if len(base.items) == len(session.state.Items) {
		stable := true
		for i := range base.items {
			if base.items[i].ID != session.state.Items[i].ID {
				stable = false
				break
			}
		}
		if stable {
			delta := sqliteRowDelta{changes: make(map[sqliteKey]sqliteRowChange)}
			for i, old := range base.items {
				current := session.state.Items[i]
				if sameSQLiteItem(old, current) {
					continue
				}
				row, err := sqliteItemRow(current)
				if err != nil {
					return sqliteRowDelta{}, err
				}
				key := sqliteKey{kind: "items", key: formatSQLiteID(current.ID)}
				oldRow, exists := session.rows[key]
				if !exists {
					return sqliteRowDelta{}, errors.New("missing item row in SQLite hot session")
				}
				delta.changes[key] = sqliteRowChange{before: &oldRow, after: &row}
			}
			return delta, nil
		}
	}
	before := make(map[int64]Item, len(base.items))
	for _, item := range base.items {
		if _, exists := before[item.ID]; exists {
			return sqliteRowDelta{}, errors.New("duplicate item id in SQLite hot session")
		}
		before[item.ID] = item
	}
	after := make(map[int64]Item, len(session.state.Items))
	for _, item := range session.state.Items {
		if _, exists := after[item.ID]; exists {
			return sqliteRowDelta{}, errors.New("duplicate item id in SQLite hot session")
		}
		after[item.ID] = item
	}
	delta := sqliteRowDelta{changes: make(map[sqliteKey]sqliteRowChange)}
	for id, old := range before {
		key := sqliteKey{kind: "items", key: formatSQLiteID(id)}
		if _, exists := after[id]; !exists {
			oldRow, exists := session.rows[key]
			if !exists {
				return sqliteRowDelta{}, errors.New("missing item row in SQLite hot session")
			}
			delta.changes[key] = sqliteRowChange{before: &oldRow}
			continue
		}
		if reflect.DeepEqual(old, after[id]) {
			continue
		}
		row, err := sqliteItemRow(after[id])
		if err != nil {
			return sqliteRowDelta{}, err
		}
		oldRow, exists := session.rows[key]
		if !exists {
			return sqliteRowDelta{}, errors.New("missing item row in SQLite hot session")
		}
		delta.changes[key] = sqliteRowChange{before: &oldRow, after: &row}
	}
	for id, item := range after {
		if _, exists := before[id]; exists {
			continue
		}
		row, err := sqliteItemRow(item)
		if err != nil {
			return sqliteRowDelta{}, err
		}
		key := sqliteKey{kind: "items", key: formatSQLiteID(id)}
		delta.changes[key] = sqliteRowChange{after: &row}
	}
	return delta, nil
}

func sameSQLiteItem(a, b Item) bool {
	if a.Kind != b.Kind || a.ID != b.ID || a.Rev != b.Rev || a.Path != b.Path || a.IsDir != b.IsDir ||
		!a.CreatedAt.Equal(b.CreatedAt) || !a.ExpiresAt.Equal(b.ExpiresAt) || a.Force != b.Force || a.Note != b.Note ||
		a.Source != b.Source || a.Project != b.Project || a.Session != b.Session || a.WatchID != b.WatchID ||
		a.Attempts != b.Attempts || !a.NextAttempt.Equal(b.NextAttempt) || a.LastError != b.LastError {
		return false
	}
	if !reflect.DeepEqual(a.Identity, b.Identity) || !reflect.DeepEqual(a.Container, b.Container) || !reflect.DeepEqual(a.Volume, b.Volume) || !reflect.DeepEqual(a.Watch, b.Watch) {
		return false
	}
	return true
}

func formatSQLiteID(id int64) string { return strconv.FormatInt(id, 10) }

func sqliteItemRow(item Item) (sqliteRow, error) {
	payload, err := json.Marshal(item)
	if err != nil {
		return sqliteRow{}, err
	}
	return sqliteRow{payload: payload, position: item.IDAsPosition(), rev: formatSQLiteID(item.Rev)}, nil
}

func sqliteRowsDelta(before, after sqliteRows) sqliteRowDelta {
	delta := sqliteRowDelta{changes: make(map[sqliteKey]sqliteRowChange)}
	for key, old := range before {
		current, exists := after[key]
		if !exists {
			oldCopy := old
			delta.changes[key] = sqliteRowChange{before: &oldCopy}
			continue
		}
		if old.position == current.position && old.rev == current.rev && bytes.Equal(old.payload, current.payload) {
			continue
		}
		oldCopy, currentCopy := old, current
		delta.changes[key] = sqliteRowChange{before: &oldCopy, after: &currentCopy}
	}
	for key, current := range after {
		if _, exists := before[key]; exists {
			continue
		}
		currentCopy := current
		delta.changes[key] = sqliteRowChange{after: &currentCopy}
	}
	return delta
}

func writeSQLiteDelta(ctx context.Context, tx *sql.Tx, delta sqliteRowDelta) error {
	keys := make([]sqliteKey, 0, len(delta.changes))
	for key := range delta.changes {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].kind != keys[j].kind {
			return keys[i].kind < keys[j].kind
		}
		return keys[i].key < keys[j].key
	})
	for _, key := range keys {
		change := delta.changes[key]
		switch {
		case change.before != nil && change.after == nil:
			result, err := tx.ExecContext(ctx, "DELETE FROM ledger WHERE kind=? AND key=? AND rev=?", key.kind, key.key, change.before.rev)
			if err != nil {
				return err
			}
			if n, _ := result.RowsAffected(); n != 1 {
				return fmt.Errorf("ledger revision conflict deleting %s/%s", key.kind, key.key)
			}
		case change.before == nil && change.after != nil:
			if _, err := tx.ExecContext(ctx, "INSERT INTO ledger(kind,key,position,rev,payload) VALUES(?,?,?,?,?)", key.kind, key.key, change.after.position, change.after.rev, change.after.payload); err != nil {
				return fmt.Errorf("insert ledger row %s/%s: %w", key.kind, key.key, err)
			}
		case change.before != nil && change.after != nil:
			result, err := tx.ExecContext(ctx, "UPDATE ledger SET position=?,rev=?,payload=? WHERE kind=? AND key=? AND rev=?", change.after.position, change.after.rev, change.after.payload, key.kind, key.key, change.before.rev)
			if err != nil {
				return err
			}
			if n, _ := result.RowsAffected(); n != 1 {
				return fmt.Errorf("ledger revision conflict updating %s/%s", key.kind, key.key)
			}
		}
	}
	return nil
}

func inverseSQLiteDelta(delta sqliteRowDelta) sqliteRowDelta {
	inverse := sqliteRowDelta{changes: make(map[sqliteKey]sqliteRowChange, len(delta.changes))}
	for key, change := range delta.changes {
		inverse.changes[key] = sqliteRowChange{before: change.after, after: change.before}
	}
	return inverse
}

func applySQLiteDelta(rows sqliteRows, delta sqliteRowDelta) {
	for key, change := range delta.changes {
		if change.after == nil {
			delete(rows, key)
			continue
		}
		rows[key] = *change.after
	}
}

func sqliteDeltaReceiptID(delta sqliteRowDelta) string {
	keys := make([]sqliteKey, 0, len(delta.changes))
	for key := range delta.changes {
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
		change := delta.changes[key]
		_, _ = hash.Write([]byte(key.kind + "\x00" + key.key + "\x00"))
		if change.before != nil {
			_, _ = hash.Write([]byte(change.before.rev))
		}
		_, _ = hash.Write([]byte{'\x00'})
		if change.after != nil {
			_, _ = hash.Write([]byte(change.after.rev))
			_, _ = hash.Write(change.after.payload)
		}
		_, _ = hash.Write([]byte{'\x00'})
	}
	return fmt.Sprintf("delta:%x", hash.Sum(nil))
}

func (s Store) updateHotSQLiteSession(session *sqliteSession, fn func(*State) error) error {
	session.mu.Lock()
	defer session.mu.Unlock()
	if err := session.refresh(s); err != nil {
		return err
	}
	base, err := session.baseline()
	if err != nil {
		return err
	}
	guard := guardHotGraveyard(&session.state)
	boundary, err := hotBoundary(session.state)
	if err != nil {
		return err
	}
	if err := fn(&session.state); err != nil {
		return err
	}
	if err := checkHotMutation(session.state, boundary, guard); err != nil {
		return err
	}
	sort.Slice(session.state.Items, func(i, j int) bool { return session.state.Items[i].ExpiresAt.Before(session.state.Items[j].ExpiresAt) })
	if err := s.Context().Err(); err != nil {
		return err
	}
	delta, err := session.delta(base)
	if err != nil {
		return err
	}
	if delta.empty() {
		return nil
	}
	if err := s.commitSQLiteSession(session, delta); err != nil {
		return err
	}
	return nil
}

func (s Store) commitSQLiteSession(session *sqliteSession, delta sqliteRowDelta) error {
	ctx := s.Context()
	tx, err := session.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	rollback := true
	defer func() {
		if rollback {
			_ = tx.Rollback()
		}
	}()
	if err := writeSQLiteDelta(ctx, tx, delta); err != nil {
		return err
	}
	receiptID := sqliteDeltaReceiptID(delta)
	if _, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO receipts(id,result) VALUES(?,?)", receiptID, []byte("committed")); err != nil {
		return err
	}
	pendingSQLiteMu.Lock()
	pendingSQLite[s.Home] = session.state
	pendingSQLiteMu.Unlock()
	if err := commitSQLite(ctx, tx); err != nil {
		return fmt.Errorf("commit unconfirmed: %w", err)
	}
	rollback = false
	fenceErr := writeDurably(s.statePath(), sqliteFenceBytes(session.state))
	if fenceErr != nil && !fileio.IsCommitUnconfirmed(fenceErr) {
		if err := s.rollbackSQLiteDelta(ctx, session, delta, receiptID); err != nil {
			return errors.Join(fenceErr, fmt.Errorf("rollback unpublished SQLite commit: %w", err))
		}
	}
	pendingSQLiteMu.Lock()
	if fenceErr == nil || !fileio.IsCommitUnconfirmed(fenceErr) {
		delete(pendingSQLite, s.Home)
	}
	pendingSQLiteMu.Unlock()
	if fenceErr != nil {
		if fileio.IsCommitUnconfirmed(fenceErr) {
			return fenceErr
		}
		return fmt.Errorf("publish SQLite version fence: %w", fenceErr)
	}
	applySQLiteDelta(session.rows, delta)
	if info, err := os.Stat(s.statePath()); err == nil {
		session.modTime = info.ModTime().UnixNano()
	}
	if s.changed != nil {
		*s.changed = true
	}
	return nil
}

func (s Store) rollbackSQLiteDelta(ctx context.Context, session *sqliteSession, delta sqliteRowDelta, receiptID string) error {
	tx, err := session.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	rollback := true
	defer func() {
		if rollback {
			_ = tx.Rollback()
		}
	}()
	if err := writeSQLiteDelta(ctx, tx, inverseSQLiteDelta(delta)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM receipts WHERE id=?", receiptID); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	rollback = false
	return nil
}
