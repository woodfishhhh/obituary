//go:build windows

package app

import (
	"log"
	"os"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/explorer"
)

// Watcher/GUI services outlive configuration edits, so every notification
// reads the current snapshot. A bad configuration preserves existing labels.
func syncCurrentExplorer(svc *core.Service) {
	// Stamp the ledger as it was before syncing: a change committed during
	// the sync leaves the stamp stale, so the next tick syncs again.
	stamp := ledgerStamp(svc.Store.Home)
	cfg, err := config.Load()
	if err != nil {
		logExplorerError(svc, err)
		return
	}
	if syncExplorer(svc, cfg.Explorer) {
		writeExplorerStamp(svc.Store.Home, stamp)
	}
}

func syncExplorer(svc *core.Service, cfg config.Explorer) bool {
	options := explorer.Options{Enabled: cfg.Enabled, InGit: cfg.InGit, Everywhere: cfg.Everywhere}
	var err error
	if cfg.Enabled && !cfg.Everywhere {
		options.Folders, err = explorerFolders()
	}
	if err == nil {
		err = explorer.Sync(svc, options)
	}
	if err != nil {
		logExplorerError(svc, err)
		return false
	}
	return true
}

func logExplorerError(svc *core.Service, err error) {
	f, openErr := os.OpenFile(filepath.Join(svc.Store.Home, "explorer.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if openErr != nil {
		log.Printf("Explorer labels: %v; log: %v", err, openErr)
		return
	}
	defer f.Close()
	log.New(f, "", log.LstdFlags).Printf("Explorer labels: %v", err)
}
