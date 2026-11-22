package docker

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
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
//
//nolint:funlen,nestif // this is the bounded event state machine for archive evidence.
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
	base := []string{"events", "--since", since.UTC().Format(time.RFC3339Nano), "--until", until.UTC().Format(time.RFC3339Nano), "--format", "{{json .}}"}
	var rows []volumeEvent
	for _, filters := range [][]string{{"type=container"}, {"type=volume", "volume=" + name}} {
		args := append([]string{}, base[:len(base)-2]...)
		for _, filter := range filters {
			args = append(args, "--filter", filter)
		}
		args = append(args, "--format", "{{json .}}")
		out, runErr := bound.run(ctx, args...)
		if runErr != nil {
			return nil, runErr
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), string(rune(10))) {
			if strings.TrimSpace(line) == "" {
				continue
			}
			var row volumeEvent
			if err := json.Unmarshal([]byte(line), &row); err != nil {
				return nil, err
			}
			rows = append(rows, row)
		}
	}
	slices.SortFunc(rows, func(a, b volumeEvent) int { return cmp.Compare(a.TimeNano, b.TimeNano) })
	return rows, nil
}

// CheckVolumeEvents refuses incomplete or foreign activity in the operation window.
//
//nolint:funlen // the bounded event evidence state machine stays in one ordered pass.
func CheckVolumeEvents(rows []volumeEvent, name, holder string, post bool) error {
	started, holderDestroyed, destroyed := false, false, 0
	helpers, created := map[string]bool{}, map[string]bool{}
	var previous int64
	for _, row := range rows {
		if row.TimeNano <= 0 || row.TimeNano < previous {
			return errors.New("daemon event time is inconsistent")
		}
		previous = row.TimeNano
		if row.Type == "container" {
			if row.Action == "create" && row.Actor.ID == holder {
				started = true
			} else if row.Action == "create" && row.Actor.ID != "" {
				created[row.Actor.ID] = true
			}
			if row.Action == "destroy" && row.Actor.ID == holder {
				holderDestroyed = true
			}
			continue
		}
		if row.Type != "volume" || row.Actor.ID != name {
			continue
		}
		switch row.Action {
		case "mount", "unmount":
			container := row.Actor.Attributes["container"]
			if container != "" && container != holder {
				helpers[container] = true
			}
		case "create":
			return errors.New("volume was recreated during archival")
		case "destroy":
			destroyed++
		}
	}
	if len(helpers) == 0 {
		for id := range created {
			if id != holder {
				helpers[id] = true
			}
		}
	}
	holderID := holder
	//nolint:nestif // derive the holder only when Docker omitted the expected create identity.
	if !started {
		var candidates []string
		for id := range created {
			if !helpers[id] {
				candidates = append(candidates, id)
			}
		}
		if len(candidates) == 1 {
			started, holderID = true, candidates[0]
			for _, row := range rows {
				if row.Type == "container" && row.Action == "destroy" && row.Actor.ID == holderID {
					holderDestroyed = true
				}
			}
		}
	}
	if !started || len(helpers) != 1 {
		return fmt.Errorf("volume event evidence incomplete: started=%t helpers=%d created=%d destroyed=%d holderDestroyed=%t holder=%s events=%s", started, len(helpers), len(created), destroyed, holderDestroyed, holderID, summarizeVolumeEvents(rows))
	}
	if post && (!holderDestroyed || destroyed != 1) {
		return errors.New("volume deletion event evidence incomplete; do not assume success")
	}
	if !post && destroyed != 0 {
		return errors.New("volume was destroyed before archival completed")
	}
	return nil
}

func summarizeVolumeEvents(rows []volumeEvent) string {
	var parts []string
	for _, row := range rows {
		parts = append(parts, row.Type+":"+row.Action+":"+row.Actor.ID)
	}
	return strings.Join(parts, ",")
}
