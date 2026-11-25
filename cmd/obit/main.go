// Command obit gives files and folders a lifespan: when it runs out, they
// are buried in a graveyard and can be resurrected until they are purged.
package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/buildinfo"
	addcmd "github.com/woodfishhhh/obituary/internal/cli/add"
	burycmd "github.com/woodfishhhh/obituary/internal/cli/bury"
	cancelcmd "github.com/woodfishhhh/obituary/internal/cli/cancel"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	configcmd "github.com/woodfishhhh/obituary/internal/cli/config"
	contextmenucmd "github.com/woodfishhhh/obituary/internal/cli/context-menu"
	daemoncmd "github.com/woodfishhhh/obituary/internal/cli/daemon"
	dockercmd "github.com/woodfishhhh/obituary/internal/cli/docker"
	doctorcmd "github.com/woodfishhhh/obituary/internal/cli/doctor"
	extendcmd "github.com/woodfishhhh/obituary/internal/cli/extend"
	gitcleancmd "github.com/woodfishhhh/obituary/internal/cli/gitclean"
	graveyardcmd "github.com/woodfishhhh/obituary/internal/cli/graveyard"
	hookcmd "github.com/woodfishhhh/obituary/internal/cli/hook"
	initcmd "github.com/woodfishhhh/obituary/internal/cli/init"
	listcmd "github.com/woodfishhhh/obituary/internal/cli/list"
	logcmd "github.com/woodfishhhh/obituary/internal/cli/log"
	mcpcmd "github.com/woodfishhhh/obituary/internal/cli/mcp"
	mktempcmd "github.com/woodfishhhh/obituary/internal/cli/mktemp"
	proposalscmd "github.com/woodfishhhh/obituary/internal/cli/proposals"
	purgecmd "github.com/woodfishhhh/obituary/internal/cli/purge"
	removeitemcmd "github.com/woodfishhhh/obituary/internal/cli/removeitem"
	resurrectcmd "github.com/woodfishhhh/obituary/internal/cli/resurrect"
	rmcmd "github.com/woodfishhhh/obituary/internal/cli/rm"
	rmdircmd "github.com/woodfishhhh/obituary/internal/cli/rmdir"
	rulescmd "github.com/woodfishhhh/obituary/internal/cli/rules"
	statscmd "github.com/woodfishhhh/obituary/internal/cli/stats"
	sweepcmd "github.com/woodfishhhh/obituary/internal/cli/sweep"
	trustcmd "github.com/woodfishhhh/obituary/internal/cli/trust"
	uninstallcmd "github.com/woodfishhhh/obituary/internal/cli/uninstall"
	untrustcmd "github.com/woodfishhhh/obituary/internal/cli/untrust"
	updatecmd "github.com/woodfishhhh/obituary/internal/cli/update"
	watchcmd "github.com/woodfishhhh/obituary/internal/cli/watch"
)

func main() {
	cmdutil.SkipExplorerCheck()
	root := newRoot(os.Args[1:])
	cmdutil.SetUsage(root)
	if command, err := root.ExecuteC(); err != nil {
		if !errors.Is(err, cmdutil.ErrReported) {
			fmt.Fprintln(os.Stderr, "obit:", cmdutil.CommandError(command, os.Args[1:], err))
		}
		os.Exit(1)
	}
}

func newRoot(args []string) *cobra.Command {
	root := &cobra.Command{
		Use:           "obit",
		Short:         "Obituary (讣告), the trash bin for AI agents: give files a lifespan; when it runs out they are buried in a graveyard you can resurrect from",
		Version:       buildinfo.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	if len(args) > 0 && args[0] == "hook" {
		root.AddCommand(hookcmd.NewCmd())
		return root
	}
	root.AddCommand(
		addcmd.NewCmd(), listcmd.NewCmd(), extendcmd.NewCmd(), cancelcmd.NewCmd(),
		sweepcmd.NewCmd(), burycmd.NewCmd(), resurrectcmd.NewCmd(), graveyardcmd.NewCmd(),
		purgecmd.NewCmd(), statscmd.NewCmd(), mktempcmd.NewCmd(), rulescmd.NewCmd(),
		trustcmd.NewCmd(), untrustcmd.NewCmd(), proposalscmd.NewCmd(),
		proposalscmd.NewDecideCmd("approve", "Approve proposals: register what they list", true),
		proposalscmd.NewDecideCmd("reject", "Reject proposals", false),
		initcmd.NewCmd(), hookcmd.NewCmd(), logcmd.NewCmd(), configcmd.NewCmd(),
		daemoncmd.NewCmd(), doctorcmd.NewCmd(), contextmenucmd.NewCmd(),
		watchcmd.NewCmd(), dockercmd.NewCmd(),
		uninstallcmd.NewCmd(), updatecmd.NewCmd(),
	)
	root.AddCommand(
		rmcmd.NewCmd(), rmdircmd.NewCmd(), removeitemcmd.NewCmd(),
		gitcleancmd.NewCmd(), mcpcmd.NewCmd(buildinfo.Version),
	)
	help := root.HelpFunc()
	root.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		if cmd == root {
			localizeCommandList(root)
		}
		help(cmd, args)
	})
	if len(args) != 0 && args[0] == cobra.ShellCompRequestCmd {
		localizeCommandList(root)
	}
	return root
}

func localizeCommandList(root *cobra.Command) {
	language := cmdutil.Localizer()
	for _, cmd := range root.Commands() {
		switch cmd.Name() {
		case "list", "bury":
			cmd.Short = language.Text("cli."+cmd.Name()+".short", nil)
		}
	}
}
