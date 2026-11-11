package docker

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// ErrNotSent proves that the action subprocess never started.
var ErrNotSent = errors.New("docker action was not sent")

var ErrNameCollision = errors.New("container name equals the full ID; Docker could select the named container")
var ErrAmbiguousAction = errors.New("docker may have acted on a same-name container; verify both containers")

func sameTarget(a, b *Target) bool {
	// Legacy identities are bound and persisted before their first destructive operation.
	if a == nil || b == nil {
		return true
	}
	return a.Endpoint == b.Endpoint && a.DaemonID == b.DaemonID
}

func (c Container) StopWait() time.Duration {
	seconds := 30
	if c.Target != nil && c.Target.OSType == "linux" {
		seconds = 10
	}
	if c.StopTimeout != nil {
		seconds = *c.StopTimeout
	}
	// An infinite engine grace period has no finite proof of completion.
	if seconds < 0 {
		return 24 * time.Hour
	}
	maxSeconds := int64((time.Duration(1<<63-1) - Timeout) / time.Second)
	if int64(seconds) > maxSeconds {
		return time.Duration(1<<63 - 1)
	}
	return time.Duration(seconds)*time.Second + Timeout
}

func (c CLI) forStop(got Container) CLI {
	if c.Timeout <= 0 {
		c.Timeout = got.StopWait()
	}
	return c
}

func (c CLI) noNameCollision(ctx context.Context, want Identity) error {
	out, err := c.run(ctx, "ps", "--all", "--no-trunc", "--filter", "name=^/"+want.ID+"$", "--format", "{{json .ID}}")
	if err != nil {
		return err
	}
	if len(out) > 0 {
		return fmt.Errorf("%w: %s", ErrNameCollision, want.ID)
	}
	return nil
}

func (c CLI) afterAction(ctx context.Context, want Identity, removed bool, actionErr error) error {
	bound, err := c.bind(ctx)
	if err != nil {
		return errors.Join(actionErr, err)
	}
	got, err := bound.inspect(ctx, want.ID)
	if errors.Is(err, ErrNotFound) && removed {
		return actionErr
	}
	if err == nil && !got.Same(want) {
		return errors.Join(actionErr, ErrAmbiguousAction)
	}
	if err != nil {
		return errors.Join(actionErr, err)
	}
	return actionErr
}
