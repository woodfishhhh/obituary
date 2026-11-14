package ledger

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
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

// BackupLifespan is how long Obituary keeps a backup it made before changing
// its ledger or the user's configuration.
const BackupLifespan = 30 * 24 * time.Hour

var migrationBackup = regexp.MustCompile(`^store\.v(\d+)-([0-9a-f]{64})\.json\.bak$`)

// PruneMigrationBackups keeps the newest migration backup and deletes each
// older one after BackupLifespan, once its bytes still match the hash in its
// name: that proves it is the backup this store wrote.
func (s Store) PruneMigrationBackups(now time.Time) error {
	entries, err := os.ReadDir(s.Home)
	if err != nil {
		return err
	}
	type backup struct {
		name, hash string
		version    int
		modified   time.Time
	}
	var found []backup
	for _, entry := range entries {
		m := migrationBackup.FindStringSubmatch(entry.Name())
		if m == nil || !entry.Type().IsRegular() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		version, _ := strconv.Atoi(m[1])
		found = append(found, backup{entry.Name(), m[2], version, info.ModTime()})
	}
	slices.SortFunc(found, func(a, b backup) int {
		return cmp.Or(cmp.Compare(b.version, a.version), b.modified.Compare(a.modified))
	})
	var errs []error
	for _, b := range found[min(1, len(found)):] {
		if now.Sub(b.modified) < BackupLifespan {
			continue
		}
		path := filepath.Join(s.Home, b.name)
		data, err := readShared(path)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if fmt.Sprintf("%x", sha256.Sum256(data)) == b.hash {
			errs = append(errs, os.Remove(path))
		}
	}
	return errors.Join(errs...)
}
