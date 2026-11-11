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
	diagnostics := make([]string, len(opts.Paths))
	checked := 0
	check := func(ref string) (bool, error) {
		i := checked
		checked++
		diagnostics[i] = checkRemovePath(ref, opts, name, directoryOnly)
		return diagnostics[i] == "", nil
	}
	results, burialErr := svc.BuryDeletionChecked(opts.Paths, opts.Force, name, check)
	for i, result := range results {
		ref := result.Ref
		if i < len(diagnostics) && diagnostics[i] != "" {
			if diagnostics[i] != "skip" {
				fmt.Fprint(os.Stderr, diagnostics[i])
				failed = true
			}
			continue
		}
		if result.Action == core.ActionBuried || result.Action == core.ActionRecoveryPending {
			buried = true
		}
		if result.Error != "" {
			report(ref, errors.New(result.Error))
		} else if opts.Verbose && result.Action != "" {
			printRemoved(name, ref, result.Action)
		}
	}
	if err := cmdutil.Committed(burialErr); err != nil {
		ref := ""
		if len(opts.Paths) > 0 {
			ref = opts.Paths[max(0, min(checked, len(opts.Paths))-1)]
		}
		report(ref, err)
	}
	if buried {
		cmdutil.EnsureDaemon(svc.Store.Home, true)
	}
	if failed {
		return cmdutil.ErrReported
	}
	return nil
}

func checkRemovePath(ref string, opts guardrm.RemoveOptions, name string, directoryOnly bool) string {
	report := func(err error) string { return fmt.Sprintf("%s: cannot remove '%s': %v\n", name, ref, err) }
	if err := CheckParentTraversal(ref); err != nil {
		return report(err)
	}
	path := ref
	info, err := os.Lstat(path)
	if ref == "" || errors.Is(err, os.ErrNotExist) {
		if opts.Force {
			return "skip"
		}
		return fmt.Sprintf("%s: cannot remove '%s': No such file or directory\n", name, ref)
	}
	if err != nil {
		return report(err)
	}
	if info.IsDir() {
		root, err := isMSYSRoot(path)
		if err != nil {
			return report(err)
		}
		if root {
			return fmt.Sprintf("%s: it is dangerous to operate recursively on '%s' (same as '/')\n", name, ref)
		}
	}
	if directoryOnly && !info.IsDir() {
		return fmt.Sprintf("%s: cannot remove '%s': Not a directory\n", name, ref)
	}
	if info.IsDir() && !opts.Recursive {
		if !opts.Dir {
			return fmt.Sprintf("%s: cannot remove '%s': Is a directory\n", name, ref)
		}
		if err := EmptyDirectory(path); err != nil {
			return report(err)
		}
	}
	return ""
}

func printRemoved(name, ref, action string) {
	if action == core.ActionRecycled {
		fmt.Printf("%s: %s '%s' (%s)\n", name, action, ref, cmdutil.RecycleRecovery)
	} else {
		fmt.Printf("%s: %s '%s'\n", name, action, ref)
	}
}
