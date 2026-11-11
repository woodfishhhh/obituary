package init

import (
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"

	"slices"

	"github.com/woodfishhhh/obituary/internal/contextmenu"
)

// ContextMenuRegistry is injectable so command tests never register real verbs.
var ContextMenuRegistry = contextmenu.UserRegistry

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
