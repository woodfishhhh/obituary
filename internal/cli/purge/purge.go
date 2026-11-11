package purge

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
)

func NewCmd() *cobra.Command {
	var all, dryRun, asJSON bool
	cmd := &cobra.Command{
		Use:     "purge [id|original path]...",
		Aliases: []string{"cremate"},
		Short:   "Delete selected or expired burials; explicitly select an unresolved restore to abandon it",
		Long:    "Delete selected or expired file burials. Docker containers require elapsed [graveyard] retention_days, including with --all or an explicit ID; running containers are treated as resurrected. Docker removal preserves volumes and never forces removal. Container handling is identical in recycle mode.",
		Args: func(cmd *cobra.Command, args []string) error {
			if all && len(args) > 0 {
				return fmt.Errorf("--all cannot be combined with explicit burial references")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := cmdutil.Open()
			if err != nil {
				return err
			}
			var results []core.GraveResult
			if len(args) > 0 {
				results, err = svc.PurgeRefs(args, dryRun)
			} else {
				results, err = svc.Purge(all, dryRun)
			}
			if !asJSON && len(results) == 0 {
				fmt.Println("nothing to purge")
			}
			verb := ""
			if dryRun {
				verb = "would be "
			}
			reportErr := cmdutil.ReportGraves(results, asJSON, func(r core.GraveResult) string {
				if r.Action == core.ActionResurrected {
					if dryRun {
						return fmt.Sprintf("#%d would confirm an already completed resurrection  %s", r.Burial.ID, r.Path)
					}
					return fmt.Sprintf("#%d resurrected  %s (confirmed)", r.Burial.ID, r.Path)
				}
				return fmt.Sprintf("#%d %s%s  %s  (%s)", r.Burial.ID, verb, r.Action, r.Burial.Path, cmdutil.HumanBytes(r.Burial.Size))
			})
			if err := cmdutil.Committed(err); err != nil {
				return err
			}
			return reportErr
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "purge every burial, not only those past their retention")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "only show what would happen")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	return cmd
}
