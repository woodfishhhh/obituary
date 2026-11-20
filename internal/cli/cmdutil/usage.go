package cmdutil

import (
	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/app"
)

func SetUsage(root *cobra.Command) {
	root.PersistentPostRun = func(cmd *cobra.Command, _ []string) {
		top := cmd
		for top.Parent() != nil && top.Parent() != root {
			top = top.Parent()
		}
		switch top.Name() {
		case "add", "bury", "cancel", "sweep", "list", "extend", "resurrect", "graveyard", "purge", "stats", "mktemp", "rules", "trust", "untrust", "proposals", "approve", "reject", "watch", "docker", "rm", "remove-item", "rmdir", "git-clean", "log":
			app.ObserveUsage()
		}
	}
}
