package untrust

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	rulescmd "github.com/woodfishhhh/obituary/internal/cli/rules"
)

func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "untrust [rules file]",
		Short: "Stop applying the project's .obitrules",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := rulescmd.RulesFile(args)
			if err != nil {
				return err
			}
			e, err := cmdutil.Engine()
			if err != nil {
				return err
			}
			was, err := e.Trust.Untrust(path)
			if err != nil {
				return err
			}
			if was {
				fmt.Println("no longer trusted:", path)
			} else {
				fmt.Println("was not trusted:", path)
			}
			return nil
		},
	}
}
