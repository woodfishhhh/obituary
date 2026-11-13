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
	root := &cobra.Command{
		Use:           "obit",
		Short:         "Obituary (讣告), the trash bin for AI agents: give files a lifespan; when it runs out they are buried in a graveyard you can resurrect from",
		Version:       buildinfo.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
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
	if err := root.Execute(); err != nil {
		if !errors.Is(err, cmdutil.ErrReported) {
			fmt.Fprintln(os.Stderr, "obit:", err)
		}
		os.Exit(1)
	}
}
