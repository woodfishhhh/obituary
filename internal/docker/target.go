package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// Target pins both routing and daemon identity; a reused socket is not evidence of absence.
type Target struct {
	Context  string `json:"context,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	DaemonID string `json:"daemonId,omitempty"`
	OSType   string `json:"osType,omitempty"`
}

func (id Identity) Key() string {
	if id.Target == nil {
		return "docker:" + id.ID
	}
	return "docker:" + id.Target.Endpoint + "/" + id.Target.DaemonID + "/" + id.ID
}

var ErrTarget = errors.New("docker endpoint or daemon changed; original target must be restored")

type targetKey struct{}

func WithTarget(ctx context.Context, target *Target) context.Context {
	if target == nil {
		return ctx
	}
	return context.WithValue(ctx, targetKey{}, *target)
}

func (c CLI) contextEndpoint(ctx context.Context, name string) (string, error) {
	// Context metadata must be resolved without inherited endpoint overrides.
	c.target = &Target{}
	out, err := c.run(ctx, "context", "inspect", name)
	if err != nil {
		return "", err
	}
	var rows []struct {
		Endpoints map[string]struct{ Host string }
	}
	if err := json.Unmarshal(out, &rows); err != nil {
		return "", err
	}
	if len(rows) != 1 || rows[0].Endpoints["docker"].Host == "" {
		return "", ErrTarget
	}
	return rows[0].Endpoints["docker"].Host, nil
}

func (c CLI) resolveTarget(ctx context.Context) (Target, error) {
	t := Target{Context: os.Getenv("DOCKER_CONTEXT")}
	if t.Context == "" {
		t.Endpoint = os.Getenv("DOCKER_HOST")
	}
	if t.Context == "" && t.Endpoint == "" {
		out, err := c.run(ctx, "context", "show")
		if err != nil {
			return t, err
		}
		t.Context = strings.TrimSpace(string(out))
	}
	if t.Context != "" {
		endpoint, err := c.contextEndpoint(ctx, t.Context)
		if err != nil {
			return t, err
		}
		t.Endpoint = endpoint
	}
	return t, nil
}

func (c CLI) selectedTarget(ctx context.Context) (Target, error) {
	t, ok := ctx.Value(targetKey{}).(Target)
	if !ok && c.target != nil {
		t, ok = *c.target, true
	}
	if !ok {
		return c.resolveTarget(ctx)
	}
	if t.Context != "" {
		endpoint, err := c.contextEndpoint(ctx, t.Context)
		if err != nil {
			return t, err
		}
		if endpoint != t.Endpoint {
			return t, fmt.Errorf("%w: context %q now points to %q (expected %q)", ErrTarget, t.Context, endpoint, t.Endpoint)
		}
	}
	return t, nil
}

func (c CLI) bind(ctx context.Context) (CLI, error) {
	t, err := c.selectedTarget(ctx)
	if err != nil {
		return c, err
	}
	if t.Endpoint == "" {
		return c, ErrTarget
	}
	c.target = &t
	out, err := c.run(ctx, "info", "--format", "{{json .}}")
	if err != nil {
		return c, err
	}
	var info struct{ ID, OSType string }
	if err := json.Unmarshal(out, &info); err != nil {
		return c, err
	}
	if info.ID == "" || t.DaemonID != "" && t.DaemonID != info.ID {
		return c, fmt.Errorf("%w: daemon %q (expected %q) at %s", ErrTarget, info.ID, t.DaemonID, t.Endpoint)
	}
	t.DaemonID, t.OSType = info.ID, info.OSType
	return c, nil
}

func (c CLI) arguments(args []string) []string {
	if c.target == nil || args[0] == "context" {
		return args
	}
	if c.target.Context != "" {
		return append([]string{"--context", c.target.Context}, args...)
	}
	return append([]string{"--host", c.target.Endpoint}, args...)
}

func targetEnvironment() []string {
	var env []string
	for _, e := range os.Environ() {
		key, _, _ := strings.Cut(e, "=")
		if strings.EqualFold(key, "DOCKER_CONTEXT") || strings.EqualFold(key, "DOCKER_HOST") {
			continue
		}
		env = append(env, e)
	}
	return env
}
