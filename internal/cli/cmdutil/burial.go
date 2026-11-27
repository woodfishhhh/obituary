package cmdutil

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/config"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/project"
)

// RecycleRecovery describes a confirmed file receipt. Directories receive a grave and a resurrection ID.
const RecycleRecovery = "sent to the system Recycle Bin; restore from the Recycle Bin"

// Cobra renders help before RunE; load only config here, without opening a store.
func SetBurialHelp(cmd *cobra.Command) {
	help := cmd.HelpFunc()
	cmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		language := Localizer()
		cmd.Short = language.Text("cli.bury.short", nil)
		cfg, err := config.Load()
		if err == nil {
			switch cfg.Graveyard.Mode {
			case config.ModeGraveyard:
				cmd.Short = language.Text("cli.bury.short.graveyard", nil)
			case config.ModeRecycle:
				cmd.Short = language.Text("cli.bury.short.recycle", nil)
			}
		}
		if flag := cmd.Flags().Lookup("force"); flag != nil {
			flag.Usage = language.Text("cli.bury.force", nil)
		}
		if flag := cmd.Flags().Lookup("note"); flag != nil {
			flag.Usage = language.Text("cli.bury.note", nil)
		}
		if flag := cmd.Flags().Lookup("json"); flag != nil {
			flag.Usage = language.Text("cli.bury.json", nil)
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

func BeginBurial(svc *lifecycle.Service) (*lifecycle.Service, error) {
	session, root := BurialScope()
	return svc.BeginBurial(session, root)
}
