package bury

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/textutil"
)

func NewCmd() *cobra.Command {
	var (
		force, asJSON bool
		note          string
	)
	cmd := &cobra.Command{
		Use:   "bury <path>...",
		Short: "Move paths now using the configured deletion mode",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := cmdutil.Open()
			if err != nil {
				return err
			}
			svc, err = cmdutil.BeginBurial(svc.WithContext(cmd.Context()))
			if err != nil {
				return err
			}
			results, err := svc.Bury(args, force, note)
			for _, r := range results {
				if r.Action == core.ActionBuried || r.Action == core.ActionRecoveryPending {
					cmdutil.EnsureDaemon(svc.Store.Home, asJSON)
					break
				}
			}
			reportErr := cmdutil.ReportBury(results, asJSON, func(r core.GraveResult) string {
				if r.Action == core.ActionRecycled {
					return fmt.Sprintf("%-8s %s  (%s)", r.Action, r.Ref, cmdutil.RecycleRecovery)
				}
				if r.Action == core.ActionResurrected && r.Burial != nil {
					return fmt.Sprintf("#%d resurrected  %s (recovered)", r.Burial.ID, r.Path)
				}
				if r.Burial == nil {
					return fmt.Sprintf("%-8s %s", r.Action, r.Ref)
				}
				return fmt.Sprintf("#%d buried   %s  (%s, restorable until %s)", r.Burial.ID, r.Burial.Path,
					textutil.Bytes(r.Burial.Size), r.Burial.PurgeAt.Local().Format(cmdutil.TimeLayout))
			})
			if err := cmdutil.Committed(err); err != nil {
				return err
			}
			return reportErr
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "allow folders that contain a .git repository")
	cmd.Flags().StringVar(&note, "note", "", "free-text note kept with the burial")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	cmdutil.SetBurialHelp(cmd)
	return cmd
}
