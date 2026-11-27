package sweep

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/textutil"
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
			if errors.Is(err, lifecycle.ErrSweepBusy) {
				return errors.New(cmdutil.OutputLocalizer(asJSON).Text("cli.sweep.another_sweep_is_running_probably_the_daemon", nil))
			}
			err = cmdutil.Committed(err)
			if asJSON {
				if jerr := cmdutil.PrintJSON(outcomes); jerr != nil {
					return jerr
				}
			} else {
				printOutcomes(outcomes, dryRun)
			}
			if err != nil {
				return err
			}
			for _, o := range outcomes {
				if o.Action == ledger.ActionRetry || o.Action == ledger.ActionBlocked {
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

func printOutcomes(outcomes []lifecycle.Outcome, dryRun bool) {
	if len(outcomes) == 0 {
		fmt.Println(cmdutil.Text("cli.sweep.nothing_is_due"))
	}
	l := cmdutil.Localizer()
	verb := ""
	if dryRun {
		verb = cmdutil.Text("cli.preview.would_be")
	}
	for _, o := range outcomes {
		if o.QuotaOverflow != nil {
			fmt.Fprintln(os.Stderr, cmdutil.FormatQuotaOverflow(l, o.QuotaOverflow, dryRun))
		} else if o.Action == ledger.ActionPurged && o.Detail == ledger.PurgeBasisQuotaFIFO {
			fmt.Printf("#%d %s%s %s (%s) [%s]\n", o.Item.ID, verb, l.Label(o.Action), o.Path, textutil.Bytes(o.Size), o.Detail)
		} else {
			fmt.Printf("#%d %s%s %s %s\n", o.Item.ID, verb, l.Label(o.Action), o.Path, o.Detail)
		}
		if o.QuotaEviction != nil {
			cmdutil.PrintQuotaEviction(os.Stderr, l, o.QuotaEviction, dryRun)
		}
	}
}
