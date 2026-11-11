//go:build linux || darwin

package removeitem

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/woodfishhhh/obituary/internal/core"
)

func NewCmd() *cobra.Command {
	return &cobra.Command{Use: "remove-item", Short: "Windows PowerShell Remove-Item compatibility (Windows only)", DisableFlagParsing: true, RunE: func(*cobra.Command, []string) error {
		return fmt.Errorf("remove-item is Windows-only: %w", core.ErrUnsupported)
	}}
}
