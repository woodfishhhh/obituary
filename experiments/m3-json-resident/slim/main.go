// Command slim is `obit` with only `list` and `hook`: the best a launch of those two commands could
// do if the binary did not link every other command's dependencies. A measurement build, not a product.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	hookcmd "github.com/woodfishhhh/obituary/internal/cli/hook"
	listcmd "github.com/woodfishhhh/obituary/internal/cli/list"
)

func main() {
	cmdutil.SkipExplorerCheck()
	root := &cobra.Command{Use: "obit", SilenceUsage: true, SilenceErrors: true}
	root.AddCommand(listcmd.NewCmd(), hookcmd.NewCmd())
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "obit:", err)
		os.Exit(1)
	}
}
