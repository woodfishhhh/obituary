package config

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/i18n"
	"github.com/woodfishhhh/obituary/internal/integrate"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func NewCmd() *cobra.Command {
	var create, yes, dryRun bool
	var language, maxSize string
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show the configuration file and effective settings (--create writes the defaults)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			maxSizeSet := cmd.Flags().Changed("max-size")
			if err := validateConfigFlags(create, language, maxSizeSet, yes, dryRun); err != nil {
				return err
			}
			if language != "" {
				if err := applyLanguage(language); err != nil {
					return err
				}
				if !maxSizeSet {
					return nil
				}
			}
			if maxSizeSet {
				return applyMaxSize(cmd, maxSize, yes, dryRun)
			}
			if create {
				path, err := config.Create()
				if err != nil {
					return err
				}
				fmt.Println(cmdutil.Text("cli.config.created"), path)
				return nil
			}
			return showConfig()
		},
	}
	cmd.Flags().BoolVar(&create, "create", false, "write a config file with the defaults")
	cmd.Flags().StringVar(&language, "language", "", "set the language: auto, en or zh-CN")
	cmd.Flags().StringVar(&maxSize, "max-size", "", "set graveyard capacity limit (e.g. 5GB, 500MB, 0 or unlimited to disable)")
	cmd.Flags().BoolVar(&yes, "yes", false, "apply a lower --max-size and purge oldest graves without prompting (agents: ask the user first)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "preview what --max-size would change and purge without modifying files")
	cmd.AddCommand(telemetryCmd())
	return cmd
}

func validateConfigFlags(create bool, language string, maxSizeSet, yes, dryRun bool) error {
	if create && language != "" && !maxSizeSet {
		return errors.New(cmdutil.Text("cli.config.create_and_language_cannot_be_used_together"))
	}
	if create && (language != "" || maxSizeSet) {
		return errors.New(cmdutil.Text("cli.config.create_conflict"))
	}
	if (yes || dryRun) && !maxSizeSet {
		return errors.New(cmdutil.Text("cli.config.max_size_flag_requires_max_size"))
	}
	return nil
}

func applyLanguage(language string) error {
	path, err := config.Path()
	if err != nil {
		return err
	}
	change, err := integrate.PlanLanguageConfig(path, language)
	if err != nil {
		return err
	}
	if err := integrate.Apply([]integrate.Change{change}); err != nil {
		return err
	}
	fmt.Printf("language = %q\n", language)
	return nil
}

func showConfig() error {
	path, err := config.Path()
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	state := cmdutil.Text("cli.config.not_created_defaults_in_use_obit_config")
	if _, statErr := os.Stat(path); statErr == nil {
		state = cmdutil.Text("cli.config.present")
	}
	data, _ := userdirs.Data()
	fmt.Printf(cmdutil.Text("cli.config.config_file_data"), path, state, data)
	if cfg.CodexWindowsShell == "" {
		fmt.Println(cmdutil.Text("cli.config.codex_windows_shell_is_unset"))
	} else {
		fmt.Printf("codex_windows_shell = %q\n", cfg.CodexWindowsShell)
	}
	fmt.Printf("\n[graveyard]\nmode = %q\nretention_days = %d\nmax_size = %q\n", cfg.Graveyard.Mode, cfg.Graveyard.RetentionDays, cfg.Graveyard.MaxSize)
	fmt.Printf("\n[update]\ncheck = %t\n", cfg.Update.Check)
	fmt.Printf("\n[telemetry]\nenabled = %t\n", cfg.Telemetry.Enabled)
	fmt.Printf("\n[watch]\nstable_window = %q\nfile_expiry = %q\n", cfg.Watch.StableWindow, cfg.Watch.FileExpiry)
	fmt.Printf("\n[ui]\nlanguage = %q\neffective_language = %q\n", cfg.UI.Language, config.UILanguage())
	fmt.Printf(cmdutil.Text("cli.config.hooks_list_expiring_items_and_recent_burials"), cfg.Hooks.SessionSummary, cfg.Hooks.SessionCleanup)
	fmt.Printf(cmdutil.Text("cli.config.proposals_off_nothing_goes_in_by_itself"), cfg.Proposals.Mode)
	if len(cfg.AgentShells) > 0 {
		fmt.Println("\n[agent_shells]")
		for _, agent := range slices.Sorted(maps.Keys(cfg.AgentShells)) {
			fmt.Printf("%q = %q\n", agent, cfg.AgentShells[agent])
		}
	}
	fmt.Printf(cmdutil.Text("cli.config.explorer_default_scope_downloads_desktop_including_subfolders"), cfg.Explorer.Enabled, cfg.Explorer.Everywhere, cfg.Explorer.InGit)
	if err != nil {
		return errors.New(cmdutil.Text("cli.config.config_file_has_errors") + err.Error())
	}
	return nil
}

func telemetryCmd() *cobra.Command {
	c := &cobra.Command{Use: "telemetry on|off", Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 || (args[0] != "on" && args[0] != "off") {
			return errors.New(cmdutil.Localizer().Text("telemetry.syntax", nil))
		}
		return nil
	}, RunE: func(cmd *cobra.Command, args []string) error {
		path, err := config.Path()
		if err != nil {
			return err
		}
		change, err := integrate.PlanTelemetryConfig(path, args[0] == "on")
		if err != nil {
			return err
		}
		if err := integrate.Apply([]integrate.Change{change}); err != nil {
			return err
		}
		if args[0] == "off" {
			return app.ForgetUsage()
		}
		return nil
	}}
	// Do not load locale/config on every invocation simply to construct a command tree.
	cmdutil.SetLocalizedHelp(c, func(l *i18n.Localizer, cmd *cobra.Command) { cmd.Short = l.Text("telemetry.help", nil) })
	return c
}
