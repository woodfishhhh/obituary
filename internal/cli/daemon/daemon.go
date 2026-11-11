package daemon

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func NewCmd() *cobra.Command {
	return NewCmdWithExecutable(nil)
}

func NewCmdWithExecutable(self cmdutil.Executable) *cobra.Command {
	cmd := &cobra.Command{Use: "daemon", Short: "Control the background daemon (obitd)"}
	home := userdirs.Data
	simple := func(use, short string, run func(string) error) *cobra.Command {
		return &cobra.Command{Use: use, Short: short, Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error {
			h, err := home()
			if err != nil {
				return err
			}
			return run(h)
		}}
	}
	cmd.AddCommand(
		simple("start", "Start the daemon now", func(h string) error {
			if err := app.Start(h); err != nil {
				return err
			}
			fmt.Println("daemon running")
			return nil
		}),
		simple("stop", "Stop the daemon", func(h string) error {
			if err := app.Stop(h); err != nil {
				return err
			}
			fmt.Println("daemon stopped")
			return nil
		}),
		enableCmd(home, self),
		disableCmd(home, self),
		StatusCmd(home),
	)
	return cmd
}

func enableCmd(home func() (string, error), self cmdutil.Executable) *cobra.Command {
	var resident, dryRun bool
	cmd := &cobra.Command{
		Use:   "enable",
		Short: "Enable the login tray and the hidden 5-minute fallback runner",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			h, err := home()
			if err != nil {
				return err
			}
			if dryRun {
				return preview(cmd.OutOrStdout(), self, h, true, resident)
			}
			return enable(h, resident)
		},
	}
	cmd.Flags().BoolVar(&resident, "resident", false, "enable the tray without changing the scheduled fallback")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "only print the changes")
	return cmd
}

func enable(home string, resident bool) error {
	if runtime.GOOS != "windows" {
		if resident {
			return app.Start(home)
		}
		if err := app.InstallSchedule(home); err != nil {
			return err
		}
		fmt.Println("user timer enabled; automatic burial remains unsupported on this platform")
		return nil
	}
	if !resident {
		if err := app.InstallSchedule(home); err != nil {
			return err
		}
	}
	if err := app.SetAutostart(home, true); err != nil {
		return err
	}
	if err := app.Start(home); err != nil {
		return err
	}
	if resident {
		fmt.Println("tray daemon running; it also starts when you sign in; any scheduled fallback is preserved")
	} else {
		fmt.Printf("tray running and enabled at sign-in; hidden fallback (%q) runs every %d minutes\n", app.TaskName, int(app.Interval.Minutes()))
	}
	return nil
}

func disableCmd(home func() (string, error), self cmdutil.Executable) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{Use: "disable", Short: "Remove the scheduled runner and the resident daemon", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			h, err := home()
			if err != nil {
				return err
			}
			if dryRun {
				return preview(cmd.OutOrStdout(), self, h, false, false)
			}
			if err := app.RemoveSchedule(); err != nil {
				return err
			}
			if err := app.SetAutostart(h, false); err != nil {
				return err
			}
			if err := app.Stop(h); err != nil {
				return err
			}
			fmt.Println("background runner removed; expiries now wait for obit reap or an agent hook")
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "only print the changes")
	return cmd
}
