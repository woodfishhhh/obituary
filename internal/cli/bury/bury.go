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
	language := cmdutil.Localizer()
	cmd := &cobra.Command{
		Use:   "bury <path>...",
		Short: language.Text("cli.bury.short", nil),
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
			language := cmdutil.Localizer()
			reportErr := cmdutil.ReportBury(results, asJSON, func(r core.GraveResult) string {
				if r.Action == core.ActionRecycled {
					return language.Text("cli.bury.recycled", map[string]any{"Action": language.Label(r.Action), "Ref": r.Ref})
				}
				if r.Action == core.ActionResurrected && r.Burial != nil {
					return language.Text("cli.bury.resurrected", map[string]any{"ID": r.Burial.ID, "Path": r.Path})
				}
				if r.Burial == nil {
					return fmt.Sprintf("%-8s %s", language.Label(r.Action), r.Ref)
				}
				return language.Text("cli.bury.buried", map[string]any{
					"ID": r.Burial.ID, "Path": r.Burial.Path, "Size": textutil.Bytes(r.Burial.Size),
					"PurgeAt": r.Burial.PurgeAt.Local().Format(cmdutil.TimeLayout),
				})
			})
			if err := cmdutil.Committed(err); err != nil {
				return err
			}
			return reportErr
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, language.Text("cli.bury.force", nil))
	cmd.Flags().StringVar(&note, "note", "", language.Text("cli.bury.note", nil))
	cmd.Flags().BoolVar(&asJSON, "json", false, language.Text("cli.bury.json", nil))
	cmdutil.SetBurialHelp(cmd)
	return cmd
}
