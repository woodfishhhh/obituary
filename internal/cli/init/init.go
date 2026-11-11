package init

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	rulescmd "github.com/woodfishhhh/obituary/internal/cli/rules"
	"github.com/woodfishhhh/obituary/internal/integrate"
)

func NewCmd() *cobra.Command { return NewCmdWithExecutable(nil) }

//nolint:funlen // Preserve the existing command flow while replacing the process-wide executable test hook.
func NewCmdWithExecutable(executable cmdutil.Executable) *cobra.Command {
	planner := Planner{Executable: executable}
	var (
		agents                                        []string
		format                                        string
		global, uninstall, show, dryRun, yes, guardRM bool
		withMCP, withSkill                            bool
		withContextMenu                               bool
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Install (or --uninstall) Obituary's hooks and instructions for an agent",
		Example: `  obit init -g                 # Claude Code, for every project (~/.claude)
  obit init                    # Claude Code, this project only (.claude/)
  obit init -g --agent codex   # Codex (~/.codex/hooks.json + AGENTS.md), Windows and WSL
  obit init -g --uninstall     # remove only what Obituary added
  obit init -g --show          # what is installed
  obit init --show --format markdown > docs/agents.md`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateShowFormat(cmd, format, show, withContextMenu); err != nil {
				return err
			}
			if withContextMenu {
				return runContextMenu(cmd, executable, uninstall, show, dryRun, yes)
			}
			var selected []string
			for _, agent := range agents {
				if !slices.Contains(AgentNames(), agent) {
					return fmt.Errorf("unknown agent %q (supported: %s)", agent, strings.Join(AgentNames(), ", "))
				}
				if !slices.Contains(selected, agent) {
					selected = append(selected, agent)
				}
			}
			if len(selected) == 0 {
				return errors.New("--agent requires at least one name")
			}
			if format == "markdown" {
				if !cmd.Flags().Changed("agent") {
					selected = AgentNames()
				}
				_, err := fmt.Fprint(cmd.OutOrStdout(), coverageMarkdown(selected))
				return err
			}
			scope := integrate.Scope{Global: global, GuardRM: guardRM, MCP: withMCP, Skill: withSkill}
			if !global {
				p, err := rulescmd.CurrentProject()
				if err != nil {
					return fmt.Errorf("%w; use -g for a user-wide install", err)
				}
				scope.Project = p.Root
			}
			if show {
				scope = scope.ForInspection()
				names := AgentNames()
				statusScope := scope
				if cmd.Flags().Changed("agent") {
					names = selected
				} else {
					// The unfiltered inventory does not request optional extensions
					// for agents outside the historical default selection.
					statusScope.MCP, statusScope.Skill = false, false
				}
				if err := planner.showAgents(statusScope, names, !cmd.Flags().Changed("agent")); err != nil {
					return err
				}
				for _, agent := range selected {
					if agent == "claude" || agent == "codex" {
						changes, err := planner.IntegrationPlan(agent, scope, false)
						if err != nil {
							return err
						}
						if !integrate.Pending(changes) {
							fmt.Println(agentName(agent) + " integration: installed")
						} else {
							fmt.Println(agentName(agent) + " integration: not (fully) installed; obit init would:")
						}
						printChanges(changes)
						registered, effective := integrate.GuardRMStatus(changes, agent)
						status, _ := DeleteGuardStatus(registered, effective)
						fmt.Printf("delete guard: %s\n", status)
					}
					if err := planner.showExtensions(scope, agent); err != nil {
						return err
					}
				}
				return nil
			}
			changes, err := planner.IntegrationPlans(selected, scope, uninstall)
			if err != nil {
				return err
			}
			printChanges(changes)
			if !integrate.Pending(changes) {
				if err := incompleteIntegration(changes); err != nil {
					return err
				}
			}
			startup, err := planStartup(executable, !uninstall)
			if err != nil {
				return err
			}
			if !integrate.Pending(changes) && (startup == nil || !startup.Pending()) {
				if uninstall {
					fmt.Println("nothing to remove")
				} else {
					fmt.Println("already installed")
				}
				return nil
			}
			if dryRun {
				return incompleteIntegration(changes)
			}
			if !yes {
				if !cmdutil.IsTerminal(os.Stdin) {
					return errors.New("not a terminal: re-run with --yes to apply these changes")
				}
				fmt.Print("Apply these changes (backups are kept as .bak)? [y/N] ")
				answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
				if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
					return errors.New("nothing changed")
				}
			}
			// Manual changes only carry instructions. Keep real preserve conflicts
			// in Apply so ownership failures retain its normal nonzero result.
			automatic := slices.DeleteFunc(slices.Clone(changes), func(c integrate.Change) bool { return c.Manual })
			if err := applyIntegrationPlan(automatic, uninstall); err != nil {
				_ = incompleteIntegration(changes)
				return err
			}
			if err := incompleteIntegration(changes); err != nil {
				return err
			}
			if startup != nil {
				if err := startup.Apply(); err != nil {
					return err
				}
			}
			for _, agent := range selected {
				switch {
				case uninstall:
					fmt.Println("removed; restart " + agentName(agent))
				case agent == "codex":
					fmt.Println("installed. Codex only runs hooks you have trusted: start Codex, run /hooks and trust Obituary's hooks (project hooks also need the project to be trusted).")
				default:
					fmt.Println("installed; restart " + agentName(agent))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringSliceVar(&agents, "agent", []string{"claude"}, "agents to integrate with (repeat or comma-separate; default: claude)")
	cmd.Flags().BoolVarP(&global, "global", "g", false, "user-wide instead of this project")
	cmd.Flags().BoolVar(&uninstall, "uninstall", false, "remove what Obituary added")
	cmd.Flags().BoolVar(&show, "show", false, "show what is installed")
	cmd.Flags().StringVar(&format, "format", "text", "output format for --show: text or markdown (agent coverage)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "only print the changes")
	cmd.Flags().BoolVar(&yes, "yes", false, "apply without asking")
	cmd.Flags().BoolVar(&guardRM, "guard-rm", false, "deny supported deletion commands with restorable obit alternatives")
	cmd.Flags().BoolVar(&withMCP, "mcp", false, "register the Obituary MCP stdio server")
	cmd.Flags().BoolVar(&withSkill, "skill", false, "install the Obituary Agent Skill")
	cmd.Flags().BoolVar(&withContextMenu, "context-menu", false, "install classic Explorer context menus for the current user")
	return cmd
}

func agentName(agent string) string {
	if agent == "claude" {
		return "Claude Code"
	}
	if agent == "codex" {
		return "Codex"
	}
	return agent
}

func incompleteIntegration(changes []integrate.Change) error {
	if slices.ContainsFunc(changes, func(c integrate.Change) bool { return c.Manual && !c.Implicit }) {
		fmt.Println("未完成，需要手动步骤 (incomplete; manual steps required): complete the steps above, then inspect with obit init --show; manual host state cannot be verified automatically.")
		return cmdutil.ErrReported
	}
	return integrate.PreservedError(changes)
}

func printChanges(changes []integrate.Change) {
	for _, c := range changes {
		detail := ""
		if c.Detail != "" {
			detail = "  (" + c.Detail + ")"
		}
		fmt.Printf("  %-9s %s%s\n", c.Action, c.Path, detail)
		for _, args := range c.Commands {
			fmt.Println("    " + integrate.CommandText(args))
		}
	}
}
