package cmdutil

import (
	"errors"
	"fmt"
	"os"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/core"
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
		switch {
		case quiet:
		case errors.Is(err, core.ErrUnsupported): // macOS / Linux: no resident daemon, only the user timer
			fmt.Fprintln(os.Stderr, Text("cli.daemon.no_timer"))
		default:
			fmt.Fprintf(os.Stderr, Text("cli.daemon.start_failed"), err)
		}
		return err
	}
	if !quiet && RunnerAutostart() == "" {
		fmt.Fprintln(os.Stderr, Text("cli.daemon.started_temporarily"))
	}
	return nil
}
