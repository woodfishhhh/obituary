package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

const crashLimit = 3
const crashWindow = 15 * time.Minute
const supervisionFile = "obitd.supervision.json"

func ManagedTray(home string, run func() error) error {
	if paused, err := Paused(home); paused || err != nil {
		return err
	}
	return managedAttempt(home, time.Now, run)
}

type supervision struct {
	Open, Abandoned bool
	Failures        []time.Time
	Reason          string
}

func readSupervision(home string) (supervision, error) {
	var s supervision
	data, err := core.ReadMetadata(filepath.Join(home, supervisionFile))
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err == nil {
		err = json.Unmarshal(data, &s)
	}
	return s, err
}

func (s supervision) save(home string) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return core.WriteMetadata(filepath.Join(home, supervisionFile), data)
}

func (s *supervision) prune(now time.Time) {
	s.Failures = slices.DeleteFunc(s.Failures, func(t time.Time) bool { return !t.After(now.Add(-crashWindow)) })
	s.Abandoned = len(s.Failures) >= crashLimit
}

func (s *supervision) failed(now time.Time, reason string) {
	s.Open = false
	s.Reason = reason
	s.Failures = append(s.Failures, now)
	s.prune(now)
}

// The lock spans the attempt: only its successor can count an unclosed receipt.
func managedAttempt(home string, now func() time.Time, run func() error) error {
	if err := os.MkdirAll(home, 0700); err != nil {
		return err
	}
	unlock, ok, err := core.TryLockExclusive(filepath.Join(home, "obitd.attempt.lock"))
	if !ok || err != nil {
		return err
	}
	defer unlock()
	s, err := readSupervision(home)
	if err != nil {
		return err
	}
	at := now()
	s.prune(at)
	if s.Open {
		s.failed(at, "previous resident did not close its receipt")
	}
	if s.Abandoned {
		return s.save(home)
	}
	s.Open = true
	if err := s.save(home); err != nil {
		return err
	}
	err = run()
	s.Open = false
	if err != nil {
		s.failed(now(), err.Error())
	} else {
		s.Reason = ""
	}
	return errors.Join(err, s.save(home))
}
