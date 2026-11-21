package doctor

import (
	"fmt"
	"strings"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
	"github.com/woodfishhhh/obituary/internal/core/ledger"
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

func (lang diagnostics) diagnoseWatches(state ledger.State, err error, now time.Time, watchPath func(ledger.WatchPolicy) (string, error)) DoctorCheck {
	c := DoctorCheck{Name: "watch", Status: "PASS"}
	if err != nil {
		c.Status, c.Detail = "FAIL", lang.Text("cli.doctor.cannot_inspect_folder_policies", nil)+err.Error()+lang.Text("cli.doctor.check_store_json", nil)
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
	c.Detail = fmt.Sprintf(lang.Text("cli.doctor.pending_approval_older_than_day_still_awaiting", nil), lang.Count("cli.count.folder_policy", w.Policies), w.Pending, lang.Count("cli.count.unavailable_changed_folder", w.Unavailable), lang.Count("cli.count.exclusion", w.Exclusions), lang.Count("cli.count.exemption", w.Exemptions), lang.Count("cli.count.item", w.Unstable))
	if w.Pending+w.Unavailable+w.Unstable > 0 {
		c.Status = "WARN"
	}
	if w.Unavailable > 0 {
		c.Detail += fmt.Sprintf(lang.Text("cli.doctor.first_unavailable_reconnect_the_folder_or_review", nil), w.FirstUnavailable, firstError)
	}
	if w.Unstable > 0 {
		c.Detail += lang.Text("cli.doctor.first_awaiting_stability", nil) + w.FirstUnstable + lang.Text("cli.doctor.check_for_unfinished_downloads_or_open_writers", nil)
	}
	for _, skip := range w.Skipped {
		c.Detail += fmt.Sprintf("; %s: %s; %s", skip.Path, skip.Reason, skip.Manage)
	}
	return c
}

func watchAwaitingStability(item ledger.Item, now time.Time) bool {
	if item.Source != "watch" || item.CreatedAt.IsZero() || !item.CreatedAt.Before(now.Add(-24*time.Hour)) {
		return false
	}
	// StableSince is the latest observation, not the start of an uninterrupted
	// wait. Report old items still waiting, never a claimed continuous duration.
	if item.Watch != nil && !item.Watch.StableSince.IsZero() && !item.Watch.StableSince.After(now) && now.Sub(item.Watch.StableSince) < ledger.WatchStableWindow {
		return true
	}
	return strings.HasPrefix(item.LastError, "folder child is still active") ||
		strings.HasPrefix(item.LastError, "download temporary name is still present:")
}
