package remove

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/guardrm"
)

func NewCmd(directoryOnly bool) *cobra.Command {
	name := "rm"
	if directoryOnly {
		name = "rmdir"
	}
	cmd := &cobra.Command{
		Use: name + " [options] <path>...", Short: "Bury paths instead of deleting them permanently",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := guardrm.ParseRemove(args, directoryOnly)
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
				return cmdutil.ErrReported
			}
			if opts.Help {
				return cmd.Help()
			}
			svc, err := cmdutil.Open()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
				return cmdutil.ErrReported
			}
			return RemovePaths(svc, opts, name, directoryOnly)
		},
	}
	cmdutil.SetBurialHelp(cmd)
	return cmd
}

func RemovePaths(svc *core.Service, opts guardrm.RemoveOptions, name string, directoryOnly bool) error {
	svc, err := cmdutil.BeginBurial(svc)
	if err != nil {
		return err
	}
	failed, buried := false, false
	report := func(ref string, err error) {
		fmt.Fprintf(os.Stderr, "%s: cannot remove '%s': %v\n", name, ref, err)
		failed = true
	}
	for _, ref := range opts.Paths {
		if err := CheckParentTraversal(ref); err != nil {
			report(ref, err)
			continue
		}
		if ref == "" {
			if !opts.Force {
				report(ref, errors.New("No such file or directory")) //nolint:staticcheck // ST1005: preserve the rm diagnostic asserted by its contract tests.
			}
			continue
		}
		path := ref
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			if !opts.Force {
				report(ref, errors.New("No such file or directory")) //nolint:staticcheck // ST1005: preserve the rm diagnostic asserted by its contract tests.
			}
			continue
		}
		if err != nil {
			report(ref, err)
			continue
		}
		if info.IsDir() {
			root, err := isMSYSRoot(path)
			if err != nil {
				report(ref, err)
				continue
			}
			if root {
				fmt.Fprintf(os.Stderr, "%s: it is dangerous to operate recursively on '%s' (same as '/')\n", name, ref)
				failed = true
				continue
			}
		}
		if directoryOnly && !info.IsDir() {
			report(ref, errors.New("Not a directory")) //nolint:staticcheck // ST1005: preserve the rmdir diagnostic asserted by its contract tests.
			continue
		}
		if info.IsDir() && !opts.Recursive {
			if !opts.Dir {
				report(ref, errors.New("Is a directory")) //nolint:staticcheck // ST1005: preserve the rm diagnostic asserted by its contract tests.
				continue
			}
			if err := EmptyDirectory(path); err != nil {
				report(ref, err)
				continue
			}
		}
		// Bury is the only deletion path. Guard/identity/recycle errors are
		// reported; neither rm nor any permanent-delete API is a fallback.
		results, err := svc.BuryDeletion([]string{path}, opts.Force, name)
		for _, result := range results {
			if result.Action == core.ActionBuried || result.Action == core.ActionRecoveryPending {
				buried = true
			}
			if result.Error != "" {
				report(ref, errors.New(result.Error))
			} else if opts.Verbose {
				if result.Action == core.ActionRecycled {
					fmt.Printf("%s: %s '%s' (%s)\n", name, result.Action, ref, cmdutil.RecycleRecovery)
				} else {
					fmt.Printf("%s: %s '%s'\n", name, result.Action, ref)
				}
			}
		}
		if err := cmdutil.Committed(err); err != nil {
			report(ref, err)
		}
	}
	if buried {
		cmdutil.EnsureDaemon(svc.Store.Home, true)
	}
	if failed {
		return cmdutil.ErrReported
	}
	return nil
}
