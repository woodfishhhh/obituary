package ledger

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Preserve the exact old bytes before the first current-version commit. Content-addressed
// backups survive interrupted retries and never replace different evidence.
func (s Store) prepareLedgerMigration(st *State) error {
	if st.Version >= stateVersion {
		return nil
	}
	data, err := readShared(s.statePath())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var header struct {
		Version int `json:"version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return err
	}
	if header.Version >= stateVersion {
		return nil
	}
	name := fmt.Sprintf("store.v%d-%x.json.bak", header.Version, sha256.Sum256(data))
	path := filepath.Join(s.Home, name)
	old, err := readShared(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read migration backup: %w", err)
	}
	if err == nil && !bytes.Equal(old, data) {
		return fmt.Errorf("migration backup differs from original ledger: %s", path)
	}
	// Reconfirm identical evidence too: a prior rename may have been visible
	// after a failed flush. The old ledger stays authoritative until success.
	if err := writeDurably(path, data); err != nil {
		return fmt.Errorf("preserve migration backup: %w", err)
	}
	QueueAudit(st, Event{Time: time.Now(), Action: "store-migrated", Path: s.statePath(),
		Detail: fmt.Sprintf("ledger v%d to v%d; original bytes retained in %s", header.Version, stateVersion, name)})
	return nil
}
