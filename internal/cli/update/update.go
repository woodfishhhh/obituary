package update

import (
	"errors"
	"fmt"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/buildinfo"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/update"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func NewCmd() *cobra.Command { return newCmd(nil) }

func newCmd(checker *update.Checker) *cobra.Command {
	var check, apply, relaunch bool
	var waitPID int
	cmd := &cobra.Command{Use: "update", Short: "Check or install a stable update", Args: cobra.NoArgs}
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		if waitPID < 0 || cmd.Flags().Changed("wait-pid") && waitPID == 0 {
			return errors.New(cmdutil.Text("cli.update.wait_pid_must_be_a_positive_process"))
		}
		if !apply && (waitPID != 0 || relaunch) {
			return errors.New(cmdutil.Text("cli.update.wait_pid_and_relaunch_gui_require_apply"))
		}
		home, err := userdirs.Data()
		if err != nil {
			return err
		}
		if apply && !check {
			return applyUpdate(cmd, home, waitPID, relaunch)
		}
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		state, err := update.Read(home)
		if check {
			c := update.Checker{Home: home}
			if checker != nil {
				c = *checker
				c.Home = home
			}
			state, err = c.Check(cmd.Context(), cfg.Update.Check, true)
		}
		if apply {
			if err != nil {
				return err
			}
			if state.LastError != "" {
				return fmt.Errorf(cmdutil.Text("cli.update.check_update"), state.LastError)
			}
			return applyUpdate(cmd, home, waitPID, relaunch)
		}
		if err != nil {
			state.LastError = err.Error()
		}
		return showUpdate(cmd, home, state, cfg.Update.Check)
	}
	cmd.Flags().BoolVar(&check, "check", false, "check GitHub now, including when automatic checks are disabled")
	cmd.Flags().BoolVar(&apply, "apply", false, "download, verify and install the cached newer stable release (a manual install replaces only Obituary's own programs)")
	cmd.Flags().IntVar(&waitPID, "wait-pid", 0, "wait for this process to exit before updating (Windows)")
	cmd.Flags().BoolVar(&relaunch, "relaunch-gui", false, "reopen the desktop window after updating (Windows)")
	return cmd
}

func applyUpdate(cmd *cobra.Command, home string, waitPID int, relaunch bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return app.ApplyUpdate(cmd.Context(), home, exe, buildinfo.Version, waitPID, relaunch, cmd.OutOrStdout())
}

// showHowToUpdate says how to install a newer release: the package manager or
// script command, or for a manual install `obit update --apply` (or why it
// cannot be used here).
func showHowToUpdate(out io.Writer, ap update.Applicability, command string, newer bool) {
	switch {
	case command != "":
		fmt.Fprintln(out, cmdutil.Text("cli.update.update"), command)
	case newer && ap.Can:
		fmt.Fprintln(out, cmdutil.Text("cli.update.update_run_obit_update_apply_replaces_only"))
	case newer && ap.Blocked != "":
		fmt.Fprintln(out, (&update.BlockedError{Reason: ap.Blocked, Detail: ap.Detail}).Error())
	}
}

func showUpdate(cmd *cobra.Command, home string, state update.Cache, automatic bool) error {
	exe, _ := os.Executable()
	ap := update.Review(exe, buildinfo.Version, state.Latest.Version)
	newer := update.Newer(buildinfo.Version, state.Latest.Version)
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, cmdutil.Text("cli.update.status"), buildinfo.Version, state.Latest.Version, newer, automatic, ap.Channel)
	showHowToUpdate(out, ap, update.Command(ap.Channel, state.Latest.Version, exe), newer)
	if state.Latest.URL != "" {
		fmt.Fprintln(out, cmdutil.Text("cli.update.release"), state.Latest.URL)
	}
	if state.LastError != "" {
		fmt.Fprintln(out, cmdutil.Text("cli.update.last_check"), state.LastError)
	}
	if state.CheckedAt.IsZero() {
		fmt.Fprintln(out, cmdutil.Text("cli.update.no_cached_check_run_obit_update_check"))
	}
	last, err := update.ReadLastApply(home)
	if last != nil {
		fmt.Fprintf(out, cmdutil.Text("cli.update.last_update_to_success_installed_log"), last.TargetVersion, last.Success, last.Version, last.LogPath)
		if last.Error != "" {
			fmt.Fprintln(out, last.Error)
		}
	}
	return err
}
