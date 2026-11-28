// Package app wires the core service to Windows and controls the obitd daemon.
package app

import (
	"errors"
	"path/filepath"
	"time"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/core/extres"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/docker"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"github.com/woodfishhhh/obituary/internal/rules"
	"github.com/woodfishhhh/obituary/internal/userdirs"
	"github.com/woodfishhhh/obituary/internal/watch"
	"github.com/woodfishhhh/obituary/internal/winfs"
)

// Open builds the service for the current user's data directory.
func Open() (*lifecycle.Service, error) {
	home, err := userdirs.Data()
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	dockerClient := docker.CLI{}
	svc := &lifecycle.Service{
		Docker: dockerClient,
		Providers: map[string]extres.Provider{
			ledger.KindContainer:    docker.NewContainerProvider(dockerClient),
			ledger.KindDockerVolume: docker.NewVolumeProvider(dockerClient),
		},
		Store: ledger.Store{Home: home}, Guard: Guard(home), FS: winfs.FS{},
		Mode: cfg.Graveyard.Mode, Retention: cfg.Graveyard.Retention(), GraveyardMaxBytes: cfg.Graveyard.MaxBytes(),
		WatchStableWindow: cfg.Watch.Stability(), WatchFileProposals: cfg.Watch.FileExpiry == "propose", SessionSummary: cfg.Hooks.SessionSummary,
		SessionCleanup: cfg.Hooks.SessionCleanup, ProposalMode: cfg.Proposals.Mode,
		Owner: auto.Owner, OwnerBatch: auto.OwnerBatch, PrepareOwners: auto.PrepareOwners,
	}
	svc.Recheck = Engine(svc).RecheckRules
	svc.AfterChange = syncCurrentExplorer
	return svc, nil
}

// Engine wraps the service with rule-based registration; trusted project
// rules are pinned in the data directory so edits require renewed consent.
func Engine(svc *lifecycle.Service) *auto.Engine {
	return &auto.Engine{Svc: svc, Trust: rules.TrustStore{Path: filepath.Join(svc.Store.Home, "trusted_rules.json")}}
}

// Configured takes one fresh configuration snapshot, as long-lived processes
// (resident, MCP, GUI) can outlive a configuration edit, without mutating the
// shared service. A malformed configuration stops the work.
func Configured(svc *lifecycle.Service) (*lifecycle.Service, config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, cfg, err
	}
	svc = svc.WithContext(svc.Store.Context())
	svc.Mode, svc.Retention = cfg.Graveyard.Mode, cfg.Graveyard.Retention()
	svc.GraveyardMaxBytes = cfg.Graveyard.MaxBytes()
	svc.WatchStableWindow = cfg.Watch.Stability()
	svc.WatchFileProposals = cfg.Watch.FileExpiry == "propose"
	svc.SessionSummary = cfg.Hooks.SessionSummary
	svc.SessionCleanup = cfg.Hooks.SessionCleanup
	svc.ProposalMode = cfg.Proposals.Mode
	return svc, cfg, nil
}

func currentGraveyardMaxBytes(svc *lifecycle.Service) int64 {
	if cfg, err := config.Load(); err == nil {
		return cfg.Graveyard.MaxBytes()
	}
	return svc.GraveyardMaxBytes
}

// ReplayObservations absorbs the writes hooks queued while the ledger was
// busy, without the rest of a tick.
func ReplayObservations(svc *lifecycle.Service) error {
	svc, _, err := Configured(svc)
	if err != nil {
		return err
	}
	return Engine(svc).ReplayObservations()
}

// Tick drains hook writes before reaping, in both resident and one-shot runs.
func Tick(svc *lifecycle.Service) ([]lifecycle.Outcome, []lifecycle.GraveResult, error) {
	paused, err := Paused(svc.Store.Home)
	if err != nil || paused {
		return nil, nil, err
	}
	// Before the configuration is read: it may switch automatic registration off.
	migrateErr := migrateAutoOff(svc)
	svc, cfg, err := Configured(svc)
	if err != nil {
		return nil, nil, err
	}
	// Hooks and other processes commit ledger changes and defer the Explorer
	// refresh to this tick, whose own sweep may change nothing. Notify once
	// when the ledger or language differs from what Explorer labels were last synced
	// against; an idle tick stays silent.
	defer func() {
		if svc.AfterChange != nil && svc.Store.Context().Err() == nil && explorerStale(svc.Store.Home, cfg.UI.Language) {
			svc.AfterChange(svc)
		}
	}()
	queuedErr := Engine(svc).ReplayObservations()
	proposalErr := Engine(svc).RefreshProposals()
	watchErr := watch.Reconcile(svc)
	outcomes, purged, err := svc.Tick()
	if errors.Is(err, lifecycle.ErrSweepBusy) {
		err = nil // another worker owns this round; callers must not log an error
	}
	vanished, vanishedErr := svc.SettleVanished()
	if errors.Is(vanishedErr, lifecycle.ErrSweepBusy) {
		vanishedErr = nil
	}
	outcomes = append(outcomes, vanished...)
	diagnostics, diagnosticErr := svc.RegistrationLocationDiagnostics()
	diagnosticErr = errors.Join(diagnosticErr, svc.Store.Record(diagnostics...))
	// Obituary's own backups in the data directory, which nothing can bury.
	backupErr := errors.Join(svc.Store.PruneMigrationBackups(time.Now()), integrate.PruneBackups(svc.Store.Home, time.Now()))
	return outcomes, purged, joinTickErrors(proposalErr, migrateErr, queuedErr, watchErr, err, vanishedErr, diagnosticErr, backupErr)
}
