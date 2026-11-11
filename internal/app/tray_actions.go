package app

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/woodfishhhh/obituary/internal/core"
)

// RecentBurials is a read-only menu snapshot. Clicking a row sends its ID,
// never its current menu position, through the ordinary resurrection checks.
func (r *Resident) RecentBurials() ([]core.Burial, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, err := r.Service.Store.Inspect()
	if err != nil {
		return nil, err
	}
	burials := slices.Clone(state.Burials)
	slices.SortFunc(burials, func(a, b core.Burial) int {
		if c := b.BuriedAt.Compare(a.BuriedAt); c != 0 {
			return c
		}
		return cmp.Compare(b.ID, a.ID)
	})
	return burials[:min(10, len(burials))], nil
}

func (r *Resident) Restore(id int64) (core.GraveResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	results, err := r.Service.Resurrect([]string{strconv.FormatInt(id, 10)})
	result := results[0]
	if err != nil {
		return result, err
	}
	if result.Error != "" {
		return result, fmt.Errorf("#%d: %s", id, result.Error)
	}
	return result, nil
}

func (r *Resident) Upcoming(now time.Time) ([]core.Item, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	state, err := r.Service.Store.Inspect()
	if err != nil {
		return nil, err
	}
	items := slices.DeleteFunc(slices.Clone(state.Items), func(item core.Item) bool { return item.ExpiresAt.After(now.Add(24 * time.Hour)) })
	slices.SortFunc(items, func(a, b core.Item) int { return a.ExpiresAt.Compare(b.ExpiresAt) })
	return items[:min(10, len(items))], nil
}
