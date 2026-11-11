package stats

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/guardrm"
)

func NewCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:     "stats",
		Aliases: []string{"eulogy"},
		Short:   "What Obituary has done: tracked, buried, space reclaimed, resurrections",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := cmdutil.Open()
			if err != nil {
				return err
			}
			sum, err := svc.Summarize()
			if err != nil {
				return err
			}
			guardCounts, err := guardrm.ReadCounts(svc.Store.Home)
			if err != nil {
				return err
			}
			guardCounts.Buried += sum.Lifetime.DeleteBuried // retain pre-ledger audit counts
			if asJSON {
				return cmdutil.PrintJSON(struct {
					core.Summary
					DeleteGuard guardrm.Counts `json:"deleteGuard"`
				}{sum, guardCounts})
			}
			l := sum.Lifetime
			fmt.Printf("tracked      %d", sum.Tracked)
			if sum.NextDue != nil {
				fmt.Printf("  (next due %s)", cmdutil.When(*sum.NextDue))
			}
			fmt.Printf("\ngraveyard    %d  %s", sum.Graves, cmdutil.HumanBytes(sum.GraveBytes))
			if sum.NextPurge != nil {
				fmt.Printf("  (next purge %s)", cmdutil.When(*sum.NextPurge))
			}
			fmt.Printf("\n\nburied       %d  %s\n", l.Buried, cmdutil.HumanBytes(l.BuriedBytes))
			fmt.Printf("reclaimed    %d  %s\n", l.Purged, cmdutil.HumanBytes(l.PurgedBytes))
			fmt.Printf("resurrected  %d  %s\n", l.Resurrected, cmdutil.HumanBytes(l.ResurrectedBytes))
			fmt.Printf("delete commands denied  %d\ndelete command burials  %d\n", guardCounts.Denied, guardCounts.Buried)
			if guardCounts.Oversized > 0 {
				fmt.Printf("delete audit lines skipped (too long)  %d\n", guardCounts.Oversized)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}
