package init

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/cli/cmdutil"
	"github.com/woodfishhhh/obituary/internal/contextmenu"
)

// ContextMenuRegistry is injectable so command tests never register real verbs.
var ContextMenuRegistry = contextmenu.UserRegistry

func runContextMenu(cmd *cobra.Command, self cmdutil.Executable, uninstall, show, dryRun, yes bool) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("context-menu integration is Windows-only")
	}
	for _, flag := range []string{"agent", "guard-rm", "mcp", "skill"} {
		if cmd.Flags().Changed(flag) {
			return fmt.Errorf("--context-menu is a separate user-wide installation; run --%s separately", flag)
		}
	}
	executable, err := self.Path()
	if err != nil {
		return err
	}
	reg := ContextMenuRegistry()
	if show {
		st, err := contextmenu.Inspect(reg, executable)
		if err == nil {
			fmt.Fprintln(cmd.OutOrStdout(), st.Detail)
		}
		return err
	}
	if !uninstall {
		if err := checkMenuDaemon(executable); err != nil {
			return err
		}
	}
	changes, err := contextmenu.Plan(reg, executable, uninstall)
	if err != nil {
		return err
	}
	return applyContextMenu(cmd, reg, changes, dryRun, yes)
}

func applyContextMenu(cmd *cobra.Command, reg contextmenu.Registry, changes []contextmenu.Change, dryRun, yes bool) error {
	for _, change := range changes {
		printContextMenuChange(cmd.OutOrStdout(), change)
	}
	if dryRun || len(changes) == 0 {
		return nil
	}
	if !yes {
		if !cmdutil.IsTerminal(os.Stdin) {
			return errors.New("not a terminal: re-run with --yes to apply these changes")
		}
		fmt.Fprint(cmd.OutOrStdout(), "Apply these context menu changes? [y/N] ")
		answer, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
		if a := strings.ToLower(strings.TrimSpace(answer)); a != "y" && a != "yes" {
			return errors.New("nothing changed")
		}
	}
	if err := contextmenu.Apply(reg, changes); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), "classic context menu updated; Windows 11: 显示更多选项 (Show more options)")
	return nil
}

func printContextMenuChange(out io.Writer, change contextmenu.Change) {
	action, tree := "update", change.After
	if change.Before == nil {
		action = "create"
	}
	if change.After == nil {
		action, tree = "remove", change.Before
	}
	fmt.Fprintf(out, "  %-9s HKCU\\Software\\Classes\\%s\n", action, change.Path)
	for _, key := range slices.Sorted(maps.Keys(tree)) {
		values := tree[key]
		label := values["MUIVerb"]
		if label == "" {
			label = values["DisplayName"]
		}
		if label == "" {
			continue
		}
		commandKey := "command"
		if key != "" {
			commandKey = key + `\command`
		}
		command := tree[commandKey][""]
		if submenu := values["ExtendedSubCommandsKey"]; submenu != "" {
			command = "shared submenu: " + submenu
		} else if command == "" {
			command = "application identity (no command)"
		}
		fmt.Fprintf(out, "    %s\n      %s\n", label, command)
	}
}

func checkMenuDaemon(executable string) error {
	path := filepath.Join(filepath.Dir(executable), "obitd.exe")
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("context menu needs GUI companion %s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("context menu companion is not a regular file: %s", path)
	}
	return nil
}
