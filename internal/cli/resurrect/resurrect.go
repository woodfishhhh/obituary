package resurrect

import (
	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core/lifecycle"
	"github.com/woodfishhhh/obituary/internal/i18n"
)

func NewCmd() *cobra.Command {
	var asJSON, last bool
	language := cmdutil.Localizer()
	cmd := &cobra.Command{
		Use:     "resurrect <id|original path>... | --last",
		Aliases: []string{"raise"},
		Short:   language.Text("cli.resurrect.short", nil),
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
			var restored []lifecycle.GraveResult
			if last {
				session, project := cmdutil.BurialScope()
				args, restored, err = svc.LastBurialRefs(session, project)
				if err != nil {
					return err
				}
			}
			results, err := svc.Resurrect(args)
			results = append(restored, results...)
			language := cmdutil.Localizer()
			reportErr := cmdutil.ReportGraves(results, asJSON, func(r lifecycle.GraveResult) string {
				return language.Text("cli.resurrect.result", map[string]any{"ID": r.Burial.ID, "Path": r.Path})
			})
			if err := cmdutil.Committed(err); err != nil {
				return err
			}
			return reportErr
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, language.Text("cli.resurrect.json", nil))
	cmd.Flags().BoolVar(&last, "last", false, language.Text("cli.resurrect.last", nil))
	cmdutil.SetLocalizedHelp(cmd, func(language *i18n.Localizer, command *cobra.Command) {
		command.Short = language.Text("cli.resurrect.short", nil)
		command.Flags().Lookup("json").Usage = language.Text("cli.resurrect.json", nil)
		command.Flags().Lookup("last").Usage = language.Text("cli.resurrect.last", nil)
	})
	return cmd
}
