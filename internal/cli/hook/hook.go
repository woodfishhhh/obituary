package hook

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/hook"
	"github.com/woodfishhhh/obituary/internal/userdirs"
)

func agentHook(agent hook.Agent) *cobra.Command {
	var event string
	var noticeOnly bool
	cmd := &cobra.Command{
		Use:   agent.Name,
		Short: "Handle one " + agent.Name + " hook event from stdin; errors fail open",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			runAgentHook(agent, event, noticeOnly)
			return nil
		},
	}
	cmd.Flags().StringVar(&event, "event", "", "host event name when omitted from the payload")
	cmd.Flags().BoolVar(&noticeOnly, "notice-only", false, "deliver advisory context without the deletion guard")
	return cmd
}

func runAgentHook(agent hook.Agent, event string, noticeOnly bool) {
	var ev hook.Event
	home := ""
	defer func() {
		if r := recover(); r != nil {
			if home != "" {
				hook.Logf(home, "%s: panic: %v", agent.Name, r)
			}
			hook.WriteFallback(agent.Adapter, ev, os.Stdout)
		}
	}()
	if hook.Disabled() {
		hook.WriteFallback(agent.Adapter, ev, os.Stdout)
		return
	}
	var err error
	home, err = userdirs.Data()
	if err != nil {
		hook.WriteFallback(agent.Adapter, ev, os.Stdout)
		return
	}
	data, err := hook.ReadInput(os.Stdin)
	if err == nil {
		ev, err = hook.Decode(agent.Adapter, data, hook.Invocation{Agent: agent.Name, Event: event})
	}
	if err != nil {
		if !errors.Is(err, hook.ErrSkip) {
			hook.LogInputError(home, hook.Invocation{Agent: agent.Name, Event: event}, data, err)
		}
		hook.WriteFallback(agent.Adapter, ev, os.Stdout)
		return
	}
	// Decode first: PreToolUse avoids service/config loading. Grave notices
	// read a bounded shared snapshot only for indexed, absent paths.
	ev.NoticeOnly = noticeOnly
	runDecodedEvent(agent, home, ev)
}

func runDecodedEvent(agent hook.Agent, home string, ev hook.Event) {
	svc := &core.Service{Store: core.Store{Home: home}}
	var runner *hook.Runner
	if ev.Name != "PreToolUse" {
		var err error
		svc, err = cmdutil.Open()
		if err != nil {
			hook.LogEventError(home, agent.Name, ev, fmt.Errorf("open service: %w", err))
			hook.WriteFallback(agent.Adapter, ev, os.Stdout)
			return
		}
		defer cmdutil.DeferExplorerSync(svc)()
		runner = cmdutil.Runner(svc)
	}
	hook.RunEvent(agent, app.Engine(svc), ev, os.Stdout, runner)
}

func NewCmd() *cobra.Command {
	// Cobra v1.10.2 command_win.go probes the parent through mousetrap before
	// dispatch, taking two system-wide process snapshots on every invocation.
	// Installed hooks are protocol endpoints, so use Cobra's documented opt-out
	// for them; ordinary CLI invocations retain the Explorer double-click hint.
	if len(os.Args) > 1 && os.Args[1] == "hook" {
		cobra.MousetrapHelpText = ""
	}
	cmd := &cobra.Command{Use: "hook", Short: "Agent hook entry points (called by the agent, not by people)"}
	finish, audit := finishCmd(), auditCmd()
	for _, agent := range hook.Agents() {
		cmd.AddCommand(agentHook(agent))
	}
	cmd.AddCommand(finish, audit)
	return cmd
}

func finishCmd() *cobra.Command {
	var project, session string
	finish := &cobra.Command{
		Use:    "finish",
		Short:  "Compute a session's leftovers (started in the background by the hooks)",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := cmdutil.Open()
			if err != nil {
				if home, homeErr := userdirs.Data(); homeErr == nil {
					hook.LogEventError(home, "background", hook.Event{Name: "finish", Cwd: project, Session: session}, fmt.Errorf("open service: %w", err))
				}
				return nil
			}
			if _, err := app.Engine(svc).Finish(project, session); err != nil {
				hook.LogEventError(svc.Store.Home, "background", hook.Event{Name: "finish", Cwd: project, Session: session}, err)
			}
			return nil
		},
	}
	finish.Flags().StringVar(&project, "project", "", "project root")
	finish.Flags().StringVar(&session, "session", "", "agent session id")
	return finish
}

func auditCmd() *cobra.Command {
	var lines int
	audit := &cobra.Command{
		Use:   "audit",
		Short: "Show hook and delete-guard audit trails",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := userdirs.Data()
			if err != nil {
				return err
			}
			return printAudit(home, lines)
		},
	}
	audit.Flags().IntVarP(&lines, "lines", "n", 40, "number of lines per source")
	return audit
}

func printAudit(home string, lines int) error {
	shown := false
	for _, source := range []string{"hook-audit.log", "delete-guard.jsonl"} {
		data, err := os.ReadFile(filepath.Join(home, source))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		trail := strings.TrimSpace(string(data))
		if trail == "" {
			continue
		}
		all := strings.Split(trail, "\n")
		if len(all) > lines {
			all = all[len(all)-lines:]
		}
		fmt.Printf("== %s ==\n%s\n", source, strings.Join(all, "\n"))
		shown = true
	}
	if !shown {
		fmt.Println("no audit trail in hook-audit.log or delete-guard.jsonl; delete denials are recorded automatically; set OBIT_HOOK_AUDIT=1 to also record hook events and skipped commands")
	}
	return nil
}
