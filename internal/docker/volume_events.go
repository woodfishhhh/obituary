package docker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

type volumeEvent struct {
	Type, Action string
	TimeNano     int64
	Actor        struct {
		ID         string
		Attributes map[string]string
	}
}

type volumeEvidence struct{ Name, Holder, Helper, Witness string }

// Anchor both ends in the same buffered history. A filtered empty reply cannot distinguish idle from truncated.
func (w volumeEvidence) check(rows []volumeEvent, post bool) error {
	started, mounted, verified, released, closed := false, false, false, false, false
	removals := 0
	var previous int64
	for _, e := range rows {
		if !started {
			started = e.Type == "container" && e.Actor.ID == w.Holder && e.Action == "create"
			if !started {
				continue
			}
		}
		if e.TimeNano <= 0 || e.TimeNano < previous {
			return errors.New("daemon event time is inconsistent")
		}
		previous = e.TimeNano
		if e.Type == "container" {
			if e.Action == "create" && e.Actor.ID != w.Holder && e.Actor.ID != w.Helper && e.Actor.ID != w.Witness {
				return errors.New("cannot exclude volume use by another newly created container")
			}
			if e.Action == "destroy" && e.Actor.ID == w.Helper {
				verified = true
			}
			if e.Action == "destroy" && e.Actor.ID == w.Holder {
				if !post || !verified {
					return errors.New("volume holder lost before release")
				}
				released = true
			}
			if e.Action == "destroy" && e.Actor.ID == w.Witness && post {
				closed = true
				break
			}
		}
		if e.Type != "volume" || e.Actor.ID != w.Name {
			continue
		}
		switch e.Action {
		case "mount", "unmount":
			if e.Actor.Attributes["container"] != w.Helper {
				return errors.New("foreign or unattributed volume mount activity")
			}
			mounted = mounted || e.Action == "mount"
		case "destroy":
			if !post || !released {
				return errors.New("unexpected volume destruction")
			}
			removals++
		default:
			return errors.New("unexpected volume event during archival")
		}
	}
	if !started || !mounted || !verified || post && (!closed || removals != 1) {
		return errors.New("volume event evidence incomplete; do not assume success")
	}
	return nil
}

func (c CLI) VolumeEvents(ctx context.Context, name string, since, until time.Time) ([]volumeEvent, error) {
	bound, err := c.bind(ctx)
	if err != nil {
		return nil, err
	}
	out, err := bound.run(ctx, "events", "--since", since.UTC().Format(time.RFC3339Nano), "--until", until.UTC().Format(time.RFC3339Nano), "--filter", "type=volume", "--filter", "volume="+name, "--format", "{{json .}}")
	if err != nil {
		return nil, err
	}
	var rows []volumeEvent
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var row volumeEvent
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			return nil, err
		}
		rows = append(rows, row)
	}
	return rows, nil
}
