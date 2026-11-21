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
	action, command := cmdutil.Text("cli.daemon.set"), plan.After
	if !enabled {
		action, command = cmdutil.Text("cli.daemon.remove"), plan.Before
	}
	if plan.Pending() {
		fmt.Fprintf(out, cmdutil.Text("cli.daemon.hkcu_run"), action, plan.Name, command)
	} else {
		fmt.Fprintf(out, cmdutil.Text("cli.daemon.unchanged_hkcu_run"), plan.Name)
	}
	action = cmdutil.Text("cli.daemon.start")
	if !enabled {
		action = cmdutil.Text("cli.daemon.stop")
	}
	if cmdutil.RunnerRunning(home) == enabled {
		action = cmdutil.Text("cli.daemon.unchanged")
	}
	fmt.Fprintf(out, cmdutil.Text("cli.daemon.resident_obitd_for"), action, home)
	return nil
}

func previewSchedule(out io.Writer, daemon, home, config string, enabled, resident bool) {
	if resident {
		fmt.Fprintf(out, cmdutil.Text("cli.daemon.preserve_scheduled_task"), app.TaskName)
		return
	}
	if !enabled {
		fmt.Fprintln(out, "  "+scheduleRemovalPreview(daemon, home, config))
		return
	}
	action := cmdutil.Text("cli.daemon.create")
	installed := cmdutil.RunnerInstalled()
	if installed {
		action = cmdutil.Text("cli.daemon.update")
	}
	fmt.Fprintf(out, cmdutil.Text("cli.daemon.scheduled_task_every_minutes_and_at_sign"), action, app.TaskName, int(app.Interval.Minutes()))
	fmt.Fprintln(out, "    "+windows.ComposeCommandLine([]string{daemon, "--once", "--home", home, "--config-dir", config}))
}
