package cmdutil

import (
	"fmt"
	"os"

	"github.com/woodfishhhh/obituary/internal/app"
)

// Process and scheduler boundaries shared by commands and diagnostics.
var (
	RunnerRunning   = app.Running
	RunnerStart     = app.Start
	RunnerAutostart = app.Autostart
	RunnerInstalled = app.ScheduleInstalled
	RunnerServes    = app.ScheduleServes
	RunnerTarget    = app.ScheduleTarget
)

// ensureDaemon makes sure something will act on new lifespans: the scheduled
// runner for this home, otherwise a resident daemon started for this session.
func EnsureDaemon(home string, quiet bool) error {
	if RunnerRunning(home) || RunnerServes(home) {
		return nil
	}
	if err := RunnerStart(home); err != nil {
		if !quiet {
			fmt.Fprintf(os.Stderr, "warning: could not start the daemon (%v); run `obit reap` or `obit daemon enable`\n", err)
		}
		return err
	}
	if !quiet && RunnerAutostart() == "" {
		fmt.Fprintln(os.Stderr, "started a daemon for now; `obit daemon enable` sets up the hidden 5-minute runner that survives restarts")
	}
	return nil
}
