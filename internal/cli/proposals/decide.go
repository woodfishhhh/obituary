package proposals

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
)

func NewDecideCmd(use, short string, approve bool) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   use + " <proposal id>...",
		Short: short,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := cmdutil.Open()
			if err != nil {
				return err
			}
			var results []lifecycle.ProposalResult
			if approve {
				results, err = app.Engine(svc).Approve(args)
			} else {
				results, err = svc.Reject(args)
			}
			failed, registered := false, false
			for _, r := range results {
				failed = failed || r.Error != "" || lifecycle.Failed(r.Results)
				registered = registered || cmdutil.AnyOK(r.Results)
				registered = registered || r.Proposal.Kind == ledger.ProposalWatch && r.Error == ""
			}
			if approve && registered {
				cmdutil.EnsureDaemon(svc.Store.Home, asJSON)
			}
			err = cmdutil.Committed(err)
			if asJSON {
				if err := cmdutil.PrintJSON(results); err != nil {
					return err
				}
			} else {
				printDecisions(results, approve)
			}
			if err != nil {
				return err
			}
			if failed {
				return cmdutil.ErrReported
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}

func printDecisions(results []lifecycle.ProposalResult, approve bool) {
	for _, r := range results {
		if r.Error != "" {
			fmt.Fprintln(os.Stderr, cmdutil.Text("cli.error_prefix"), r.Error)
			if len(r.Results) == 0 {
				continue
			}
		}
		if !approve {
			fmt.Printf(cmdutil.Text("cli.proposals.rejected"), r.Proposal.ID)
			if r.Proposal.Kind == ledger.ProposalOnboard {
				fmt.Print(cmdutil.Text("cli.proposals.obituary_will_not_propose_automatic_registration_in"))
			}
			fmt.Println()
			continue
		}
		fmt.Printf(cmdutil.Text("cli.proposals.approved"), r.Proposal.ID)
		for _, res := range r.Results {
			if res.Error != "" {
				fmt.Printf(cmdutil.Text("cli.proposals.skipped"), res.Ref, res.Error)
			} else if r.Proposal.Kind == ledger.ProposalWatchBurial || r.Proposal.Kind == ledger.ProposalGraveyardQuota {
				fmt.Printf(cmdutil.Text("cli.proposals.buried"), res.Item.ID, res.Item.Path)
			} else {
				fmt.Printf(cmdutil.Text("cli.proposals.expires"), res.Item.ID, res.Item.Path, cmdutil.When(res.Item.ExpiresAt))
			}
		}
		if r.QuotaEviction != nil {
			cmdutil.PrintQuotaEviction(os.Stderr, cmdutil.Localizer(), r.QuotaEviction, false)
		}
	}
}
