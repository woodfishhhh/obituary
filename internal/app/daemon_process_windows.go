package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
)

type daemonProcess struct {
	PID     uint32 `json:"pid"`
	Created uint64 `json:"created"`
	Closed  bool   `json:"closed"`
	Error   string `json:"error,omitempty"`
}

func processCreated(h windows.Handle) (uint64, error) {
	var created, exited, kernel, user windows.Filetime
	err := windows.GetProcessTimes(h, &created, &exited, &kernel, &user)
	return uint64(created.HighDateTime)<<32 | uint64(created.LowDateTime), err
}

func writeDaemonProcess(home string, closed bool, runErr error) error {
	created, err := processCreated(windows.CurrentProcess())
	if err != nil {
		return err
	}
	p := daemonProcess{PID: uint32(os.Getpid()), Created: created, Closed: closed}
	if runErr != nil {
		p.Error = runErr.Error()
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(home, "obitd.process.json"), data, 0o600)
}

func readDaemonProcess(home string) (daemonProcess, error) {
	var p daemonProcess
	data, err := os.ReadFile(filepath.Join(home, "obitd.process.json"))
	if err == nil {
		err = json.Unmarshal(data, &p)
	}
	return p, err
}

// Wait and query exit status through the process handle. os.Process does not
// expose creation-time validation or a bounded wait, so use x/sys here.
// Pin before requesting stop and check creation time against PID reuse.
func daemonExitCheck(home string) (func() error, func(), error) {
	p, err := readDaemonProcess(home)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot verify daemon process: %w", err)
	}
	h, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, p.PID)
	if err != nil {
		return nil, nil, err
	}
	closeHandle := func() { windows.CloseHandle(h) }
	created, err := processCreated(h)
	if err != nil {
		closeHandle()
		return nil, nil, err
	}
	if created != p.Created {
		closeHandle()
		return nil, nil, fmt.Errorf("daemon process identity changed")
	}
	return func() error { return verifyDaemonExit(home, p, h) }, closeHandle, nil
}

type daemonExitPending struct{ cause error }

func (e *daemonExitPending) Error() string { return e.cause.Error() }
func (e *daemonExitPending) Unwrap() error { return e.cause }

func waitDaemonExit(h windows.Handle) error {
	status, err := windows.WaitForSingleObject(h, uint32((10*time.Second)/time.Millisecond))
	if err != nil {
		return &daemonExitPending{err}
	}
	if status != windows.WAIT_OBJECT_0 {
		return &daemonExitPending{fmt.Errorf("daemon process did not exit")}
	}
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("daemon exited with code %d", code)
	}
	return nil
}

func verifyDaemonExit(home string, p daemonProcess, h windows.Handle) error {
	// In-process Serve users have no separate process to wait for.
	if p.PID != uint32(os.Getpid()) {
		if err := waitDaemonExit(h); err != nil {
			return err
		}
	}
	closed, err := readDaemonProcess(home)
	if err != nil {
		return err
	}
	if closed.PID != p.PID || closed.Created != p.Created || !closed.Closed || closed.Error != "" {
		return fmt.Errorf("daemon shutdown was not confirmed: %s", closed.Error)
	}
	return nil
}
