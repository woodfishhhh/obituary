package daemon

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func preview(out io.Writer, self cmdutil.Executable, home string, enabled, resident bool) error {
	if resident {
		return fmt.Errorf(cmdutil.Text("cli.daemon.resident_mode_is_unavailable_use_the_launchd"), core.ErrUnsupported)
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
	unit := units[0]
	fmt.Fprintf(out, cmdutil.Text("cli.daemon.launchd_user_agent"), unit.Action, unit.Path)
	for _, line := range strings.Split(strings.TrimSuffix(unit.Content, "\n"), "\n") {
		fmt.Fprintln(out, "    "+line)
	}
	if enabled {
		fmt.Fprintf(out, cmdutil.Text("cli.daemon.launchctl_bootout_gui_if_loaded"), os.Getuid(), app.TaskName)
		fmt.Fprintf(out, "  launchctl enable gui/%d/%s\n", os.Getuid(), app.TaskName)
		fmt.Fprintf(out, "  launchctl bootstrap gui/%d %q\n", os.Getuid(), unit.Path)
	} else if unit.Action == "remove" {
		fmt.Fprintf(out, cmdutil.Text("cli.daemon.launchctl_bootout_gui_if_loaded"), os.Getuid(), app.TaskName)
	}
	fmt.Fprintln(out, cmdutil.Text("cli.daemon.preview_made_no_changes"))
	return nil
}
