package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/hook"
	"golang.org/x/sys/windows"
)

func diagnoseRunner(home string) DoctorCheck {
	c := DoctorCheck{Name: "runner", Status: "WARN", Detail: "no runner serves this data directory; obit daemon enable installs one"}
	running := cmdutil.RunnerRunning(home)
	if cmdutil.RunnerInstalled() { //nolint:nestif // Existing Windows diagnostic flow moved unchanged into the platform file.
		target, err := cmdutil.RunnerTarget()
		switch {
		case errors.Is(err, app.ErrScheduleDisabled):
			c.Status, c.Detail = "FAIL", "scheduled task is disabled: "+target
		case err != nil:
			c.Status, c.Detail = "FAIL", "cannot read scheduled task: "+err.Error()
		case !cmdutil.RunnerServes(home):
			c.Status, c.Detail = "FAIL", "scheduled task does not serve this data directory: "+target
		default:
			args, err := windows.DecomposeCommandLine(target)
			if err == nil && len(args) > 0 {
				err = regularFile(args[0])
				if err == nil && !strings.EqualFold(filepath.Base(args[0]), "obitd.exe") {
					err = errors.New("scheduled command is not obitd.exe")
				}
			} else if err == nil {
				err = errors.New("empty scheduled command")
			}
			if err != nil {
				c.Status, c.Detail = "FAIL", "invalid scheduled executable: "+err.Error()
			} else {
				c.Status, c.Detail = "PASS", "scheduled task serves this data directory: "+target
			}
		}
		if c.Status == "FAIL" && running && !errors.Is(err, app.ErrScheduleDisabled) {
			c.Status, c.Detail = "WARN", c.Detail+"; a resident daemon currently serves this directory"
		}
	} else if running {
		c.Status, c.Detail = "PASS", "resident daemon serves this data directory"
	}
	return c
}

func diagnoseData(home string) DoctorCheck {
	c := DoctorCheck{Name: "data", Status: "PASS", Detail: "data directory grants creation access; existing store.lock and sessions are accessible: " + home}
	info, err := os.Stat(home)
	if errors.Is(err, os.ErrNotExist) {
		c.Status, c.Detail = "WARN", "data directory does not exist: "+home
		return c
	}
	if err == nil && !info.IsDir() {
		err = errors.New("data path is not a directory: " + home)
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
