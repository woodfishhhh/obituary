package project

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// ExcludeExplorerLabels uses Git's own metadata path (also for worktrees),
// like Ensure. It leaves user exclusions and their order intact.
func (p Project) ExcludeExplorerLabels() error {
	out, err := p.git("rev-parse", "--git-path", "info/exclude")
	if err != nil {
		return err
	}
	path := strings.TrimSpace(string(out))
	if !filepath.IsAbs(path) {
		path = filepath.Join(p.Root, path)
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "desktop.ini" {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	prefix := ""
	if len(data) > 0 && !bytes.HasSuffix(data, []byte("\n")) {
		prefix = "\n"
	}
	_, err = f.WriteString(prefix + "# Obituary Explorer display names\ndesktop.ini\n")
	return err
}
