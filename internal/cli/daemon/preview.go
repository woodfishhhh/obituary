//go:build windows

package daemon

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/runkey"
	"github.com/woodfishhhh/obituary/internal/userdirs"
	"golang.org/x/sys/windows"
)

var scheduleRemovalPreview = app.PreviewScheduleRemoval

// Reuse init's read-only autostart plan and action/path/command presentation.
func preview(out io.Writer, self cmdutil.Executable, home string, enabled, resident bool) error {
	exe, err := self.Path()
	if err != nil {
		return err
	}
	config, err := userdirs.Config()
	if err != nil {
		return err
	}
	daemon := filepath.Join(filepath.Dir(exe), "obitd.exe")
	plan, err := app.PlanAutostart(runkey.Current, daemon, home, config, enabled)
	if err != nil {
		return err
	}
	previewSchedule(out, daemon, home, config, enabled, resident)
	action, command := "set", plan.After
	if !enabled {
		action, command = "remove", plan.Before
	}
	if plan.Pending() {
		fmt.Fprintf(out, "  %-9s HKCU Run %s\n    %s\n", action, plan.Name, command)
	} else {
		fmt.Fprintf(out, "  unchanged HKCU Run %s\n", plan.Name)
	}
	action = "start"
	if !enabled {
		action = "stop"
	}
	if cmdutil.RunnerRunning(home) == enabled {
		action = "unchanged"
	}
	fmt.Fprintf(out, "  %-9s resident obitd for %s\n", action, home)
	return nil
}

func previewSchedule(out io.Writer, daemon, home, config string, enabled, resident bool) {
	if resident {
		fmt.Fprintf(out, "  preserve  scheduled task %q\n", app.TaskName)
		return
	}
	if !enabled {
		fmt.Fprintln(out, "  "+scheduleRemovalPreview(daemon, home, config))
		return
	}
	action := "create"
	installed := cmdutil.RunnerInstalled()
	if installed {
		action = "update"
	}
	fmt.Fprintf(out, "  %-9s scheduled task %q (every %d minutes and at sign-in)\n", action, app.TaskName, int(app.Interval.Minutes()))
	fmt.Fprintln(out, "    "+windows.ComposeCommandLine([]string{daemon, "--once", "--home", home, "--config-dir", config}))
}
