package ledger

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

// UpdateItem updates a single existing revision; a missing or superseded item is left alone.
func (s Store) UpdateItem(id, rev int64, change func(*Item)) error {
	err := s.withLock("store.lock", true, func() error {
		fence, err := s.readSQLiteFence()
		if err != nil {
			return err
		}
		if fence == nil {
			return errSQLiteSessionFallback
		}
		if s.persistentSQLite {
			session, err := s.sqliteHotSession()
			if err != nil {
				return err
			}
			session.mu.Lock()
			defer session.mu.Unlock()
			// A subsequent full-state callback must reload its own projection.
			session.rows = nil
			return s.updateSQLiteItem(session.db, *fence, id, rev, change)
		}
		db, err := openSQLite(s.databasePath(), "rw")
		if err != nil {
			return err
		}
		defer db.Close()
		return s.updateSQLiteItem(db, *fence, id, rev, change)
	})
	if !errors.Is(err, errSQLiteSessionFallback) {
		return err
	}
	return s.UpdateHot(func(st *State) error {
		if i := st.Index(id); i >= 0 && st.Items[i].Rev == rev {
			change(&st.Items[i])
			if st.Items[i].ID != id {
				return errors.New("item update cannot change its ID")
			}
		}
		return nil
	})
}

func (s Store) sqliteRow(db *sql.DB, kind, key string) (r sqliteRow, err error) {
	err = db.QueryRowContext(s.Context(), "SELECT position,rev,payload FROM ledger WHERE kind=? AND key=?", kind, key).Scan(&r.position, &r.rev, &r.payload)
	return r, err
}

func (s Store) updateSQLiteItem(db *sql.DB, fence sqliteFence, id, rev int64, change func(*Item)) error {
	if err := checkSQLiteSchema(s.Context(), db); err != nil {
		return err
	}
	metadata, err := s.sqliteRow(db, "meta", "state")
	if err != nil {
		return err
	}
	var st State
	if err := json.Unmarshal(metadata.payload, (*plainState)(&st)); err != nil {
		return err
	}
	if st.Version != stateVersion {
		return fmt.Errorf("unsupported SQLite state version %d", st.Version)
	}
	if err := s.checkSQLiteFence(db, fence, st); err != nil {
		return err
	}
	key := sqliteKey{"items", formatSQLiteID(id)}
	before, err := s.sqliteRow(db, key.kind, key.key)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	var item Item
	if err := json.Unmarshal(before.payload, &item); err != nil {
		return err
	}
	if item.ID != id || formatSQLiteID(item.Rev) != before.rev {
		return fmt.Errorf("item row %s revision disagrees with payload", key.key)
	}
	if item.Rev != rev {
		return nil
	}
	change(&item)
	if item.ID != id {
		return errors.New("item update cannot change its ID")
	}
	if err := validateContainerState(State{Items: []Item{item}}); err != nil {
		return err
	}
	after, err := sqliteItemRow(item)
	if err != nil {
		return err
	}
	if bytes.Equal(before.payload, after.payload) {
		return nil
	}
	// A short-lived writer notifies the resident. The resident need not wake itself for an unchanged fence.
	publishFence := !s.persistentSQLite || !reflect.DeepEqual(fence, sqliteFenceOf(st))
	return s.commitSQLiteDelta(db, st, sqliteRowDelta{map[sqliteKey]sqliteRowChange{key: {before: &before, after: &after}}}, publishFence)
}
