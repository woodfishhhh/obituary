package integrate

import "github.com/woodfishhhh/obituary/internal/userdirs"

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
