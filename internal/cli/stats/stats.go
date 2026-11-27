package stats

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/guardrm"
	"github.com/woodfishhhh/obituary/internal/textutil"
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
			fmt.Printf(cmdutil.Text("cli.stats.tracked"), sum.Tracked)
			if sum.NextDue != nil {
				fmt.Printf(cmdutil.Text("cli.stats.next_due"), cmdutil.When(*sum.NextDue))
			}
			if sum.GraveMaxBytes > 0 {
				if sum.GraveBytes != sum.GraveQuotaBytes {
					fmt.Printf(cmdutil.Text("cli.stats.graveyard_quota_total"), sum.Graves, textutil.Bytes(sum.GraveQuotaBytes), textutil.Bytes(sum.GraveMaxBytes), textutil.Bytes(sum.GraveBytes))
				} else {
					fmt.Printf(cmdutil.Text("cli.stats.graveyard_quota"), sum.Graves, textutil.Bytes(sum.GraveQuotaBytes), textutil.Bytes(sum.GraveMaxBytes))
				}
			} else {
				fmt.Printf(cmdutil.Text("cli.stats.graveyard"), sum.Graves, textutil.Bytes(sum.GraveBytes))
			}
			if sum.NextPurge != nil {
				fmt.Printf(cmdutil.Text("cli.stats.next_purge"), cmdutil.When(*sum.NextPurge))
			}
			if sum.QuotaPendingCount > 0 {
				fmt.Printf(cmdutil.Text("cli.stats.quota_pending"), sum.QuotaPendingCount)
			}
			fmt.Printf(cmdutil.Text("cli.stats.buried"), l.Buried, textutil.Bytes(l.BuriedBytes))
			fmt.Printf(cmdutil.Text("cli.stats.reclaimed"), l.Purged, textutil.Bytes(l.PurgedBytes))
			fmt.Printf(cmdutil.Text("cli.stats.resurrected"), l.Resurrected, textutil.Bytes(l.ResurrectedBytes))
			fmt.Printf(cmdutil.Text("cli.stats.delete_commands_denied_delete_command_burials"), guardCounts.Denied, guardCounts.Buried)
			if guardCounts.Oversized > 0 {
				fmt.Printf(cmdutil.Text("cli.stats.delete_audit_lines_skipped_too_long"), guardCounts.Oversized)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}
