package update

import (
	"context"
	"errors"
	"io"
	"time"
)

func runCommand(ctx context.Context, log io.Writer, argv []string) error {
	cmd, err := updateCommand(ctx, argv)
	if err != nil {
		return err
	}
	cmd.Stdout, cmd.Stderr = log, log
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Run(); err != nil {
		return errors.Join(ctx.Err(), err)
	}
	return nil
}
