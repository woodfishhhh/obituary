package app

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/woodfishhhh/obituary/internal/userdirs"
	"golang.org/x/sys/windows"
)

const (
	daemonName  = "obitd.exe"
	quietPeriod = 10 * time.Minute // repeat an unchanged error at most this often
)

func stopPath(home string) string { return filepath.Join(home, "obitd.stop") }

// Running reports whether a daemon is serving home.
func Running(home string) bool {
	name, err := daemonMutexName(home)
	if err != nil {
		return false
	}
	h, err := windows.OpenMutex(windows.SYNCHRONIZE, false, name)
	if err != nil {
		return false
	}
	windows.CloseHandle(h)
	return true
}

// DaemonPath is obitd.exe next to the running executable.
func DaemonPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	path := filepath.Join(filepath.Dir(exe), daemonName)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("%s not found next to %s", daemonName, exe)
	}
	return path, nil
}

// Start launches the daemon for home in the background unless it is already running.
func Start(home string) error {
	if Running(home) {
		return nil
	}
	path, err := DaemonPath()
	if err != nil {
		return err
	}
	cmd := exec.Command(path, daemonArgs(home)...)
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", path, err)
	}
	_ = cmd.Process.Release()
	return waitFor(func() bool { return Running(home) }, 5*time.Second, "daemon did not start")
}

// daemonArgs pins what obitd must serve: the data directory and the
// configuration directory as this process resolved them, so a runner started
// from a shell with OBIT_HOME or OBIT_CONFIG_DIR set keeps using them.
func daemonArgs(home string) []string {
	args := []string{"--home", home}
	if dir, err := userdirs.Config(); err == nil {
		args = append(args, "--config-dir", dir)
	}
	return args
}

func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW,
	}
}

// RunOnce starts a detached `obitd --once` that does one round of work for
// home and exits. It does not wait.
func RunOnce(home string) error {
	path, err := DaemonPath()
	if err != nil {
		return err
	}
	cmd := exec.Command(path, append([]string{"--once"}, daemonArgs(home)...)...)
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", path, err)
	}
	return cmd.Process.Release()
}

// SpawnSelf starts this executable again, detached and without a window,
// with args; hooks use it to hand slow work to the background.
func SpawnSelf(args ...string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, args...)
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// Stop asks a running daemon to exit and waits for it.
func Stop(home string) error {
	if !Running(home) {
		return clearStopRequest(home)
	}
	check, closeHandle, verifyErr := daemonExitCheck(home)
	if closeHandle != nil {
		defer closeHandle()
	}
	if err := os.WriteFile(stopPath(home), nil, 0o600); err != nil {
		return err
	}
	if err := waitFor(func() bool { return !Running(home) }, 10*time.Second, "daemon did not stop"); err != nil {
		return err
	}
	if verifyErr != nil {
		if errors.Is(verifyErr, os.ErrNotExist) {
			verifyErr = fmt.Errorf("daemon stopped; legacy resident has no shutdown receipt; cannot verify exit code: %w", verifyErr)
		}
		return errors.Join(verifyErr, clearStopRequest(home))
	}
	if err := check(); err != nil {
		var pending *daemonExitPending
		if errors.As(err, &pending) {
			return err
		}
		return errors.Join(err, clearStopRequest(home))
	}
	return clearStopRequest(home)
}

func clearStopRequest(home string) error {
	// The watcher can see creation before WriteFile closes its handle. The
	// writer removes the request after the resident exits, avoiding a Windows
	// sharing violation in the resident's shutdown path.
	err := os.Remove(stopPath(home))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func waitFor(cond func() bool, limit time.Duration, msg string) error {
	for deadline := time.Now().Add(limit); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if cond() {
			return nil
		}
	}
	return errors.New(msg)
}

// errorLog repeats an unchanged message at most once per quietPeriod.
type errorLog struct {
	logger *log.Logger
	last   map[string]string
	at     map[string]time.Time
}

func (e *errorLog) report(key string, err error) {
	msg := err.Error()
	if e.last[key] == msg && time.Since(e.at[key]) < quietPeriod {
		return
	}
	e.last[key], e.at[key] = msg, time.Now()
	e.logger.Printf("%s: %v", key, err)
}

func (e *errorLog) clear(key string) { delete(e.last, key) }
