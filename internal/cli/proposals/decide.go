package proposals

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/app"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
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
			var results []core.ProposalResult
			if approve {
				results, err = app.Engine(svc).Approve(args)
			} else {
				results, err = svc.Reject(args)
			}
			failed, registered := false, false
			for _, r := range results {
				failed = failed || r.Error != "" || core.Failed(r.Results)
				registered = registered || cmdutil.AnyOK(r.Results)
				registered = registered || r.Proposal.Kind == core.ProposalWatch && r.Error == ""
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

func printDecisions(results []core.ProposalResult, approve bool) {
	for _, r := range results {
		if r.Error != "" {
			fmt.Fprintln(os.Stderr, "error:", r.Error)
			if len(r.Results) == 0 {
				continue
			}
		}
		if !approve {
			fmt.Printf("#%d rejected", r.Proposal.ID)
			if r.Proposal.Kind == core.ProposalOnboard {
				fmt.Print(" (Obituary will not propose automatic registration in this project again)")
			}
			fmt.Println()
			continue
		}
		fmt.Printf("#%d approved\n", r.Proposal.ID)
		for _, res := range r.Results {
			if res.Error != "" {
				fmt.Printf("    skipped  %s  (%s)\n", res.Ref, res.Error)
			} else if r.Proposal.Kind == core.ProposalWatchBurial {
				fmt.Printf("    #%d buried  %s\n", res.Item.ID, res.Item.Path)
			} else {
				fmt.Printf("    #%d %s  expires %s\n", res.Item.ID, res.Item.Path, cmdutil.When(res.Item.ExpiresAt))
			}
		}
	}
}
