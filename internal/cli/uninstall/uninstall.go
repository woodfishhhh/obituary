// Package uninstall coordinates the existing, ownership-aware uninstallers.
package uninstall

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/contextmenu"
	"github.com/woodfishhhh/obituary/internal/runkey"
)

// Native boundaries are functions so tests need neither a live tray nor a
// scheduled task. The registries already have their own test implementations.
type dependencies struct {
	executable      cmdutil.Executable
	runRegistry     runkey.Registry
	menuRegistry    contextmenu.Registry
	previewSchedule func(string, string, string) string
	removeSchedule  func() error
	stop            func(string) error
	terminal        func() bool
}

func NewCmd() *cobra.Command {
	return newCmd(dependencies{
		runRegistry: runkey.Current, menuRegistry: contextmenu.UserRegistry(),
		previewSchedule: func(daemon, home, config string) string {
			return app.PreviewScheduleRemovalFor(cmdutil.Localizer(), daemon, home, config)
		}, removeSchedule: app.RemoveSchedule, stop: app.Stop,
		terminal: func() bool { return cmdutil.IsTerminal(os.Stdin) },
	})
}

func newCmd(d dependencies) *cobra.Command {
	var dryRun, yes, purgeData bool
	cmd := &cobra.Command{
		Use: "uninstall", Short: "Remove Obituary integrations and background services; keep data by default",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			p, err := prepare(d)
			if err != nil {
				return err
			}
			p.print(cmd.OutOrStdout(), purgeData)
			if dryRun {
				return nil
			}
			input := bufio.NewReader(cmd.InOrStdin())
			if err := confirm(cmd, input, d.terminal(), yes, cmdutil.Text("cli.uninstall.apply_this_uninstall_plan_y_n")); err != nil {
				return err
			}
			if purgeData {
				if err := confirm(cmd, input, d.terminal(), yes, cmdutil.Text("cli.uninstall.permanently_purge_the_listed_graves_and_remove")); err != nil {
					return err
				}
			}
			if err := p.apply(d, cmd.OutOrStdout(), purgeData); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), cmdutil.Text("cli.uninstall.uninstalled_integrations_and_background_services_remove_binaries"))
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "only print the uninstall plan; do not change files or services")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm the plan (and data purge only when --purge-data is also given)")
	cmd.Flags().BoolVar(&purgeData, "purge-data", false, "permanently purge listed graves and remove owned data; keep unknown files")
	return cmd
}

func confirm(cmd *cobra.Command, input *bufio.Reader, terminal, yes bool, prompt string) error {
	if yes {
		return nil
	}
	if !terminal {
		return errors.New(cmdutil.Text("cli.uninstall.not_a_terminal_re_run_with_yes"))
	}
	fmt.Fprint(cmd.OutOrStdout(), prompt)
	answer, err := input.ReadString('\n')
	if err != nil {
		return fmt.Errorf(cmdutil.Text("cli.uninstall.confirmation_not_read_nothing_changed"), err)
	}
	if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
		return errors.New(cmdutil.Text("cli.init.nothing_changed"))
	}
	return nil
}
