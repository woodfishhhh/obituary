//go:build linux || darwin

package doctor

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"golang.org/x/sys/unix"
)

func diagnoseStartup(string, string) DoctorCheck {
	return DoctorCheck{Name: "tray-startup", Status: "INFO", Detail: "tray startup and Explorer labels are Windows-only"}
}

func diagnoseRunner(home string) DoctorCheck {
	c := DoctorCheck{Name: "runner", Status: "WARN", Detail: "no user timer serves this data directory; obit daemon enable installs one; without systemd run obitd --once from cron"}
	if cmdutil.RunnerInstalled() {
		target, err := cmdutil.RunnerTarget()
		if err != nil {
			c.Detail = err.Error()
		} else if cmdutil.RunnerServes(home) {
			c.Status, c.Detail = "PASS", "user timer serves this data directory: "+target
		}
	}
	return c
}

func diagnoseData(home string) DoctorCheck {
	c := DoctorCheck{Name: "data", Status: "PASS", Detail: "data directory is accessible: " + home}
	info, err := os.Stat(home)
	if errors.Is(err, os.ErrNotExist) {
		c.Status, c.Detail = "WARN", "data directory does not exist: "+home
		return c
	}
	if err == nil && !info.IsDir() {
		err = errors.New("data path is not a directory")
	}
	if err == nil {
		err = unix.Access(home, unix.W_OK|unix.X_OK)
	}
	if err == nil {
		err = checkDataChildren(home)
	}
	if err != nil {
		c.Status, c.Detail = "FAIL", err.Error()
	}
	return c
}

func checkDataChildren(home string) error {
	for _, item := range []struct {
		name string
		mode uint32
	}{{"sessions", unix.W_OK | unix.X_OK}, {"store.lock", unix.R_OK | unix.W_OK}} {
		path := filepath.Join(home, item.name)
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		if err := unix.Access(path, item.mode); err != nil {
			return &os.PathError{Op: "check data access", Path: path, Err: err}
		}
	}
	return nil
}
