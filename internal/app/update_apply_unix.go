//go:build linux || darwin

package app

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/woodfishhhh/obituary/internal/update"
)

func StartUpdater(context.Context, string, string, bool) (update.ApplyResult, error) {
	return update.ApplyResult{}, errors.New("desktop updates are only available on Windows; use obit update --apply")
}

func ApplyUpdate(ctx context.Context, home, exe, current string, waitPID int, relaunch bool, out io.Writer) error {
	if waitPID != 0 || relaunch {
		return errors.New("--wait-pid and --relaunch-gui are Windows-only")
	}
	target, err := update.Target(home, exe, current)
	if err != nil {
		return err
	}
	result, err := update.Apply(ctx, home, exe, current, target, update.ApplyHooks{Retire: retireUpdateFiles})
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "Updated to %s; log: %s\n", result.Version, result.LogPath)
	return err
}
