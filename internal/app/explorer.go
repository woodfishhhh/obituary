package app

import (
	"log"
	"os"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/explorer"
	"github.com/woodfishhhh/obituary/internal/i18n"
)

// Watcher/GUI services outlive configuration edits, so every notification
// reads the current snapshot. A bad configuration preserves existing labels.
func syncCurrentExplorer(svc *lifecycle.Service) {
	cfg, err := config.Load()
	if err != nil {
		logExplorerError(svc, err)
		return
	}
	syncExplorer(svc, cfg)
}

func syncExplorer(svc *lifecycle.Service, cfg config.Config) {
	lang := i18n.Resolve(cfg.UI.Language, i18n.SystemLanguage)
	// Stamp the input snapshot, not a ledger or language changed during sync.
	stamp := explorerStamp(svc.Store.Home, lang)
	options := explorer.Options{Enabled: cfg.Explorer.Enabled, InGit: cfg.Explorer.InGit, Everywhere: cfg.Explorer.Everywhere, Language: lang}
	var err error
	if options.Enabled && !options.Everywhere {
		options.Folders, err = explorerFolders()
	}
	if err == nil {
		err = explorer.Sync(svc, options)
	}
	if err != nil {
		logExplorerError(svc, err)
		return
	}
	writeExplorerStamp(svc.Store.Home, stamp)
}

// Same bound as hook.log: a folder that keeps failing adds a line per sync.
const explorerLogLimit = 512 << 10

func logExplorerError(svc *lifecycle.Service, err error) {
	path := filepath.Join(svc.Store.Home, "explorer.log")
	if info, statErr := os.Stat(path); statErr == nil && info.Size() > explorerLogLimit {
		_ = os.Rename(path, path+".1") // a concurrent writer keeps the old file; retried next time
	}
	f, openErr := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if openErr != nil {
		log.Printf("Explorer labels: %v; log: %v", err, openErr)
		return
	}
	defer f.Close()
	log.New(f, "", log.LstdFlags).Printf("Explorer labels: %v", err)
}
