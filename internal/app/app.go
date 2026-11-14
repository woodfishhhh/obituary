// Package app wires the core service to Windows and controls the obitd daemon.
package app

import (
	"errors"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/docker"
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

// Tick drains hook writes before reaping, in both resident and one-shot runs.
func Tick(svc *core.Service) ([]core.Outcome, []core.GraveResult, error) {
	paused, err := Paused(svc.Store.Home)
	if err != nil || paused {
		return nil, nil, err
	}
	// The resident can outlive a GUI configuration edit. Take one fresh
	// configuration snapshot per tick without mutating the service shared
	// with the folder watcher. A malformed configuration stops this tick.
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	svc = svc.WithContext(svc.Store.Context())
	svc.Mode, svc.Retention = cfg.Graveyard.Mode, cfg.Graveyard.Retention()
	svc.WatchStableWindow = cfg.Watch.Stability()
	svc.ProposalMode = cfg.Proposals.Mode
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
	diagnostics, diagnosticErr := svc.RegistrationLocationDiagnostics()
	diagnosticErr = errors.Join(diagnosticErr, svc.Store.Record(diagnostics...))
	return outcomes, purged, joinTickErrors(proposalErr, queuedErr, watchErr, err, diagnosticErr)
}
