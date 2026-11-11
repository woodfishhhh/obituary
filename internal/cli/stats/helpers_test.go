package stats

import (
	"os"

	"github.com/spf13/cobra"
	removeitemcmd "github.com/woodfishhhh/obituary/internal/cli/removeitem"
	"github.com/woodfishhhh/obituary/internal/core"
)

// Use the public command registration, including parameter parsing and errors.
func itemCommand() *cobra.Command { return removeitemcmd.NewCmd() }

type failBurialCommitFS struct {
	core.FS
	blocked string
}

func (f *failBurialCommitFS) MovePinned(path string, want core.Identity, to string) error {
	if err := f.FS.MovePinned(path, want, to); err != nil {
		return err
	}
	if f.blocked != "" {
		blocked := f.blocked
		f.blocked = ""
		return os.Mkdir(blocked, 0o700) // only the following store commit fails
	}
	return nil
}
