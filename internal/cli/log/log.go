package log

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
)

func NewCmd() *cobra.Command {
	var (
		n      int
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show the audit log of everything Obituary did",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := cmdutil.Open()
			if err != nil {
				return err
			}
			events, err := svc.Store.History(n)
			if err != nil {
				return err
			}
			if asJSON {
				return cmdutil.PrintJSON(events)
			}
			for _, e := range events {
				fmt.Printf("%s  #%d %-9s %s %s\n", e.Time.Local().Format("2006-01-02 15:04:05"), e.ID, e.Action, e.Path, e.Detail)
			}
			return nil
		},
	}
	cmd.Flags().IntVarP(&n, "lines", "n", 20, "number of events")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}
