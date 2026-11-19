//go:build linux || darwin

package update

import (
	"context"
	"os/exec"
)

func updateCommand(ctx context.Context, argv []string) (*exec.Cmd, error) {
	return exec.CommandContext(ctx, argv[0], argv[1:]...), nil
}
