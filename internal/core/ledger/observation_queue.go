package ledger

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

func (s Store) observationDir() string { return filepath.Join(s.Home, "observations") }

// ObservationDir is where hooks queue the writes they could not record.
func (s Store) ObservationDir() string { return s.observationDir() }

func observationName(token string) string {
	return fmt.Sprintf("%x.json", sha256.Sum256([]byte(token)))
}

var errInvalidObservation = errors.New("invalid queued observation")

// ProbeObservationStore avoids spending the hook's entire budget on a ledger
// that is already busy. Actual reads/writes still take their ordinary locks.
func (s Store) ProbeObservationStore() error {
	if err := os.MkdirAll(s.Home, 0o700); err != nil {
		return err
	}
	release, ok, err := tryLockFile(filepath.Join(s.Home, "store.lock"), false)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("lock store.lock: %w", context.DeadlineExceeded)
	}
	release()
	return nil
}

// DeferObservations reuses the observation handoff, with one committed file per
// request. Producers never acquire store.lock or the replay/migration lock.
// Errors stay outside the hook protocol to preserve its response format.
// The existing os.Root durability primitive supplies the file/rename flushes
// required for this Windows handoff.
func (s Store) DeferObservations(rows []Observation) error {
	if len(rows) == 0 {
		return nil
	}
	if err := os.MkdirAll(s.observationDir(), 0o700); err != nil {
		return err
	}
	var failures error
	for _, row := range rows {
		if row.Identity == nil {
			failures = errors.Join(failures, errors.New("deferred observation requires a verified identity"))
			continue
		}
		name := filepath.Join(s.observationDir(), observationName(row.Token))
		if _, err := os.Stat(name); err == nil {
			continue
		} else if !errors.Is(err, os.ErrNotExist) {
			failures = errors.Join(failures, err)
			continue
		}
		row.Path = filepath.Clean(row.Path)
		if err := writeObservation(name, row); err != nil {
			failures = errors.Join(failures, err)
		}
	}
	return failures
}

func validateObservation(row Observation) error {
	if !filepath.IsAbs(row.Path) || row.Session == "" || row.Token == "" || row.ObservedAt.IsZero() {
		return errors.New("deferred observation requires an absolute path, session, token and time")
	}
	return nil
}

func writeObservation(path string, row Observation) error {
	if err := validateObservation(row); err != nil {
		return err
	}
	data, err := json.Marshal(row)
	if err != nil {
		return err
	}
	if len(data) > observationBytes {
		return errors.New("observation exceeds 1 MiB; update not saved")
	}
	return writeObservationFile(path, data)
}

// Unique temporary names also isolate duplicate producers of the same request.
// os.Root and replaceDurably already implement the platform-specific flushes;
// no independent queue filesystem or database is needed.
func writeObservationFile(path string, data []byte) error {
	root, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer root.Close()
	tmp := filepath.Base(path) + "." + rand.Text() + ".tmp"
	f, err := root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer func() { _ = root.Remove(tmp) }() // only this invocation's temporary file
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := syncFile(f); err != nil {
		f.Close()
		return err
	}
	return replaceDurably(root, tmp, filepath.Base(path), f)
}

func readObservation(path string) (Observation, error) {
	var row Observation
	f, err := os.Open(path)
	if err != nil {
		return row, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, observationBytes+1))
	if err != nil {
		return row, err
	}
	if len(data) > observationBytes {
		return row, fmt.Errorf("%w: exceeds 1 MiB", errInvalidObservation)
	}
	if err := json.Unmarshal(data, &row); err != nil {
		return row, fmt.Errorf("%w: %w", errInvalidObservation, err)
	}
	if err := validateObservation(row); err != nil {
		return row, fmt.Errorf("%w: %w", errInvalidObservation, err)
	}
	return row, nil
}

// DeferredObservations returns committed requests in event-time order. Invalid
// files are reported separately by inspection and quarantined by the consumer;
// one damaged request cannot prevent healthy requests from being replayed.
func (s Store) DeferredObservations() ([]Observation, error) {
	entries, err := os.ReadDir(s.observationDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rows []Observation
	var failures error
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		row, err := readObservation(filepath.Join(s.observationDir(), entry.Name()))
		if err == nil && entry.Name() != observationName(row.Token) {
			err = errors.New("observation request ID does not match its filename")
		}
		if err != nil {
			failures = errors.Join(failures, fmt.Errorf("%s: %w", entry.Name(), err))
			continue
		}
		rows = append(rows, row)
	}
	slices.SortFunc(rows, func(a, b Observation) int {
		if c := a.ObservedAt.Compare(b.ObservedAt); c != 0 {
			return c
		}
		return strings.Compare(a.Token, b.Token)
	})
	return rows, failures
}

// RetryObservation is called by the sole replay consumer. Producers never
// modify an existing request, so they cannot reset its failure budget.
func (s Store) RetryObservation(row Observation, now time.Time, cause error) (bool, error) {
	row.Attempts++
	row.LastError = cause.Error()
	if len(row.LastError) > 256 {
		end := 256
		for !utf8.RuneStart(row.LastError[end]) {
			end--
		}
		row.LastError = row.LastError[:end]
	}
	path := filepath.Join(s.observationDir(), observationName(row.Token))
	if err := writeObservation(path, row); err != nil {
		return false, err
	}
	if row.Attempts < ObservationMaxAttempts && now.Sub(row.ObservedAt) < ObservationMaxAge {
		return false, nil
	}
	return true, s.quarantineObservation(observationName(row.Token), cause)
}

// AcknowledgeObservation only removes the exact request. Ledger receipts make
// a crash before this removal safe, including a replay after spare or reject.
func (s Store) AcknowledgeObservation(row Observation) error {
	err := os.Remove(filepath.Join(s.observationDir(), observationName(row.Token)))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s Store) HasDeferredObservations() bool {
	if info, err := os.Stat(filepath.Join(s.Home, "observations.jsonl")); err == nil && info.Size() > 0 {
		return true
	}
	entries, _ := os.ReadDir(s.observationDir())
	return slices.ContainsFunc(entries, func(entry os.DirEntry) bool { return strings.HasSuffix(entry.Name(), ".json") })
}

// WithObservationReplay serializes consumers without making producers wait.
// The OS releases this lock after a crash; requests are never renamed to an
// in-flight state, so there is no lease or abandoned-claim recovery protocol.
func (s Store) WithObservationReplay(replay func() error) error {
	if !s.HasDeferredObservations() {
		return nil
	}
	release, ok, err := TryLockExclusive(filepath.Join(s.Home, "observations.lock"))
	if err != nil || !ok {
		return err
	}
	defer release()
	err = s.migrateObservations()
	err = errors.Join(err, s.quarantineInvalidObservations())
	return errors.Join(err, replay())
}

func (s Store) quarantineObservation(name string, cause error) error {
	dead := filepath.Join(s.observationDir(), "dead")
	if err := os.MkdirAll(dead, 0o700); err != nil {
		return err
	}
	// Preserve the failed input and its reason for doctor/manual inspection.
	if err := s.Record(Event{Time: time.Now(), Action: "observation dropped", Path: name, Detail: cause.Error()}); err != nil {
		return err
	}
	root, err := os.OpenRoot(s.observationDir())
	if err != nil {
		return err
	}
	defer root.Close()
	f, err := root.OpenFile(name, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	if err := replaceDurably(root, name, filepath.Join("dead", name), f); err != nil {
		return err
	}
	deadRoot, err := os.OpenRoot(dead)
	if err != nil {
		return err
	}
	defer deadRoot.Close()
	return syncParentDirectory(deadRoot)
}

func (s Store) quarantineInvalidObservations() error {
	entries, err := os.ReadDir(s.observationDir())
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var failures error
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		row, err := readObservation(filepath.Join(s.observationDir(), entry.Name()))
		if err == nil && entry.Name() != observationName(row.Token) {
			err = fmt.Errorf("%w: request ID does not match its filename", errInvalidObservation)
		}
		if err != nil {
			failures = errors.Join(failures, err)
			if errors.Is(err, errInvalidObservation) {
				failures = errors.Join(failures, s.quarantineObservation(entry.Name(), err))
			}
		}
	}
	return failures
}
