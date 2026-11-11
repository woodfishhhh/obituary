package cmdutil

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/project"
)

const RecycleRecovery = "sent to the system Recycle Bin; restore from the Recycle Bin"

// Cobra renders help before RunE; load only config here, without opening a store.
func SetBurialHelp(cmd *cobra.Command) {
	help := cmd.HelpFunc()
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		cmd.Short = "Move paths using the configured deletion mode; recovery depends on that mode"
		cfg, err := config.Load()
		if err == nil {
			switch cfg.Graveyard.Mode {
			case config.ModeGraveyard:
				cmd.Short = "Move paths into the graveyard now (restore with obit resurrect)"
			case config.ModeRecycle:
				cmd.Short = "Move paths to the system Recycle Bin (restore from the Recycle Bin)"
			}
		}
		help(cmd, args)
	})
}

// BurialScope discovers invocation metadata best-effort. A missing project
// cannot block deletion; each target still passes the normal safety checks.
func BurialScope() (session, root string) {
	session = os.Getenv("OBIT_SESSION_ID")
	if session == "" {
		if id := os.Getenv("CLAUDE_CODE_SESSION_ID"); id != "" {
			session = "claude:" + id
		} else if id := os.Getenv("CODEX_THREAD_ID"); id != "" {
			// CODEX_THREAD_ID exists in current Codex tool environments but is
			// not an officially guaranteed interface; use it best-effort.
			session = "codex:" + id
		}
	}
	cwd, err := os.Getwd()
	if err != nil {
		return session, ""
	}
	p, found, err := project.Find(cwd)
	if err == nil && found {
		root = p.Root
	}
	return session, root
}

func BeginBurial(svc *core.Service) (*core.Service, error) {
	session, root := BurialScope()
	return svc.BeginBurial(session, root)
}
