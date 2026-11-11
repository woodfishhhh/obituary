package sweep

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
)

func NewCmd() *cobra.Command {
	var dryRun, asJSON bool
	cmd := &cobra.Command{
		Use:     "sweep",
		Aliases: []string{"reap"},
		Short:   "Reap now: bury everything that is due without waiting for the runner",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := cmdutil.Open()
			if err != nil {
				return err
			}
			var proposalErr error
			if !dryRun {
				proposalErr = app.Engine(svc).RefreshProposals()
			}
			outcomes, err := svc.Sweep(dryRun)
			err = errors.Join(proposalErr, err)
			if errors.Is(err, core.ErrSweepBusy) {
				return errors.New("another sweep is running (probably the daemon); try again shortly")
			}
			err = cmdutil.Committed(err)
			if asJSON {
				if jerr := cmdutil.PrintJSON(outcomes); jerr != nil {
					return jerr
				}
			} else {
				if len(outcomes) == 0 {
					fmt.Println("nothing is due")
				}
				verb := ""
				if dryRun {
					verb = "would be "
				}
				for _, o := range outcomes {
					fmt.Printf("#%d %s%s %s %s\n", o.Item.ID, verb, o.Action, o.Path, o.Detail)
				}
			}
			if err != nil {
				return err
			}
			for _, o := range outcomes {
				if o.Action == core.ActionRetry || o.Action == core.ActionBlocked {
					return cmdutil.ErrReported // still pending: not everything due could be recycled
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "only show what would happen")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}
