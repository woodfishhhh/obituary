package docker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"time"
)

const RestoreLabel = "io.obituary.restore"
const volumeImage = "docker.io/library/debian@sha256:734a72b3d6142a71993b4bc580ca01d8ca687f77a2c422d96a961d3376b179bc"

var volumeName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]+$`)
var ErrNoVolume = errors.New("Docker volume does not exist on the original daemon")

// Volume contains only inspect fields used to identify a local volume. Never use Mountpoint as a host path.
type Volume struct {
	Name              string            `json:"name"`
	Driver            string            `json:"driver"`
	Scope             string            `json:"scope"`
	CreatedAt         time.Time         `json:"createdAt"`
	Labels            map[string]string `json:"labels,omitempty"`
	Options           map[string]string `json:"options,omitempty"`
	Target            Target            `json:"target"`
	RestoreLabelOwned bool              `json:"restoreLabelOwned,omitempty"`
	mountpoint        string
	HolderID          string `json:"-"`
}

func (v Volume) Check() error {
	if !volumeName.MatchString(v.Name) || v.CreatedAt.IsZero() || v.Target.Endpoint == "" || v.Target.DaemonID == "" {
		return errors.New("invalid Docker volume identity")
	}
	if v.Driver != "local" || v.Scope != "local" || v.Target.OSType != "linux" {
		return errors.New("requires a Linux daemon and a local-scope, local-driver Docker volume")
	}
	if len(v.Options) > 0 {
		return fmt.Errorf("unsupported local volume option keys: %s", strings.Join(slices.Sorted(maps.Keys(v.Options)), ", "))
	}
	return nil
}

func (v Volume) Same(o Volume) bool {
	return v.Name == o.Name && v.CreatedAt.Equal(o.CreatedAt) && v.Driver == o.Driver && v.Scope == o.Scope &&
		v.Target == o.Target && maps.Equal(v.Labels, o.Labels) && maps.Equal(v.Options, o.Options)
}

func (v Volume) RestoreLabels(previous *Volume, token string) (map[string]string, error) {
	if err := v.Check(); err != nil {
		return nil, err
	}
	if token == "" {
		return nil, errors.New("missing restore transaction")
	}
	if _, taken := v.Labels[RestoreLabel]; taken && !v.RestoreLabelOwned && (previous == nil || !v.Same(*previous)) {
		return nil, errors.New("io.obituary.restore is not owned by this ledger")
	}
	result := make(map[string]string, len(v.Labels)+1)
	maps.Copy(result, v.Labels)
	result[RestoreLabel] = token
	return result, nil
}

func (c CLI) InspectVolume(ctx context.Context, name string) (Volume, error) {
	if !volumeName.MatchString(name) {
		return Volume{}, errors.New("invalid Docker volume name")
	}
	bound, err := c.bind(ctx)
	if err != nil {
		return Volume{}, err
	}
	out, err := bound.run(ctx, "volume", "inspect", "--", name)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "no such volume") || strings.Contains(strings.ToLower(err.Error()), "no such object") {
			return Volume{}, ErrNoVolume
		}
		return Volume{}, err
	}
	var rows []struct {
		Volume
		Mountpoint string
	}
	if err := json.Unmarshal(out, &rows); err != nil {
		return Volume{}, err
	}
	if len(rows) != 1 || rows[0].Name != name {
		return Volume{}, errors.New("volume inspect returned an unexpected object")
	}
	v := rows[0].Volume
	v.mountpoint = rows[0].Mountpoint
	v.Target = *bound.target
	return v, v.Check()
}
