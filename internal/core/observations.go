package core

import (
	"crypto/rand"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/pathutil"
)

const ObservationMaxAge = ledger.ObservationMaxAge
const ObservationMaxAttempts = ledger.ObservationMaxAttempts

type Observation = ledger.Observation

// CaptureObservations pins the event before any ledger-lock wait. An absent
// object has nothing to replay; an unreadable identity cannot authorize a
// later object at that path either.
func (s *Service) CaptureObservations(paths []string, session string) ([]Observation, error) {
	var rows []Observation
	var failures error
	for _, path := range paths {
		path, err := pathutil.Abs(path)
		if err != nil {
			failures = errors.Join(failures, err)
			continue
		}
		id, err := s.FS.Identify(path)
		if errors.Is(err, ErrNotFound) || errors.Is(err, ErrUnsafe) {
			continue
		}
		if err != nil {
			failures = errors.Join(failures, fmt.Errorf("capture %s: %w", path, err))
			continue
		}
		rows = append(rows, Observation{Path: filepath.Clean(path), Session: session, Token: rand.Text(), Identity: &id, ObservedAt: s.now()})
	}
	return rows, failures
}

// CheckObservation checks the captured object, including whether a missing
// name is on the same online volume. Writers call it while holding store.lock.
func (s *Service) CheckObservation(row *Observation) error {
	if row == nil {
		return nil
	}
	if row.Identity == nil {
		return fmt.Errorf("observation lacks an identity: %w", ErrUnsafe)
	}
	path, err := pathutil.Abs(row.Path)
	if err != nil {
		return err
	}
	got, err := s.FS.Identify(path)
	if errors.Is(err, ErrNotFound) && !s.missingOnOnlineVolume(path, err, row.Identity) {
		// Do not wrap ErrNotFound: an offline/replaced volume is retryable.
		return fmt.Errorf("observed volume unavailable or changed for %s: %v", path, err) //nolint:errorlint // Preserve retry instead of dropping an offline volume's observation.
	}
	if err != nil {
		return err
	}
	if got.Serial != row.Identity.Serial {
		return fmt.Errorf("observed volume changed for %s; original volume may be offline", row.Path)
	}
	if got != *row.Identity {
		return fmt.Errorf("observed object at %s changed; replacement left untouched: %w", row.Path, ErrUnsafe)
	}
	return nil
}
