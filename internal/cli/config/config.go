package config

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func NewCmd() *cobra.Command {
	var create bool
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show the configuration file and effective settings (--create writes the defaults)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if create {
				path, err := config.Create()
				if err != nil {
					return err
				}
				fmt.Println("created", path)
				return nil
			}
			path, err := config.Path()
			if err != nil {
				return err
			}
			cfg, err := config.Load()
			state := "not created (defaults in use; obit config --create writes them)"
			if _, statErr := os.Stat(path); statErr == nil {
				state = "present"
			}
			data, _ := userdirs.Data()
			fmt.Printf("config file  %s (%s)\ndata         %s\n\n", path, state, data)
			if cfg.CodexWindowsShell == "" {
				fmt.Println("# codex_windows_shell is unset")
			} else {
				fmt.Printf("codex_windows_shell = %q\n", cfg.CodexWindowsShell)
			}
			fmt.Printf("\n[graveyard]\nmode = %q\nretention_days = %d\n", cfg.Graveyard.Mode, cfg.Graveyard.RetentionDays)
			fmt.Printf("\n[watch]\nstable_window = %q\n", cfg.Watch.StableWindow)
			fmt.Printf("\n[hooks]\n# List expiring items and recent burials at session start.\nsession_summary = %t\n", cfg.Hooks.SessionSummary)
			if len(cfg.AgentShells) > 0 {
				fmt.Println("\n[agent_shells]")
				for _, agent := range slices.Sorted(maps.Keys(cfg.AgentShells)) {
					fmt.Printf("%q = %q\n", agent, cfg.AgentShells[agent])
				}
			}
			fmt.Printf("\n[explorer]\n# Default scope: Downloads, Desktop (including subfolders), and folder-policy items.\nenabled = %t\n# Include other locations; Git still follows in_git.\neverywhere = %t\nin_git = %t\n", cfg.Explorer.Enabled, cfg.Explorer.Everywhere, cfg.Explorer.InGit)
			if err != nil {
				return errors.New("config file has errors: " + err.Error())
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&create, "create", false, "write a config file with the defaults")
	return cmd
}
