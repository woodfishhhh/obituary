// Package docker adapts the Docker CLI without touching images, volumes or networks.
package docker

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const Timeout = 5 * time.Second

// StopTimeout covers the Windows default grace period plus transport overhead.
const StopTimeout = 35 * time.Second

// Identity is immutable across renames and pins a container across daemon changes.
type Identity struct {
	Target  *Target   `json:"target,omitempty"`
	ID      string    `json:"id"`
	Created time.Time `json:"created"`
	Name    string    `json:"name"`
	ShortID string    `json:"shortId"`
}

func (id Identity) Valid() bool {
	decoded, err := hex.DecodeString(id.ID)
	return err == nil && len(decoded) == 32 && id.ID == strings.ToLower(id.ID) && !id.Created.IsZero() && (id.Target == nil || id.Target.Endpoint != "" && id.Target.DaemonID != "")
}

func (id Identity) Display() string { return "docker:" + id.Name + " (" + id.ShortID + ")" }
func (id Identity) Same(other Identity) bool {
	return id.ID == other.ID && id.Created.Equal(other.Created) && sameTarget(id.Target, other.Target)
}

type Container struct {
	Identity
	Running     bool
	StopTimeout *int
	AutoRemove  bool
	Labels      map[string]string
	Binds       []string
}

type Client interface {
	Inspect(context.Context, string) (Container, error)
	List(context.Context) ([]Container, error)
	ListIDs(context.Context) ([]string, error)
	// Stop calls beforeSend after preflight and immediately before starting the CLI.
	Stop(context.Context, Identity, func(Container) error) error
	Start(context.Context, Identity) error
	Remove(context.Context, Identity, func([]string) error) (running bool, err error)
}

var ErrIdentity = errors.New("container identity changed or is invalid")
var ErrNotFound = errors.New("container not found")
var ErrAutoRemove = errors.New("container has AutoRemove enabled; stopping it would delete its writable layer")

// CLI uses os/exec for context deadlines and argument boundaries, preserving
// the user's Docker transport configuration without invoking a shell.
type CLI struct {
	Executable string
	Timeout    time.Duration
	target     *Target
}

func (c CLI) run(ctx context.Context, args ...string) ([]byte, error) {
	return c.runBefore(ctx, nil, args...)
}

func (c CLI) runBefore(ctx context.Context, beforeSend func() error, args ...string) ([]byte, error) {
	timeout := c.Timeout
	if timeout <= 0 {
		timeout = Timeout
		if args[0] == "stop" {
			timeout = StopTimeout
		}
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	executable := c.Executable
	if executable == "" {
		executable = "docker"
	}
	cmd := exec.CommandContext(ctx, executable, c.arguments(args)...)
	if c.target != nil {
		cmd.Env = targetEnvironment()
	}
	hideWindow(cmd)
	cmd.WaitDelay = time.Second
	if beforeSend != nil {
		if err := beforeSend(); err != nil {
			return nil, errors.Join(ErrNotSent, err)
		}
	}
	out, err := cmd.Output()
	if err != nil && cmd.Process == nil && beforeSend != nil {
		return nil, errors.Join(ErrNotSent, err)
	}
	if ctx.Err() != nil {
		return nil, fmt.Errorf("docker %s: %w", args[0], ctx.Err())
	}
	if err != nil {
		var exit *exec.ExitError
		if len(args) == 4 && args[0] == "container" && args[1] == "inspect" && errors.As(err, &exit) {
			message := strings.TrimSpace(string(exit.Stderr))
			if strings.HasSuffix(message, "No such container: "+args[3]) || strings.HasSuffix(message, "No such object: "+args[3]) {
				return nil, ErrNotFound
			}
		}
		return nil, fmt.Errorf("docker %s unavailable: %w%s", args[0], err, errorSummary(err))
	}
	return out, nil
}

type inspection struct {
	ID      string `json:"Id"`
	Created time.Time
	Name    string
	State   struct{ Running bool }
	Config  struct {
		Labels      map[string]string
		StopTimeout *int
	}
	HostConfig struct{ AutoRemove bool }
	Mounts     []struct{ Type, Source string }
}

func (c CLI) Inspect(ctx context.Context, ref string) (Container, error) {
	bound, err := c.bind(ctx)
	if err != nil {
		return Container{}, err
	}
	got, err := bound.inspect(ctx, ref)
	if errors.Is(err, ErrNotFound) {
		if _, targetErr := bound.bind(ctx); targetErr != nil {
			return Container{}, targetErr
		}
	}
	return got, err
}
func (c CLI) inspect(ctx context.Context, ref string) (Container, error) {
	out, err := c.run(ctx, "container", "inspect", "--", ref)
	if err != nil {
		return Container{}, err
	}
	var rows []inspection
	if err := json.Unmarshal(out, &rows); err != nil {
		return Container{}, err
	}
	if len(rows) != 1 {
		return Container{}, errors.New("docker inspect did not return exactly one container")
	}
	row := rows[0]
	id := Identity{Target: c.target, ID: row.ID, Created: row.Created, Name: strings.TrimPrefix(row.Name, "/")}
	if !id.Valid() {
		return Container{}, ErrIdentity
	}
	id.ShortID = id.ID[:12]
	result := Container{Identity: id, StopTimeout: row.Config.StopTimeout, Running: row.State.Running, AutoRemove: row.HostConfig.AutoRemove, Labels: row.Config.Labels}
	for _, mount := range row.Mounts {
		if mount.Type == "bind" {
			result.Binds = append(result.Binds, mount.Source)
		}
	}
	return result, nil
}

func (c CLI) Available(ctx context.Context) error {
	_, err := c.bind(ctx)
	return err
}

func (c CLI) List(ctx context.Context) ([]Container, error) {
	var err error
	c, err = c.bind(ctx)
	if err != nil {
		return nil, err
	}
	out, err := c.run(ctx, "ps", "--all", "--no-trunc", "--format", "{{json .ID}}")
	if err != nil {
		return nil, err
	}
	var rows []Container
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var id string
		if err := json.Unmarshal([]byte(line), &id); err != nil {
			return nil, err
		}
		row, err := c.inspect(ctx, id)
		if err != nil {
			return nil, err
		}
		if row.ID != id {
			return nil, ErrIdentity
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (c CLI) verified(ctx context.Context, want Identity) (Container, error) {
	if !want.Valid() {
		return Container{}, ErrIdentity
	}
	got, err := c.inspect(ctx, want.ID)
	if err != nil {
		return Container{}, err
	}
	if !got.Same(want) {
		return Container{}, ErrIdentity
	}
	return got, nil
}

func (c CLI) Stop(ctx context.Context, want Identity, beforeSend func(Container) error) error {
	var err error
	c, err = c.bind(WithTarget(ctx, want.Target))
	if err != nil {
		return errors.Join(ErrNotSent, err)
	}
	got, err := c.verified(ctx, want)
	if err != nil {
		return errors.Join(ErrNotSent, err)
	}
	if got.AutoRemove {
		return errors.Join(ErrNotSent, ErrAutoRemove)
	}
	if err := c.noNameCollision(ctx, want); err != nil {
		return errors.Join(ErrNotSent, err)
	}
	stop := c.forStop(got)
	_, err = stop.runBefore(ctx, func() error {
		if beforeSend != nil {
			return beforeSend(got)
		}
		return nil
	}, "stop", "--", want.ID)
	if errors.Is(err, ErrNotSent) {
		return err
	}
	return c.afterAction(ctx, want, false, err)
}

func (c CLI) Start(ctx context.Context, want Identity) error {
	var err error
	c, err = c.bind(WithTarget(ctx, want.Target))
	if err != nil {
		return err
	}
	if _, err := c.verified(ctx, want); err != nil {
		return err
	}
	if err := c.noNameCollision(ctx, want); err != nil {
		return err
	}
	_, err = c.run(ctx, "start", "--", want.ID)
	return c.afterAction(ctx, want, false, err)
}

func (c CLI) Remove(ctx context.Context, want Identity, beforeSend func([]string) error) (bool, error) {
	var err error
	c, err = c.bind(WithTarget(ctx, want.Target))
	if err != nil {
		return false, err
	}
	before, err := c.listIDs(ctx)
	if err != nil {
		return false, err
	}
	got, err := c.verified(ctx, want)
	if err != nil {
		return false, err
	}
	if got.Running {
		return true, nil
	}
	if err := c.noNameCollision(ctx, want); err != nil {
		return false, err
	}
	_, err = c.runBefore(ctx, func() error {
		if beforeSend != nil {
			return beforeSend(before)
		}
		return nil
	}, "rm", "--", want.ID)
	if errors.Is(err, ErrNotSent) {
		return false, err
	}
	// Docker itself refuses a concurrent start: rm never receives --force or --volumes.
	err = c.afterAction(ctx, want, true, err)
	after, listErr := c.ListIDs(ctx)
	if listErr == nil {
		listErr = CheckRemoval(before, after, want.ID)
	}
	return false, errors.Join(err, listErr)
}
