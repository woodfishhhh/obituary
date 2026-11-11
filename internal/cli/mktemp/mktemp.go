package mktemp

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
)

func NewCmd() *cobra.Command {
	var (
		name, session string
		asJSON        bool
	)
	cmd := &cobra.Command{
		Use:   "mktemp <lifespan>",
		Short: "Create a temporary directory that buries itself; prints its path",
		Example: `  dir=$(obit mktemp 1d)
  obit mktemp 2h --name screenshots`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := core.ParseDuration(args[0])
			if err != nil {
				return err
			}
			e, err := cmdutil.Engine()
			if err != nil {
				return err
			}
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			dir, item, err := e.MkTemp(d, name, cwd, session)
			if err := cmdutil.Committed(err); err != nil {
				return err
			}
			cmdutil.EnsureDaemon(e.Svc.Store.Home, asJSON)
			if asJSON {
				return cmdutil.PrintJSON(item)
			}
			fmt.Println(dir)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "tmp", "name prefix for the directory")
	cmd.Flags().StringVar(&session, "session", "", "agent session the directory belongs to")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}
