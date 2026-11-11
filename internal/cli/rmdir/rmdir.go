package rmdir

import (
	"github.com/spf13/cobra"
	removecmd "github.com/woodfishhhh/obituary/internal/cli/remove"
)

func NewCmd() *cobra.Command {
	return removecmd.NewCmd(true)
}
