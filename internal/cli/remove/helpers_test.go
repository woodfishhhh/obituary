package remove_test

import (
	"github.com/spf13/cobra"
	removeitemcmd "github.com/woodfishhhh/obituary/internal/cli/removeitem"
)

// Use the public command registration, including parameter parsing and errors.
func itemCommand() *cobra.Command { return removeitemcmd.NewCmd() }
