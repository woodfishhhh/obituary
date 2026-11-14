// Package app wires the core service to Windows and controls the obitd daemon.
package app

import (
	"errors"
	"path/filepath"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/docker"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"github.com/woodfishhhh/obituary/internal/rules"
	"github.com/woodfishhhh/obituary/internal/userdirs"
	"github.com/woodfishhhh/obituary/internal/watch"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

// Open builds the service for the current user's data directory.
func Open() (*core.Service, error) {
	home, err := userdirs.Data()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	svc := &core.Service{
		Docker: docker.CLI{},
		Store:  core.Store{Home: home}, Guard: Guard(home), FS: winfs.FS{},
		Mode: cfg.Graveyard.Mode, Retention: cfg.Graveyard.Retention(), WatchStableWindow: cfg.Watch.Stability(), SessionSummary: cfg.Hooks.SessionSummary,
		SessionCleanup: cfg.Hooks.SessionCleanup, ProposalMode: cfg.Proposals.Mode,
		Owner: auto.Owner, OwnerBatch: auto.OwnerBatch, PrepareOwners: auto.PrepareOwners,
	}
	svc.Recheck = Engine(svc).RecheckRules
	svc.AfterChange = syncCurrentExplorer
	return svc, nil
}

// Engine wraps the service with rule-based registration; trusted project
// rules are pinned in the data directory so edits require renewed consent.
func Engine(svc *core.Service) *auto.Engine {
	return &auto.Engine{Svc: svc, Trust: rules.TrustStore{Path: filepath.Join(svc.Store.Home, "trusted_rules.json")}}
}

// configured takes one fresh configuration snapshot, as the resident can
// outlive a GUI configuration edit, without mutating the service shared with
// the folder watcher. A malformed configuration stops the work.
func configured(svc *core.Service) (*core.Service, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	svc = svc.WithContext(svc.Store.Context())
	svc.Mode, svc.Retention = cfg.Graveyard.Mode, cfg.Graveyard.Retention()
	svc.WatchStableWindow = cfg.Watch.Stability()
	svc.ProposalMode = cfg.Proposals.Mode
	return svc, nil
}

// ReplayObservations absorbs the writes hooks queued while the ledger was
// busy, without the rest of a tick.
func ReplayObservations(svc *core.Service) error {
	svc, err := configured(svc)
	if err != nil {
		return err
	}
	return Engine(svc).ReplayObservations()
}

// Tick drains hook writes before reaping, in both resident and one-shot runs.
func Tick(svc *core.Service) ([]core.Outcome, []core.GraveResult, error) {
	paused, err := Paused(svc.Store.Home)
	if err != nil || paused {
		return nil, nil, err
	}
	// Before the configuration is read: it may switch automatic registration off.
	migrateErr := migrateAutoOff(svc)
	svc, err = configured(svc)
	if err != nil {
		return nil, nil, err
	}
	// Hooks and other processes commit ledger changes and defer the Explorer
	// refresh to this tick, whose own sweep may change nothing. Notify once
	// when the ledger differs from the one Explorer labels were last synced
	// against; an idle tick stays silent.
	defer func() {
		if svc.AfterChange != nil && svc.Store.Context().Err() == nil && explorerStale(svc.Store.Home) {
			svc.AfterChange(svc)
		}
	}()
	queuedErr := Engine(svc).ReplayObservations()
	proposalErr := Engine(svc).RefreshProposals()
	watchErr := watch.Reconcile(svc)
	outcomes, purged, err := svc.Tick()
	if errors.Is(err, core.ErrSweepBusy) {
		err = nil // another worker owns this round; callers must not log an error
	}
	vanished, vanishedErr := svc.SettleVanished()
	if errors.Is(vanishedErr, core.ErrSweepBusy) {
		vanishedErr = nil
	}
	outcomes = append(outcomes, vanished...)
	diagnostics, diagnosticErr := svc.RegistrationLocationDiagnostics()
	diagnosticErr = errors.Join(diagnosticErr, svc.Store.Record(diagnostics...))
	// Obituary's own backups in the data directory, which nothing can bury.
	backupErr := errors.Join(svc.Store.PruneMigrationBackups(time.Now()), integrate.PruneBackups(svc.Store.Home, time.Now()))
	return outcomes, purged, joinTickErrors(proposalErr, migrateErr, queuedErr, watchErr, err, vanishedErr, diagnosticErr, backupErr)
}
