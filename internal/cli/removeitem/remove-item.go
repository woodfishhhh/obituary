//go:build windows

package removeitem

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	removecmd "github.com/woodfishhhh/obituary/internal/cli/remove"
	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/guardrm"
	"golang.org/x/sys/windows"
)

func NewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "remove-item [-Recurse] [-Force] [-Path|-LiteralPath] <path>",
		Short: "Bury filesystem items using noninteractive PowerShell parameters",
		Long: `Bury filesystem items using a limited subset of PowerShell parameters.
This is not a complete replacement for PowerShell Remove-Item.
Accepts exactly one path: a positional argument, or one -Path / -LiteralPath value.
Positional paths and -Path reject wildcards: * ? [ ].
For wildcards use obit rm in bash, where the shell expands them.
Use -LiteralPath for literal filenames, including names containing [ or ].
Commas are literal filename characters, not path separators.
Does not honor $WhatIfPreference or $ConfirmPreference; use native Remove-Item
when these session preferences are needed.`,
		DisableFlagParsing: true,
		RunE:               run,
	}
}

func run(cmd *cobra.Command, args []string) error {
	if len(args) == 1 && args[0] == "--help" {
		return cmd.Help()
	}
	opts, err := guardrm.ParseRemoveItem(args)
	if err != nil {
		itemError("", err)
		return cmdutil.ErrReported
	}
	svc, err := cmdutil.Open()
	if err != nil {
		itemError("", err)
		return cmdutil.ErrReported
	}
	failed, buried := false, false
	for _, path := range opts.Paths {
		if err := removecmd.CheckParentTraversal(path); err != nil {
			itemError(path, err)
			failed = true
			continue
		}
		if err := checkItem(path, opts); err != nil {
			itemError(path, err)
			failed = true
			continue
		}
		// -Force only affects provider attributes. Guard stays enabled.
		results, err := svc.BuryDeletion([]string{path}, false, "remove-item")
		for _, result := range results {
			if result.Action == core.ActionBuried || result.Action == core.ActionRecoveryPending {
				buried = true
			}
			if result.Error != "" {
				itemError(path, errors.New(result.Error))
				failed = true
			}
		}
		if err := cmdutil.Committed(err); err != nil {
			itemError(path, err)
			failed = true
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

func itemError(path string, err error) {
	fmt.Fprintf(os.Stderr, "Remove-Item : Cannot remove item '%s': %v\n    + CategoryInfo          : WriteError: (%s) [Remove-Item], IOException\n    + FullyQualifiedErrorId : RemoveItemError,Microsoft.PowerShell.Commands.RemoveItemCommand\n", path, err, path)
}

func itemAttributes(path string) (uint32, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	return windows.GetFileAttributes(p)
}

func checkItem(path string, opts guardrm.ItemOptions) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("Cannot find path '%s' because it does not exist", path) //nolint:staticcheck // ST1005: preserve the PowerShell-compatible Remove-Item diagnostic.
	}
	if err != nil {
		return err
	}
	if info.IsDir() && !opts.Recursive {
		if err := removecmd.EmptyDirectory(path); err != nil {
			return fmt.Errorf("directory has children; -Recurse is required: %w", err)
		}
	}
	if opts.Force {
		return nil
	}
	// Moving an entire tree would otherwise silently include hidden/read-only
	// descendants. Keep the whole tree when any descendant needs -Force.
	return filepath.WalkDir(path, func(p string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		attrs, err := itemAttributes(p)
		if err != nil {
			return err
		}
		if attrs&(windows.FILE_ATTRIBUTE_HIDDEN|windows.FILE_ATTRIBUTE_SYSTEM|windows.FILE_ATTRIBUTE_READONLY) != 0 {
			return fmt.Errorf("Access to '%s' is denied; hidden or read-only items require -Force", p) //nolint:staticcheck // ST1005: preserve the PowerShell-compatible Remove-Item diagnostic.
		}
		return nil
	})
}
