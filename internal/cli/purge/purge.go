package purge

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/textutil"
)

// isTerminal is replaced in tests; piped input can never confirm a purge.
var isTerminal = func() bool { return cmdutil.IsTerminal(os.Stdin) }

func NewCmd() *cobra.Command {
	var all, dryRun, asJSON, yes bool
	cmd := &cobra.Command{
		Use:     "purge [id|original path]...",
		Aliases: []string{"cremate"},
		Short:   "Delete selected or expired burials; explicitly select an unresolved restore to abandon it",
		Long:    "Delete selected or expired file burials. --all and explicit references skip retention, so they ask first; outside a terminal they require --yes. Docker containers require elapsed [graveyard] retention_days, including with --all or an explicit ID; running containers are treated as resurrected. Docker removal preserves volumes and never forces removal. Container handling is identical in recycle mode.",
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
			run := func(dryRun bool) ([]core.GraveResult, error) {
				if len(args) > 0 {
					return svc.PurgeRefs(args, dryRun)
				}
				return svc.Purge(all, dryRun)
			}
			// Plain `obit purge` only does what the background does once retention
			// has passed; a selection deletes early, which is the user's call.
			if (all || len(args) > 0) && !dryRun && !yes {
				preview, err := run(true)
				if err := cmdutil.Committed(err); err != nil {
					return err
				}
				if len(preview) == 0 {
					return report(preview, nil, false, asJSON) // nothing to confirm, nothing to run
				}
				if err := confirm(cmd, preview, asJSON); err != nil {
					return err
				}
			}
			results, err := run(dryRun)
			return report(results, err, dryRun, asJSON)
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "purge every burial, not only those past their retention")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "only show what would happen")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print JSON")
	cmd.Flags().BoolVar(&yes, "yes", false, "purge --all or the selected burials without asking (agents: ask the user first)")
	return cmd
}

// confirm lists what would be purged, then asks; outside a terminal it
// refuses, so an agent cannot purge without the user passing --yes.
func confirm(cmd *cobra.Command, preview []core.GraveResult, asJSON bool) error {
	if !asJSON {
		_ = report(preview, nil, true, false) // the run reports failures again
	}
	const retry = "purge deletes permanently; re-run with --yes to confirm (agents: ask the user first)"
	if !isTerminal() {
		return errors.New("not a terminal: " + retry)
	}
	fmt.Fprint(os.Stderr, "Permanently delete the selected burials? [y/N] ")
	// A character device such as NUL also counts as a terminal; it reads EOF.
	answer, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
		if err != nil && answer == "" {
			return fmt.Errorf("confirmation not read (%w): %s", err, retry)
		}
		return errors.New("nothing purged")
	}
	return nil
}

func report(results []core.GraveResult, err error, dryRun, asJSON bool) error {
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
		return fmt.Sprintf("#%d %s%s  %s  (%s)", r.Burial.ID, verb, r.Action, r.Burial.Path, textutil.Bytes(r.Burial.Size))
	})
	if err := cmdutil.Committed(err); err != nil {
		return err
	}
	return reportErr
}
