package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
)

// Named events are session-local like the resident mutex. No control file or timer
// wakes a healthy supervisor: os/exec waits on the actual child process handle.
func supervisorEventName(home, suffix string) (*uint16, error) {
	name, err := daemonMutexName(home)
	if err != nil {
		return nil, err
	}
	return windows.UTF16PtrFromString(windows.UTF16PtrToString(name) + suffix)
}

func supervisorEvent(home, suffix string) (windows.Handle, bool, error) {
	name, err := supervisorEventName(home, suffix)
	if err != nil {
		return 0, false, err
	}
	h, err := windows.CreateEvent(nil, 1, 0, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		windows.CloseHandle(h)
		return 0, false, nil
	}
	return h, err == nil, err
}

func signalSupervisor(home, suffix string, signal bool) (bool, error) {
	name, err := supervisorEventName(home, suffix)
	if err != nil {
		return false, err
	}
	access := uint32(windows.SYNCHRONIZE)
	if signal {
		access = windows.EVENT_MODIFY_STATE
	}
	h, err := windows.OpenEvent(access, false, name)
	if errors.Is(err, windows.ERROR_FILE_NOT_FOUND) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(h)
	if signal {
		err = windows.SetEvent(h)
	}
	return true, err
}

const supervisorSuffix = "-supervisor"
const retrySuffix = "-backoff"

func Supervise(home string) (result error) {
	if err := os.MkdirAll(home, 0700); err != nil {
		return err
	}
	stop, owner, err := supervisorEvent(home, supervisorSuffix)
	if !owner || err != nil {
		return err
	}
	defer windows.CloseHandle(stop)
	receipt := filepath.Join(home, "obitd.supervisor.json")
	if err := writeProcessReceipt(receipt, false, nil); err != nil {
		return err
	}
	defer func() { result = errors.Join(result, writeProcessReceipt(receipt, true, result)) }()
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	launch := func() error {
		return runTrayProcess(exec.Command(exe, append([]string{"--tray"}, daemonArgs(home)...)...))
	}
	for {
		stopped, err := windows.WaitForSingleObject(stop, 0)
		if err != nil || stopped == windows.WAIT_OBJECT_0 {
			if err == nil {
				_ = recordSupervisionReason(home, "stopped intentionally")
				logSupervisionFailure(home, errors.New("resident supervision stopped intentionally"))
			}
			return err
		}
		if paused, err := Paused(home); paused || err != nil {
			if err == nil {
				_ = recordSupervisionReason(home, "paused")
				logSupervisionFailure(home, errors.New("resident supervision paused"))
			}
			return err
		}
		err = managedAttempt(home, time.Now, launch)
		if err != nil {
			logSupervisionFailure(home, err)
		}
		if err == nil {
			_ = recordSupervisionReason(home, "stopped normally")
			logSupervisionFailure(home, errors.New("resident stopped normally"))
			return nil
		}
		state, readErr := readSupervision(home)
		if readErr != nil || state.Open || state.LaunchFailed || len(state.Failures) == 0 {
			return errors.Join(err, readErr)
		}
		if state.Abandoned {
			logSupervisionFailure(home, fmt.Errorf("resident supervision gave up after %d failures in %s: %s", len(state.Failures), crashWindow, state.Reason))
			return nil
		}
		retry, waitErr := waitSupervisorRetry(home, stop)
		if waitErr != nil || !retry {
			return waitErr
		}
	}
}

func waitSupervisorRetry(home string, stop windows.Handle) (bool, error) {
	cancel, ok, err := supervisorEvent(home, retrySuffix)
	if err != nil || !ok {
		return false, err
	}
	defer windows.CloseHandle(cancel)
	if paused, err := Paused(home); paused || err != nil {
		return false, err
	}
	status, err := windows.WaitForMultipleObjects([]windows.Handle{stop, cancel}, false, 60_000)
	if err != nil {
		return false, err
	}
	switch status {
	case uint32(windows.WAIT_TIMEOUT):
		return true, nil
	case windows.WAIT_OBJECT_0, windows.WAIT_OBJECT_0 + 1:
		return false, nil
	default:
		return false, fmt.Errorf("unexpected supervisor wait result %d", status)
	}
}

func cancelPendingRestart(home string) error {
	_, err := signalSupervisor(home, retrySuffix, true)
	return err
}

func SupervisorProblem(home string) (string, error) {
	if paused, err := Paused(home); err != nil {
		return "", err
	} else if paused {
		return "resident supervision is paused; fallback maintenance is paused too", nil
	}
	s, err := readSupervision(home)
	if err != nil {
		return "", err
	}
	if s.Abandoned {
		return "resident supervision gave up after 3 failures in 15 minutes (" + supervisionDetails(s) + ")", nil
	}
	if s.LaunchFailed {
		return "resident supervisor could not launch the tray (" + supervisionDetails(s) + ")", nil
	}
	if s.Reason == "paused" {
		return "resident supervision is paused; fallback maintenance is paused too (" + supervisionDetails(s) + ")", nil
	}
	if s.Reason == "stopped intentionally" || s.Reason == "stopped normally" {
		return "resident daemon was intentionally stopped; fallback schedule remains independent (" + supervisionDetails(s) + ")", nil
	}
	retry, err := signalSupervisor(home, retrySuffix, false)
	if err != nil {
		return "", err
	}
	alive, err := signalSupervisor(home, supervisorSuffix, false)
	if err != nil {
		return "", err
	}
	if alive && retry {
		return "resident supervisor is waiting to retry the tray in 60 seconds (" + supervisionDetails(s) + ")", nil
	}
	if alive {
		return "", nil
	}
	if _, err := os.Stat(stopPath(home)); err == nil {
		return "resident daemon was intentionally stopped; fallback schedule remains independent", nil
	}
	p, err := readProcessReceipt(filepath.Join(home, "obitd.supervisor.json"))
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	if !p.Closed {
		tray := "tray is not running"
		if Running(home) {
			tray = "tray is still running without its supervisor"
		}
		return "resident supervisor stopped unexpectedly; " + tray + "; no restart until the next login; fallback schedule is independent (" + supervisionDetails(s) + ")", nil
	}
	if p.Error != "" {
		return "resident supervisor exited: " + p.Error + " (" + supervisionDetails(s) + ")", nil
	}
	return "", nil
}
