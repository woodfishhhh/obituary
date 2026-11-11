package doctor

import (
	"fmt"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/textutil"
)

type WatchCheck struct {
	Policies         int              `json:"policies"`
	Pending          int              `json:"pending"`
	Unavailable      int              `json:"unavailable"`
	Unstable         int              `json:"unstable"`
	Exclusions       int              `json:"exclusions"`
	Exemptions       int              `json:"exemptions"`
	FirstUnavailable string           `json:"firstUnavailable,omitempty"`
	FirstUnstable    string           `json:"firstUnstable,omitempty"`
	Skipped          []core.WatchSkip `json:"skipped,omitempty"`
}

func diagnoseWatches(state core.State, err error, now time.Time, watchPath func(core.WatchPolicy) (string, error)) DoctorCheck {
	c := DoctorCheck{Name: "watch", Status: "PASS"}
	if err != nil {
		c.Status, c.Detail = "FAIL", "cannot inspect folder policies: "+err.Error()+"; check store.json"
		return c
	}
	w := &WatchCheck{Policies: len(state.Watches), Exemptions: len(state.WatchExemptions)}
	c.Watch = w
	firstError := ""
	for _, policy := range state.Watches {
		w.Exclusions += len(policy.Exclusions)
		if policy.Consent == "pending" {
			w.Pending++
		}
		path, err := watchPath(policy)
		if err == nil {
			var skips []core.WatchSkip
			skips, err = core.WatchDownloadSkips(path)
			w.Skipped = append(w.Skipped, skips...)
		}
		if err != nil {
			w.Unavailable++
			if w.FirstUnavailable == "" {
				w.FirstUnavailable, firstError = policy.Path, err.Error()
			}
		}
	}
	for _, item := range state.Items {
		if watchAwaitingStability(item, now) {
			w.Unstable++
			if w.FirstUnstable == "" {
				w.FirstUnstable = item.Path
			}
		}
	}
	c.Detail = fmt.Sprintf("%s, %d pending approval, %s, %s, %s, %s older than 1 day still awaiting stability; obit watch --list shows exemptions (path, reason, time); obit proposals shows pending decisions", textutil.Count(w.Policies, "folder policy", "folder policies"), w.Pending, textutil.Count(w.Unavailable, "unavailable/changed folder"), textutil.Count(w.Exclusions, "exclusion"), textutil.Count(w.Exemptions, "exemption"), textutil.Count(w.Unstable, "item"))
	if w.Pending+w.Unavailable+w.Unstable > 0 {
		c.Status = "WARN"
	}
	if w.Unavailable > 0 {
		c.Detail += fmt.Sprintf("; first unavailable: %s (%s); reconnect the folder or review and replace its policy", w.FirstUnavailable, firstError)
	}
	if w.Unstable > 0 {
		c.Detail += "; first awaiting stability: " + w.FirstUnstable + "; check for unfinished downloads or open writers before changing its lifespan"
	}
	for _, skip := range w.Skipped {
		c.Detail += fmt.Sprintf("; %s: %s; %s", skip.Path, skip.Reason, skip.Manage)
	}
	return c
}

func watchAwaitingStability(item core.Item, now time.Time) bool {
	if item.Source != "watch" || item.CreatedAt.IsZero() || !item.CreatedAt.Before(now.Add(-24*time.Hour)) {
		return false
	}
	// StableSince is the latest observation, not the start of an uninterrupted
	// wait. Report old items still waiting, never a claimed continuous duration.
	if item.Watch != nil && !item.Watch.StableSince.IsZero() && !item.Watch.StableSince.After(now) && now.Sub(item.Watch.StableSince) < core.WatchStableWindow {
		return true
	}
	return strings.HasPrefix(item.LastError, "folder child is still active") ||
		strings.HasPrefix(item.LastError, "download temporary name is still present:")
}
