package extend

import (
	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core/lifespan"
)

func NewCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "extend <id|path>... <duration>",
		Short:   "Push expiries back (counted from now if already due)",
		Example: "  obit extend 3 1d\n  obit extend debug.log 30m",
		Args:    cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := lifespan.ParseDuration(args[len(args)-1])
			if err != nil {
				return err
			}
			svc, err := cmdutil.Open()
			if err != nil {
				return err
			}
			results, err := svc.Extend(args[:len(args)-1], d)
			if err := cmdutil.Committed(err); err != nil {
				return err
			}
			return cmdutil.Report(results, asJSON)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}
