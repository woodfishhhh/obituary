package resurrect

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
)

func NewCmd() *cobra.Command {
	var asJSON, last bool
	cmd := &cobra.Command{
		Use:     "resurrect <id|original path>... | --last",
		Aliases: []string{"raise"},
		Short:   "Restore files without overwriting; start buried Docker containers by ID",
		Args: func(cmd *cobra.Command, args []string) error {
			if last {
				return cobra.NoArgs(cmd, args)
			}
			return cobra.MinimumNArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := cmdutil.Open()
			if err != nil {
				return err
			}
			svc = svc.WithContext(cmd.Context())
			var restored []core.GraveResult
			if last {
				session, project := cmdutil.BurialScope()
				args, restored, err = svc.LastBurialRefs(session, project)
				if err != nil {
					return err
				}
			}
			results, err := svc.Resurrect(args)
			results = append(restored, results...)
			reportErr := cmdutil.ReportGraves(results, asJSON, func(r core.GraveResult) string {
				return fmt.Sprintf("#%d resurrected  %s", r.Burial.ID, r.Path)
			})
			if err := cmdutil.Committed(err); err != nil {
				return err
			}
			return reportErr
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	cmd.Flags().BoolVar(&last, "last", false, "restore the latest deletion batch in the current session or Git project")
	return cmd
}
