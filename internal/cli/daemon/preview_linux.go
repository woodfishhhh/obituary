package daemon

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func preview(out io.Writer, self cmdutil.Executable, home string, enabled, resident bool) error {
	if resident {
		return fmt.Errorf("resident mode is unavailable; use the user timer: %w", core.ErrUnsupported)
	}
	exe, err := self.Path()
	if err != nil {
		return err
	}
	config, err := userdirs.Config()
	if err != nil {
		return err
	}
	units, err := app.PreviewSchedule(filepath.Join(filepath.Dir(exe), "obitd"), home, config, enabled)
	if err != nil {
		return err
	}
	changed := false
	for _, unit := range units {
		fmt.Fprintf(out, "  %-9s user unit %s\n", unit.Action, unit.Path)
		for _, line := range strings.Split(strings.TrimSuffix(unit.Content, "\n"), "\n") {
			fmt.Fprintln(out, "    "+line)
		}
		changed = changed || unit.Action != "unchanged"
	}
	if enabled || changed {
		fmt.Fprintln(out, "  systemctl --user daemon-reload")
		action := "enable"
		if !enabled {
			action = "disable"
		}
		fmt.Fprintf(out, "  systemctl --user %s --now %s\n", action, app.TaskName)
	}
	fmt.Fprintln(out, "  preview made no changes")
	return nil
}
