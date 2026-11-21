package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/woodfishhhh/obituary/internal/update"
	"github.com/woodfishhhh/obituary/internal/userdirs"
	"golang.org/x/sys/windows"
)

const updateOrigin = "OBIT_UPDATE_ORIGIN"
const updateReadyTimeout = 10 * time.Second
const updateExitTimeout = 30 * time.Second

func StartUpdater(ctx context.Context, exe, current string, relaunch bool) (update.ApplyResult, error) {
	home, err := userdirs.Data()
	if err != nil {
		return update.ApplyResult{}, err
	}
	if _, err := update.Target(home, exe, current); err != nil {
		return update.ApplyResult{}, err
	}
	config, err := userdirs.Config()
	if err != nil {
		return update.ApplyResult{}, err
	}
	worker, err := copyUpdater(home, exe)
	if err != nil {
		return update.ApplyResult{}, err
	}
	args := []string{"update", "--apply", "--wait-pid", strconv.Itoa(os.Getpid())}
	if relaunch {
		args = append(args, "--relaunch-gui")
	}
	cmd := exec.Command(worker, args...)
	cmd.Dir = home
	cmd.Env = append(os.Environ(), updateOrigin+"="+exe, "OBIT_HOME="+home, "OBIT_CONFIG_DIR="+config)
	detach(cmd)
	return launchUpdater(ctx, cmd, (*exec.Cmd).Start)
}

func copyUpdater(home, exe string) (string, error) {
	if err := os.MkdirAll(home, 0o700); err != nil {
		return "", err
	}
	source, err := os.Open(exe)
	if err != nil {
		return "", err
	}
	defer source.Close()
	service, err := Open()
	if err != nil {
		return "", err
	}
	dir, _, err := Engine(service).MkTemp(2*time.Hour, "obit-update", home, "")
	if err != nil {
		return "", fmt.Errorf("prepare updater copy: %w", err)
	}
	path := filepath.Join(dir, "obit.exe")
	w, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o700)
	if err != nil {
		return "", err
	}
	_, err = io.Copy(w, source)
	return path, errors.Join(err, w.Close())
}

func launchUpdater(ctx context.Context, cmd *exec.Cmd, start func(*exec.Cmd) error) (update.ApplyResult, error) {
	r, w, err := os.Pipe()
	if err != nil {
		return update.ApplyResult{}, err
	}
	defer r.Close()
	cmd.Stdout, cmd.Stderr = w, w
	err = start(cmd)
	w.Close()
	if err != nil {
		return update.ApplyResult{}, err
	}
	defer cmd.Process.Release()
	ctx, cancel := context.WithTimeout(ctx, updateReadyTimeout)
	defer cancel()
	result, err := readUpdaterReady(ctx, r)
	if err != nil {
		if ctx.Err() != nil {
			_ = cmd.Process.Kill()
		}
		return update.ApplyResult{}, fmt.Errorf("start update helper: %w", err)
	}
	return result, nil
}

func readUpdaterReady(ctx context.Context, r io.Reader) (update.ApplyResult, error) {
	type reply struct {
		result update.ApplyResult
		err    error
	}
	ready := make(chan reply, 1)
	go func() {
		var message struct {
			update.ApplyResult
			Error string
		}
		err := json.NewDecoder(io.LimitReader(r, 8192)).Decode(&message)
		if err == nil && message.Error != "" {
			err = errors.New(message.Error)
		}
		if err == nil && (message.TargetVersion == "" || message.LogPath == "") {
			err = errors.New("missing update handoff receipt")
		}
		ready <- reply{message.ApplyResult, err}
	}()
	select {
	case msg := <-ready:
		return msg.result, msg.err
	case <-ctx.Done():
		return update.ApplyResult{}, ctx.Err()
	}
}

func ApplyUpdate(ctx context.Context, home, exe, current string, waitPID int, relaunch bool, out io.Writer) error {
	if origin := os.Getenv(updateOrigin); origin != "" && waitPID > 0 {
		return runUpdater(ctx, home, origin, current, waitPID, relaunch, out)
	}
	if waitPID > 0 {
		wait, closeHandle, err := updaterWait(waitPID)
		if err != nil {
			return err
		}
		defer closeHandle()
		if err := wait(); err != nil {
			return err
		}
	}
	result, err := StartUpdater(ctx, exe, current, relaunch)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "Update to %s scheduled; progress log: %s\n", result.TargetVersion, result.LogPath)
	return err
}

func runUpdater(ctx context.Context, home, exe, current string, pid int, relaunch bool, out io.Writer) (err error) {
	var acknowledged bool
	defer func() {
		if err != nil && !acknowledged {
			_ = json.NewEncoder(out).Encode(struct{ Error string }{err.Error()})
		}
	}()
	if err := os.Unsetenv(updateOrigin); err != nil {
		return err
	}
	target, err := update.Target(home, exe, current)
	if err != nil {
		return err
	}
	wait, closeHandle, err := updaterWait(pid)
	if err != nil {
		return err
	}
	defer closeHandle()
	hooks := updateHooks(home, exe, relaunch, pid)
	hooks.Wait = wait
	hooks.Ready = func(result update.ApplyResult) error {
		err := json.NewEncoder(out).Encode(result)
		acknowledged = err == nil
		return err
	}
	_, err = update.Apply(ctx, home, exe, current, target, hooks)
	return err
}

func updateHooks(home, original string, relaunch bool, pid int) update.ApplyHooks {
	var restart bool
	hooks := update.ApplyHooks{
		Retire: retireUpdateFiles,
		Stop: func() error {
			restart = Running(home)
			return Stop(home)
		},
		Restart: func(exe string) error {
			if !restart || Running(home) {
				return nil
			}
			path := installedSibling(exe, original, daemonName)
			if err := startAt(home, path); err != nil {
				return fmt.Errorf("restart resident: %w", err)
			}
			return nil
		},
	}
	if relaunch {
		hooks.Relaunch = func(exe string) error {
			// An update that failed before the window was asked to close leaves
			// that window open, showing the error; a second one would be a copy.
			if processRunning(pid) {
				return nil
			}
			cmd := exec.Command(installedSibling(exe, original, "obit-gui.exe"))
			detach(cmd)
			if err := cmd.Start(); err != nil {
				return fmt.Errorf("relaunch GUI: %w", err)
			}
			return cmd.Process.Release()
		}
	}
	return hooks
}

func installedSibling(preferred, original, base string) string {
	path := filepath.Join(filepath.Dir(preferred), base)
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return filepath.Join(filepath.Dir(original), base)
	}
	return path
}

// processRunning reports whether pid is alive. The updater holds a handle to
// it for the whole run, so the number cannot have been reused.
func processRunning(pid int) bool {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	status, err := windows.WaitForSingleObject(h, 0)
	return err == nil && status == uint32(windows.WAIT_TIMEOUT)
}

func updaterWait(pid int) (func() error, func(), error) {
	if pid <= 0 || pid == os.Getpid() || uint64(pid) > uint64(^uint32(0)) {
		return nil, nil, fmt.Errorf("invalid update wait PID %d", pid)
	}
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return nil, nil, fmt.Errorf("open process %d: %w", pid, err)
	}
	wait := func() error {
		status, err := windows.WaitForSingleObject(h, uint32(updateExitTimeout/time.Millisecond))
		if err != nil {
			return fmt.Errorf("wait for process %d: %w", pid, err)
		}
		if status != windows.WAIT_OBJECT_0 {
			return fmt.Errorf("process %d did not exit within %s", pid, updateExitTimeout)
		}
		return nil
	}
	return wait, func() { windows.CloseHandle(h) }, nil
}
