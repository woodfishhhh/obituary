package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
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
	err := managedAttempt(home, time.Now, run)
	if err == nil {
		_ = recordSupervisionReason(home, "stopped normally")
	}
	return err
}

type supervision struct {
	Open, Abandoned bool
	Failures        []time.Time
	Reason          string
	LastAttempt     time.Time
	LastExitCode    *int
	LaunchFailed    bool
}

// A failed Start has no process or exit status. Only Wait observes a child exit.
type trayLaunchError struct{ cause error }

func (e *trayLaunchError) Error() string { return "could not launch tray: " + e.cause.Error() }
func (e *trayLaunchError) Unwrap() error { return e.cause }

func runTrayProcess(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return &trayLaunchError{err}
	}
	return cmd.Wait()
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

func logSupervisionFailure(home string, err error) {
	if err == nil {
		return
	}
	f, openErr := os.OpenFile(filepath.Join(home, "obitd.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if openErr != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(time.Now().Format(time.RFC3339) + " supervisor: " + err.Error() + string([]byte{10}))
}

func supervisionDetails(s supervision) string {
	detail := fmt.Sprintf("failures=%d/%d", len(s.Failures), crashLimit)
	if !s.LastAttempt.IsZero() {
		detail += ", last attempt=" + s.LastAttempt.Format(time.RFC3339)
	}
	if s.LastExitCode != nil {
		detail += fmt.Sprintf(", last exit code=%d", *s.LastExitCode)
	}
	if s.Reason != "" {
		detail += ", reason=" + s.Reason
	}
	return detail
}

func (s supervision) save(home string) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return core.WriteMetadata(filepath.Join(home, supervisionFile), data)
}

func recordSupervisionReason(home, reason string) error {
	s, err := readSupervision(home)
	if err != nil {
		return err
	}
	s.Reason = reason
	return s.save(home)
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
		s.LastExitCode = nil
		s.failed(at, "previous resident did not close its receipt")
	}
	if s.Abandoned {
		return s.save(home)
	}
	s.Open, s.LaunchFailed = true, false
	s.LastAttempt = at
	if err := s.save(home); err != nil {
		return err
	}
	err = run()
	s.finishAttempt(now(), err)
	return errors.Join(err, s.save(home))
}

func (s *supervision) finishAttempt(at time.Time, err error) {
	s.Open = false
	s.LastExitCode = nil
	var launch *trayLaunchError
	var exit *exec.ExitError
	switch {
	case errors.As(err, &launch):
		s.LaunchFailed, s.Reason = true, launch.Error()
	case err != nil:
		s.failed(at, err.Error())
		if errors.As(err, &exit) {
			code := exit.ExitCode()
			s.LastExitCode = &code
		}
	default:
		code := 0
		s.LastExitCode, s.Reason = &code, ""
	}
}
