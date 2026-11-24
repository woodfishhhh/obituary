//go:build linux || darwin

package doctor

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"golang.org/x/sys/unix"
)

func (lang diagnostics) diagnoseStartup(string, string) DoctorCheck {
	if runtime.GOOS != "darwin" {
		return DoctorCheck{Name: "tray-startup", Status: "INFO", Detail: lang.Text("cli.doctor.tray_startup_and_explorer_labels_are_windows", nil)}
	}
	if path := cmdutil.RunnerAutostart(); path != "" {
		return DoctorCheck{Name: "tray-startup", Status: "PASS", Detail: lang.Text("cli.doctor.tray_starts_at_login", nil) + path}
	}
	return DoctorCheck{Name: "tray-startup", Status: "WARN", Detail: lang.Text("cli.doctor.tray_login_startup_is_absent_enable_it", nil)}
}

func (lang diagnostics) diagnoseRunner(home string) DoctorCheck {
	c := DoctorCheck{Name: "runner", Status: "WARN", Detail: lang.Text("cli.doctor.no_user_timer_serves_this_data_directory", nil)}
	if runtime.GOOS == "darwin" {
		c.Detail = lang.Text("cli.doctor.no_launchd_user_agent_serves_this_data", nil)
	}
	if cmdutil.RunnerInstalled() {
		target, err := cmdutil.RunnerTarget()
		if err != nil {
			c.Detail = err.Error()
		} else if cmdutil.RunnerServes(home) {
			c.Status, c.Detail = "PASS", lang.Text("cli.doctor.user_timer_serves_this_data_directory", nil)+target
		}
	}
	return c
}

func (lang diagnostics) diagnoseData(home string) DoctorCheck {
	c := DoctorCheck{Name: "data", Status: "PASS", Detail: lang.Text("cli.doctor.data_directory_is_accessible", nil) + home}
	info, err := os.Stat(home)
	if errors.Is(err, os.ErrNotExist) {
		c.Status, c.Detail = "WARN", lang.Text("cli.doctor.data_directory_does_not_exist", nil)+home
		return c
	}
	if err == nil && !info.IsDir() {
		err = errors.New(lang.Text("cli.doctor.data_path_is_not_a_directory", nil))
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

func openFile(path string) error {
	if runtime.GOOS == "darwin" {
		return startDetached(exec.Command("/usr/bin/open", path))
	}
	return startDetached(exec.Command("xdg-open", path))
}
