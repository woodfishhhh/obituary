package rules

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/auto"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/rules"
)

func NewCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Show the rules in force here: built-ins plus the project's .obitrules once trusted",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := cmdutil.Engine()
			if err != nil {
				return err
			}
			p, err := CurrentProject()
			if err != nil {
				return err
			}
			set, state, err := e.RuleSet(p.Root)
			if err != nil {
				return err
			}
			if asJSON {
				return cmdutil.PrintJSON(map[string]any{"project": p.Root, "file": state, "rules": set.Rules})
			}
			fmt.Printf("project  %s\nrules    %s (%s)\n\n", p.Root, filepath.Join(p.Root, rules.FileName), state.Status)
			if state.Warning != "" {
				fmt.Println(state.Warning)
			}
			for _, r := range set.Rules {
				life := "off"
				if !r.Negate {
					life = core.FormatRemaining(r.Lifespan)
				}
				fmt.Printf("  %-24s %-6s %s:%d\n", r.Pattern, life, r.Source, r.Line)
			}
			if state.Status == rules.Untrusted || state.Status == rules.Changed {
				fmt.Printf("\nthe project's rules are not applied until you review them: obit trust\n")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	cmd.AddCommand(applyCmd("check", "Show what the rules would register for paths (changes nothing)", true),
		applyCmd("apply", "Register paths by rule (tracked files are never registered)", false))
	return cmd
}

func applyCmd(use, short string, dryRun bool) *cobra.Command {
	var asJSON bool
	var session string
	cmd := &cobra.Command{
		Use:   use + " <path>...",
		Short: short,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := cmdutil.Engine()
			if err != nil {
				return err
			}
			decisions, err := e.Apply(args, auto.Options{DryRun: dryRun, Session: session})
			for _, d := range decisions {
				if d.Action == auto.Registered {
					cmdutil.EnsureDaemon(e.Svc.Store.Home, asJSON)
					break
				}
			}
			if err := cmdutil.Committed(err); err != nil {
				return err
			}
			if asJSON {
				return cmdutil.PrintJSON(decisions)
			}
			for _, d := range decisions {
				if d.Warning != "" {
					fmt.Println("warning:", d.Warning)
				}
				switch d.Action {
				case auto.Skipped:
					fmt.Printf("skipped        %s  (%s)\n", d.Path, d.Reason)
				default:
					fmt.Printf("%-14s %s  %s by %s\n", d.Action, d.Unit, d.Lifespan, d.Rule)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	cmd.Flags().StringVar(&session, "session", "", "agent session the paths came from")
	return cmd
}
