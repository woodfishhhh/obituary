package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/integrate"
)

// autoOffMarker records that this data directory went through the 1.3 change
// once; a user who turns automatic registration back on is never switched off
// again.
const autoOffMarker = "migrated-1.3-auto-off"

// migrateAutoOff turns automatic registration off on an install that had it
// on, the safer default since 1.3, and spares what it had registered among
// dependencies, build output and caches: the files stay, without a lifespan.
func migrateAutoOff(svc *core.Service) error {
	marker := filepath.Join(svc.Store.Home, autoOffMarker)
	if _, err := os.Stat(marker); err == nil || !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := switchAutoOff(svc); err != nil {
		return fmt.Errorf("turn automatic registration off: %w", err)
	}
	if err := spareDependencies(svc); err != nil {
		return fmt.Errorf("spare dependencies and build output: %w", err)
	}
	return os.WriteFile(marker, nil, 0o600)
}

func switchAutoOff(svc *core.Service) error {
	path, err := config.Path()
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return nil // no file: the default is off
	}
	cfg, err := config.Load()
	if err != nil || cfg.Proposals.Mode == config.ProposalsOff {
		return err
	}
	ch, err := integrate.PlanProposalModeConfig(path, config.ProposalsOff)
	if err != nil {
		return err
	}
	backups, err := integrate.ApplyWithBackups([]integrate.Change{ch})
	if err != nil {
		return err
	}
	return errors.Join(ExpireBackups(svc, backups), svc.Store.Record(core.Event{Time: time.Now(), Action: "settings", Path: path,
		Detail: "automatic registration is now off by default (was " + cfg.Proposals.Mode + "); turn it on in Settings or with [proposals] mode"}))
}

func spareDependencies(svc *core.Service) error {
	st, err := svc.Store.ReadHot()
	if err != nil {
		return err
	}
	var refs []string
	for _, it := range st.Items {
		if it.Source != "watch" && it.Automatic() && auto.NeverAutomatic(it.Project, it.Path) != "" {
			refs = append(refs, fmt.Sprintf("#%d", it.ID))
		}
	}
	if len(refs) == 0 {
		return nil
	}
	results, err := svc.Cancel(refs)
	for _, r := range results {
		if r.Error != "" {
			err = errors.Join(err, fmt.Errorf("%s: %s", r.Ref, r.Error))
		}
	}
	return err
}

// ExpireBackups gives backups Obituary wrote beside a user's files the
// lifespan of Obituary's own backups; the graveyard holds them for a while
// after.
func ExpireBackups(svc *core.Service, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	results, err := svc.Add(paths, core.AddOptions{ExpiresAt: time.Now().Add(ledger.BackupLifespan), Source: "init", Note: "Obituary backup"})
	for _, r := range results {
		if r.Error != "" {
			err = errors.Join(err, fmt.Errorf("%s: %s", r.Ref, r.Error))
		}
	}
	return err
}
