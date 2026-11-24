package doctor

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/hook"
	"golang.org/x/sys/windows"
)

func (lang diagnostics) diagnoseRunner(home string) DoctorCheck {
	c := DoctorCheck{Name: "runner", Status: "WARN", Detail: lang.Text("cli.doctor.no_runner_serves_this_data_directory_obit", nil)}
	running := cmdutil.RunnerRunning(home)
	if cmdutil.RunnerInstalled() { //nolint:nestif // Existing Windows diagnostic flow moved unchanged into the platform file.
		target, err := cmdutil.RunnerTarget()
		switch {
		case errors.Is(err, app.ErrScheduleDisabled):
			c.Status, c.Detail = "FAIL", lang.Text("cli.doctor.scheduled_task_is_disabled", nil)+target
		case err != nil:
			c.Status, c.Detail = "FAIL", lang.Text("cli.doctor.cannot_read_scheduled_task", nil)+err.Error()
		case !cmdutil.RunnerServes(home):
			c.Status, c.Detail = "FAIL", lang.Text("cli.doctor.scheduled_task_does_not_serve_this_data", nil)+target
		default:
			args, err := windows.DecomposeCommandLine(target)
			if err == nil && len(args) > 0 {
				err = lang.regularFile(args[0])
				if err == nil && !strings.EqualFold(filepath.Base(args[0]), "obitd.exe") {
					err = errors.New(lang.Text("cli.doctor.scheduled_command_is_not_obitd_exe", nil))
				}
			} else if err == nil {
				err = errors.New(lang.Text("cli.doctor.empty_scheduled_command", nil))
			}
			if err != nil {
				c.Status, c.Detail = "FAIL", lang.Text("cli.doctor.invalid_scheduled_executable", nil)+err.Error()
			} else {
				c.Status, c.Detail = "PASS", lang.Text("cli.doctor.scheduled_task_serves_this_data_directory", nil)+target
			}
		}
		if c.Status == "FAIL" && running && !errors.Is(err, app.ErrScheduleDisabled) {
			c.Status, c.Detail = "WARN", c.Detail+lang.Text("cli.doctor.a_resident_daemon_currently_serves_this_directory", nil)
		}
	} else if running {
		c.Status, c.Detail = "PASS", lang.Text("cli.doctor.resident_daemon_serves_this_data_directory", nil)
	}
	return c
}

func (lang diagnostics) diagnoseData(home string) DoctorCheck {
	c := DoctorCheck{Name: "data", Status: "PASS", Detail: lang.Text("cli.doctor.data_directory_grants_creation_access_existing_store", nil) + home}
	info, err := os.Stat(home)
	if errors.Is(err, os.ErrNotExist) {
		c.Status, c.Detail = "WARN", lang.Text("cli.doctor.data_directory_does_not_exist", nil)+home
		return c
	}
	if err == nil && !info.IsDir() {
		err = errors.New(lang.Text("cli.doctor.data_path_is_not_a_directory_diagnosedata", nil) + home)
	}
	if err == nil {
		for _, entry := range []struct {
			name   string
			access uint32
			flags  uint32
		}{
			{"", windows.FILE_WRITE_DATA | windows.FILE_APPEND_DATA, windows.FILE_FLAG_BACKUP_SEMANTICS},
			{"sessions", windows.FILE_WRITE_DATA | windows.FILE_APPEND_DATA, windows.FILE_FLAG_BACKUP_SEMANTICS},
			{"store.lock", windows.GENERIC_READ | windows.GENERIC_WRITE, 0},
		} {
			path := filepath.Join(home, entry.name)
			// OPEN_EXISTING only asks Windows to check access; no probe file,
			// directory, attribute or security descriptor is ever written.
			var handle windows.Handle
			var name *uint16
			name, err = windows.UTF16PtrFromString(path)
			if err == nil {
				handle, err = windows.CreateFile(name, entry.access,
					windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
					windows.OPEN_EXISTING, entry.flags, 0)
			}
			if entry.name != "" && errors.Is(err, os.ErrNotExist) {
				err = nil
				continue
			}
			if err != nil {
				err = &os.PathError{Op: "check data access", Path: path, Err: err}
				break
			}
			windows.CloseHandle(handle)
		}
	}
	if err != nil {
		c.Status, c.Detail = "FAIL", err.Error()
		if errors.Is(err, os.ErrPermission) {
			c.Detail += "; " + hook.PermissionHint
		}
	}
	return c
}

// openFile hands the file to its default program through the system's own
// explorer.exe, as the tray opens folders.
func openFile(path string) error {
	dir, err := windows.GetWindowsDirectory()
	if err != nil {
		return err
	}
	return startDetached(exec.Command(filepath.Join(dir, "explorer.exe"), path))
}
