package update

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/buildinfo"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/update"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func NewCmd() *cobra.Command { return newCmd(nil) }

func newCmd(checker *update.Checker) *cobra.Command {
	var check bool
	cmd := &cobra.Command{Use: "update", Short: "Show release information and installation-specific update instructions", Args: cobra.NoArgs}
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		home, err := userdirs.Data()
		if err != nil {
			return err
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
		if err != nil {
			state.LastError = err.Error()
		}
		exe, _ := os.Executable()
		channel := update.DetectChannel(exe)
		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "Current: %s\nLatest: %s\nNewer: %t\nAutomatic checks: %t\nChannel: %s\n", buildinfo.Version, state.Latest.Version, update.Newer(buildinfo.Version, state.Latest.Version), cfg.Update.Check, channel)
		if command := update.Command(channel, state.Latest.Version, exe); command != "" {
			fmt.Fprintln(out, "Update:", command)
		}
		if state.Latest.URL != "" {
			fmt.Fprintln(out, "Release:", state.Latest.URL)
		}
		if state.LastError != "" {
			fmt.Fprintln(out, "Last check:", state.LastError)
		}
		if state.CheckedAt.IsZero() {
			fmt.Fprintln(out, "No cached check; run obit update --check.")
		}
		return nil
	}
	cmd.Flags().BoolVar(&check, "check", false, "check GitHub now, including when automatic checks are disabled")
	return cmd
}
