package integrate

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

// projectBackupName is obit init's copy of a project file, named by the hash
// of the file's path, in the data directory's integration-backups folder.
var projectBackupName = regexp.MustCompile(`^[0-9a-f]{64}\.bak(\.[0-9]+)?$`)

// PruneBackups deletes those copies once they are older than the lifespan of
// Obituary's own backups. The data directory keeps them out of the graveyard.
func PruneBackups(home string, now time.Time) error {
	dir := filepath.Join(home, "integration-backups")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var errs []error
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !projectBackupName.MatchString(entry.Name()) {
			continue
		}
		if info, err := entry.Info(); err == nil && now.Sub(info.ModTime()) >= ledger.BackupLifespan {
			errs = append(errs, os.Remove(filepath.Join(dir, entry.Name())))
		}
	}
	return errors.Join(errs...)
}

func projectBackup(s Scope, path string) (Change, error) {
	path, err := userdirs.ReceiptPath(path, "integration-backups", "")
	if err != nil {
		return Change{}, err
	}
	return readIntegrationChange(dataScope(s), path)
}

func (p *integrationTarget) backupChange(ch Change, data []byte, write bool) error {
	if ch.backup == nil {
		return p.backup(data, write)
	}
	b, err := openIntegrationTarget(*ch.backup, write)
	if err != nil {
		return err
	}
	defer func() { b.root.Close() }()
	if write {
		if err := b.lockHostDirectory(); err != nil {
			return err
		}
	}
	return b.backup(data, write)
}
